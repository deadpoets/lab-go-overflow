// scanprobe checks two ways for a parent to find bytes in a child's memory on
// Windows: a live scan of a suspended child, and a full-memory minidump that
// is byte-scanned without parsing.
//
// The child derives two 16-byte patterns from a seed it reads on stdin: a
// canary it keeps live, and a secret it either keeps or wipes. The parent
// never passes either pattern to the child, so neither can appear in the
// child's command line or pipe buffers.
//
//	go run .            # parent: runs keep and wipe cases
//	go run . child      # child mode, used by the parent
package main

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func holdInRegister(p *[16]byte, ready, stop *uint32)

func derive(seed []byte, k byte) []byte {
	p := make([]byte, len(seed))
	for i := range seed {
		p[i] = seed[i] ^ k
	}
	return p
}

var keep []byte // live canary

func child() {
	var seed [16]byte
	var mode [1]byte
	io.ReadFull(os.Stdin, seed[:])
	io.ReadFull(os.Stdin, mode[:])
	keep = derive(seed[:], 0xa5)
	secret := derive(seed[:], 0x5a)
	clear(seed[:])
	if mode[0] == 'w' {
		clear(secret)
	}
	if mode[0] == 'r' {
		// Register-only: the secret lives only in X1 of a spinning thread.
		var buf [16]byte
		copy(buf[:], secret)
		clear(secret)
		var ready, stop uint32
		go func() {
			runtime.LockOSThread()
			holdInRegister(&buf, &ready, &stop)
		}()
		for atomic.LoadUint32(&ready) == 0 {
			runtime.Gosched()
		}
		os.Stdout.Write([]byte("ready\n"))
		io.ReadFull(os.Stdin, mode[:])
		atomic.StoreUint32(&stop, 1)
		runtime.KeepAlive(keep)
		return
	}
	runtime.KeepAlive(secret)
	os.Stdout.Write([]byte("ready\n"))
	io.ReadFull(os.Stdin, mode[:]) // block until parent is done
	runtime.KeepAlive(keep)
}

const (
	processQueryInfo = 0x0400
	processVMRead    = 0x0010
	threadSuspend    = 0x0002
)

var (
	k32       = windows.NewLazySystemDLL("kernel32.dll")
	pSuspend  = k32.NewProc("SuspendThread")
	pResume   = k32.NewProc("ResumeThread")
	dbghelp   = windows.NewLazySystemDLL("dbghelp.dll")
	pMiniDump = dbghelp.NewProc("MiniDumpWriteDump")
)

func threadsOf(pid uint32) []uint32 {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	must(err)
	defer windows.CloseHandle(snap)
	var te windows.ThreadEntry32
	te.Size = uint32(unsafe.Sizeof(te))
	var ids []uint32
	for err = windows.Thread32First(snap, &te); err == nil; err = windows.Thread32Next(snap, &te) {
		if te.OwnerProcessID == pid {
			ids = append(ids, te.ThreadID)
		}
	}
	return ids
}

type scan struct {
	bytes, regions int
	canary, secret int
	elapsed        time.Duration
}

