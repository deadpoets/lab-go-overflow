// ctxprobe checks whether GetThreadContext on a suspended sibling thread
// returns planted general-purpose and XMM values, and whether the XSTATE
// APIs return the planted upper half of a YMM register.
package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"runtime"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/cpu"
	"golang.org/x/sys/windows"
)

func plantAndSpin(pat *[32]byte, ready, stop *uint32)
func plantAndSpin512(pat *[64]byte, ready, stop *uint32)

const (
	contextAMD64         = 0x00100000
	contextInteger       = contextAMD64 | 0x2
	contextFloatingPoint = contextAMD64 | 0x8
	contextXState        = contextAMD64 | 0x40
	xstateAVX            = 2
	xstateMaskAVX        = 1 << xstateAVX
	xstateMaskAVX512     = 0xe0 // opmask, ZMM_H, Hi16_ZMM
	threadSuspendResume  = 0x0002
	threadGetContext     = 0x0008
	threadQueryInfo      = 0x0040
)

// Offsets into the amd64 CONTEXT structure (winnt.h).
const (
	offContextFlags = 0x30
	offRbx          = 0x90
	offR12          = 0xd8
	offR13          = 0xe0
	offXmm0         = 0x1a0 // FltSave (XMM_SAVE_AREA32) at 0x100, XmmRegisters at +0xa0
)

var (
	k32                       = windows.NewLazySystemDLL("kernel32.dll")
	pGetEnabledXStateFeatures = k32.NewProc("GetEnabledXStateFeatures")
	pInitializeContext2       = k32.NewProc("InitializeContext2")
	pSetXStateFeaturesMask    = k32.NewProc("SetXStateFeaturesMask")
	pGetXStateFeaturesMask    = k32.NewProc("GetXStateFeaturesMask")
	pLocateXStateFeature      = k32.NewProc("LocateXStateFeature")
	pGetThreadContext         = k32.NewProc("GetThreadContext")
	pSuspendThread            = k32.NewProc("SuspendThread")
	pResumeThread             = k32.NewProc("ResumeThread")
)

