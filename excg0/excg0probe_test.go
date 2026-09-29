// Temporary probe; copied into go/src/runtime by run.sh and removed after.

package runtime_test

import (
	"bytes"
	"crypto/rand"
	"runtime"
	"testing"
	"unsafe"
)

//go:noinline
func probeGrow(n int) byte {
	var pad [1024]byte
	pad[n%1024] = byte(n)
	if n == 0 {
		return pad[0]
	}
	return probeGrow(n-1) + pad[n%1024]
}

//go:noinline
func probeFault(t *testing.T, p *[16]byte, tail []byte) (recovered bool) {
	defer func() {
		recovered = recover() != nil
		// Scan g0 stacks here, inside the panic and before the unwind
		// (mcall(recovery)) runs anything else on g0.
		_, g0s := runtime.ProbeStacks()
		for _, s := range g0s {
			scanFor(t, "g0 stack, inside defer", s[0], s[1], tail)
		}
	}()
	runtime.ProbeLoadAndFault(p)
	return false
}

func scanFor(t *testing.T, name string, lo, hi uintptr, tail []byte) {
	if lo == 0 || hi <= lo {
		t.Logf("%s: no stack bounds [%#x,%#x)", name, lo, hi)
		return
	}
	for _, r := range committedRanges(lo, hi) {
		mem := unsafe.Slice((*byte)(unsafe.Pointer(r[0])), r[1]-r[0])
		for i := 0; ; {
			j := bytes.Index(mem[i:], tail)
			if j < 0 {
				break
			}
			at := i + j - 1
			reg := -1
			if at >= 0 {
				reg = int(mem[at])
			}
			t.Logf("%s [%#x,%#x): X%d at %d bytes below hi", name, lo, hi, reg, int(hi-r[0])-at)
			i += j + 1
		}
	}
}

func TestExcG0Probe(t *testing.T) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		probeGrow(256)
		seed, pat := new([16]byte), new([16]byte)
		rand.Read(seed[:])
		for i := range pat {
			pat[i] = seed[i] ^ 0x3c
		}
		tail := append([]byte(nil), pat[1:]...) // bytes 1..15, register id sits in byte 0
		if !probeFault(t, pat, tail) {
			t.Error("no fault")
			return
		}
		cur, g0s := runtime.ProbeStacks()
		t.Logf("%s/%s: %d Ms", runtime.GOOS, runtime.GOARCH, len(g0s))
		scanFor(t, "goroutine stack", cur[0], cur[1], tail)
		for _, s := range g0s {
			t.Logf("g0 [%#x,%#x) committed %v", s[0], s[1], committedRanges(s[0], s[1]))
			scanFor(t, "g0 stack, after recover", s[0], s[1], tail)
		}
	}()
	<-done
}