func liveScan(pid uint32, canary, secret []byte) scan {
	start := time.Now()
	var hs []windows.Handle
	for _, tid := range threadsOf(pid) {
		h, err := windows.OpenThread(threadSuspend, false, tid)
		if err != nil {
			continue
		}
		pSuspend.Call(uintptr(h))
		hs = append(hs, h)
	}
	defer func() {
		for _, h := range hs {
			pResume.Call(uintptr(h))
			windows.CloseHandle(h)
		}
	}()
	ph, err := windows.OpenProcess(processQueryInfo|processVMRead, false, pid)
	must(err)
	defer windows.CloseHandle(ph)
	var s scan
	var addr uintptr
	for {
		var mbi windows.MemoryBasicInformation
		if windows.VirtualQueryEx(ph, addr, &mbi, unsafe.Sizeof(mbi)) != nil {
			break
		}
		next := mbi.BaseAddress + mbi.RegionSize
		const readable = windows.PAGE_READONLY | windows.PAGE_READWRITE | windows.PAGE_WRITECOPY |
			windows.PAGE_EXECUTE_READ | windows.PAGE_EXECUTE_READWRITE | windows.PAGE_EXECUTE_WRITECOPY
		if mbi.State == windows.MEM_COMMIT && mbi.Protect&readable != 0 && mbi.Protect&(windows.PAGE_GUARD|windows.PAGE_NOACCESS) == 0 {
			buf := make([]byte, mbi.RegionSize)
			var n uintptr
			if windows.ReadProcessMemory(ph, mbi.BaseAddress, &buf[0], uintptr(len(buf)), &n) == nil || n > 0 {
				buf = buf[:n]
				s.bytes += len(buf)
				s.regions++
				s.canary += bytes.Count(buf, canary)
				s.secret += bytes.Count(buf, secret)
			}
		}
		if next <= addr {
			break
		}
		addr = next
	}
	s.elapsed = time.Since(start)
	return s
}

func dumpScan(pid uint32, dir string, canary, secret []byte) scan {
	start := time.Now()
	ph, err := windows.OpenProcess(processQueryInfo|processVMRead, false, pid)
	must(err)
	defer windows.CloseHandle(ph)
	path := filepath.Join(dir, fmt.Sprintf("child-%d.dmp", pid))
	f, err := os.Create(path)
	must(err)
	const miniDumpWithFullMemory = 0x2
	r, _, e := pMiniDump.Call(uintptr(ph), uintptr(pid), f.Fd(), miniDumpWithFullMemory, 0, 0, 0)
	f.Close()
	if r == 0 {
		must(e)
	}
	b, err := os.ReadFile(path)
	must(err)
	os.Remove(path)
	return scan{bytes: len(b), canary: bytes.Count(b, canary), secret: bytes.Count(b, secret), elapsed: time.Since(start)}
}

func runCase(exe, mode string, dump bool) scan {
	var seed [16]byte
	rand.Read(seed[:])
	canary, secret := derive(seed[:], 0xa5), derive(seed[:], 0x5a)
	cmd := exec.Command(exe, "child")
	in, _ := cmd.StdinPipe()
	out, _ := cmd.StdoutPipe()
	must(cmd.Start())
	in.Write(seed[:])
	in.Write([]byte(mode))
	var line [6]byte
	io.ReadFull(out, line[:])
	var s scan
	if dump {
		s = dumpScan(uint32(cmd.Process.Pid), os.TempDir(), canary, secret)
	} else {
		s = liveScan(uint32(cmd.Process.Pid), canary, secret)
	}
	in.Write([]byte("x"))
	cmd.Wait()
	return s
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "child" {
		child()
		return
	}
	exe, err := os.Executable()
	must(err)
	fails := 0
	for _, dump := range []bool{false, true} {
		name := "live scan"
		if dump {
			name = "minidump "
		}
		for _, mode := range []string{"k", "w", "r"} {
			for range 3 {
				s := runCase(exe, mode, dump)
				// Register-only secrets: a live memory scan must not see them;
				// a minidump must, via its thread context stream.
				wantSecret := mode == "k" || (mode == "r" && dump)
				ok := s.canary > 0 && (s.secret > 0) == wantSecret
				if !ok {
					fails++
				}
				fmt.Printf("%s mode=%s ok=%-5v canary=%d secret=%d  %6.1f MiB  regions=%d  %v\n",
					name, map[string]string{"k": "keep", "w": "wipe", "r": "reg "}[mode], ok, s.canary, s.secret,
					float64(s.bytes)/(1<<20), s.regions, s.elapsed.Round(time.Millisecond))
			}
		}
	}
	if fails > 0 {
		fmt.Println("FAIL", fails)
		os.Exit(1)
	}
	fmt.Println("PASS")
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