func main() {
	var pat [32]byte
	for i := range pat {
		pat[i] = 0xa0 + byte(i)
	}
	fmt.Printf("CPU: AVX=%v AVX2=%v AVX512F=%v\n", cpu.X86.HasAVX, cpu.X86.HasAVX2, cpu.X86.HasAVX512F)
	en, _, _ := pGetEnabledXStateFeatures.Call()
	fmt.Printf("GetEnabledXStateFeatures = %#x\n", en)

	var ready, stop uint32
	tidc := make(chan uint32)
	go func() {
		runtime.LockOSThread()
		tidc <- windows.GetCurrentThreadId()
		plantAndSpin(&pat, &ready, &stop)
	}()
	tid := <-tidc
	for atomic.LoadUint32(&ready) == 0 {
		runtime.Gosched()
	}

	h, err := windows.OpenThread(threadSuspendResume|threadGetContext|threadQueryInfo, false, tid)
	check("OpenThread", err)
	defer windows.CloseHandle(h)
	if r, _, e := pSuspendThread.Call(uintptr(h)); int32(r) == -1 {
		check("SuspendThread", e)
	}

	fails := 0
	expect := func(name string, got, want []byte) {
		ok := bytes.Equal(got, want)
		if !ok {
			fails++
		}
		fmt.Printf("  %-22s %-5v got %s\n", name, ok, hex.EncodeToString(got))
	}

	// Part 1: plain CONTEXT, integer + legacy FP/XMM.
	fmt.Println("Part 1: GetThreadContext(CONTEXT_INTEGER|CONTEXT_FLOATING_POINT)")
	buf := make([]byte, 0x4d0+16)
	ctx := unsafe.Pointer((uintptr(unsafe.Pointer(&buf[0])) + 15) &^ 15)
	*(*uint32)(unsafe.Add(ctx, offContextFlags)) = contextInteger | contextFloatingPoint
	r, _, e := pGetThreadContext.Call(uintptr(h), uintptr(ctx))
	if r == 0 {
		check("GetThreadContext", e)
	}
	c := unsafe.Slice((*byte)(ctx), 0x4d0)
	expect("RBX", c[offRbx:offRbx+8], pat[0:8])
	expect("R12", c[offR12:offR12+8], pat[8:16])
	expect("R13", c[offR13:offR13+8], pat[16:24])
	expect("XMM1", c[offXmm0+16:offXmm0+32], pat[0:16])
	expect("XMM5 (low of Y5)", c[offXmm0+5*16:offXmm0+6*16], pat[0:16])

	// Part 2: CONTEXT_XSTATE for the YMM upper halves.
	fmt.Println("Part 2: InitializeContext2 + CONTEXT_XSTATE")
	if pInitializeContext2.Find() != nil {
		fmt.Println("  InitializeContext2 not available")
		fails++
	} else {
		mask := uint64(xstateMaskAVX)
		if cpu.X86.HasAVX512F {
			mask |= xstateMaskAVX512
		}
		flags := uint32(contextInteger | contextFloatingPoint | contextXState)
		var need uint32
		pInitializeContext2.Call(0, uintptr(flags), 0, uintptr(unsafe.Pointer(&need)), uintptr(mask))
		fmt.Printf("  context size with XSTATE mask %#x: %d bytes\n", mask, need)
		xbuf := make([]byte, need)
		var xctx uintptr
		r, _, e = pInitializeContext2.Call(uintptr(unsafe.Pointer(&xbuf[0])), uintptr(flags), uintptr(unsafe.Pointer(&xctx)), uintptr(unsafe.Pointer(&need)), uintptr(mask))
		if r == 0 {
			check("InitializeContext2", e)
		}
		r, _, e = pSetXStateFeaturesMask.Call(xctx, uintptr(mask))
		if r == 0 {
			check("SetXStateFeaturesMask", e)
		}
		r, _, e = pGetThreadContext.Call(uintptr(h), xctx)
		if r == 0 {
			check("GetThreadContext(XSTATE)", e)
		}
		var got uint64
		pGetXStateFeaturesMask.Call(xctx, uintptr(unsafe.Pointer(&got)))
		fmt.Printf("  GetXStateFeaturesMask after get: %#x\n", got)
		var n uint32
		p, _, _ := pLocateXStateFeature.Call(xctx, xstateAVX, uintptr(unsafe.Pointer(&n)))
		if p == 0 {
			fmt.Println("  LocateXStateFeature(AVX) returned NULL")
			fails++
		} else {
			fmt.Printf("  AVX feature: %d bytes\n", n)
			ymmh := unsafe.Slice((*byte)(unsafe.Pointer(p)), n)
			expect("YMM5 upper half", ymmh[5*16:6*16], pat[16:32])
		}
		if cpu.X86.HasAVX512F {
			fails += checkAVX512(en)
		} else {
			fmt.Println("Part 3: AVX-512 not present; skipped")
		}
	}

	pResumeThread.Call(uintptr(h))
	atomic.StoreUint32(&stop, 1)
	if fails != 0 {
		fmt.Printf("FAIL: %d mismatches\n", fails)
		os.Exit(1)
	}
	fmt.Println("PASS")
}

// checkAVX512 plants Z5, Z16 and K2 in a fresh spinning thread and reads
// them back through the opmask, ZMM_H and Hi16_ZMM XSTATE features.
func checkAVX512(enabled uintptr) int {
	fmt.Println("Part 3: AVX-512 via XSTATE (opmask=5, ZMM_H=6, Hi16_ZMM=7)")
	if enabled&xstateMaskAVX512 != xstateMaskAVX512 {
		fmt.Printf("  OS has not enabled AVX-512 state (enabled=%#x)\n", enabled)
		return 1
	}
	var pat [64]byte
	for i := range pat {
		pat[i] = 0xc0 + byte(i)
	}
	var ready, stop uint32
	tidc := make(chan uint32)
	go func() {
		runtime.LockOSThread()
		tidc <- windows.GetCurrentThreadId()
		plantAndSpin512(&pat, &ready, &stop)
	}()
	tid := <-tidc
	for atomic.LoadUint32(&ready) == 0 {
		runtime.Gosched()
	}
	defer atomic.StoreUint32(&stop, 1)
	h, err := windows.OpenThread(threadSuspendResume|threadGetContext|threadQueryInfo, false, tid)
	check("OpenThread", err)
	defer windows.CloseHandle(h)
	if r, _, e := pSuspendThread.Call(uintptr(h)); int32(r) == -1 {
		check("SuspendThread", e)
	}
	defer pResumeThread.Call(uintptr(h))

	mask := uint64(xstateMaskAVX | xstateMaskAVX512)
	flags := uint32(contextInteger | contextFloatingPoint | contextXState)
	var need uint32
	pInitializeContext2.Call(0, uintptr(flags), 0, uintptr(unsafe.Pointer(&need)), uintptr(mask))
	fmt.Printf("  context size with XSTATE mask %#x: %d bytes\n", mask, need)
	xbuf := make([]byte, need)
	var xctx uintptr
	if r, _, e := pInitializeContext2.Call(uintptr(unsafe.Pointer(&xbuf[0])), uintptr(flags), uintptr(unsafe.Pointer(&xctx)), uintptr(unsafe.Pointer(&need)), uintptr(mask)); r == 0 {
		check("InitializeContext2", e)
	}
	if r, _, e := pSetXStateFeaturesMask.Call(xctx, uintptr(mask)); r == 0 {
		check("SetXStateFeaturesMask", e)
	}
	if r, _, e := pGetThreadContext.Call(uintptr(h), xctx); r == 0 {
		check("GetThreadContext(XSTATE 512)", e)
	}
	var got uint64
	pGetXStateFeaturesMask.Call(xctx, uintptr(unsafe.Pointer(&got)))
	fmt.Printf("  GetXStateFeaturesMask after get: %#x\n", got)

	fails := 0
	feature := func(id uintptr) []byte {
		var n uint32
		p, _, _ := pLocateXStateFeature.Call(xctx, id, uintptr(unsafe.Pointer(&n)))
		if p == 0 {
			fmt.Printf("  LocateXStateFeature(%d) returned NULL\n", id)
			fails++
			return nil
		}
		fmt.Printf("  feature %d: %d bytes\n", id, n)
		return unsafe.Slice((*byte)(unsafe.Pointer(p)), n)
	}
	expect := func(name string, got, want []byte) {
		ok := bytes.Equal(got, want)
		if !ok {
			fails++
		}
		fmt.Printf("  %-22s %-5v got %s\n", name, ok, hex.EncodeToString(got))
	}
	// Z5 = XMM5 (legacy area) + YMM5 upper (AVX) + ZMM5 upper 256 (ZMM_H).
	c := unsafe.Slice((*byte)(unsafe.Pointer(xctx)), 0x4d0)
	expect("Z5 bytes 0-15 (XMM)", c[offXmm0+5*16:offXmm0+6*16], pat[0:16])
	if f := feature(xstateAVX); f != nil {
		expect("Z5 bytes 16-31 (AVX)", f[5*16:6*16], pat[16:32])
	}
	if f := feature(6); f != nil {
		expect("Z5 bytes 32-63 (ZMM_H)", f[5*32:6*32], pat[32:64])
	}
	if f := feature(7); f != nil {
		expect("Z16 (Hi16_ZMM)", f[0:64], pat[0:64])
	}
	if f := feature(5); f != nil {
		expect("K2 (opmask)", f[2*8:3*8], pat[0:8])
	}
	return fails
}

func check(what string, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", what, err)
		os.Exit(2)
	}
}
