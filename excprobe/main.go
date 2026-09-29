// excprobe checks where Windows writes the register state of a faulting
// goroutine: it loads a pattern into XMM registers only, faults, recovers,
// and scans the goroutine's own stack below the recovering frame for it.
package main

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"os"
	"runtime"
	"unsafe"
)

func loadAndFault(p *[16]byte)

//go:noinline
func grow(n int) byte {
	var pad [1024]byte
	pad[n%1024] = byte(n)
	if n == 0 {
		return pad[0]
	}
	return grow(n-1) + pad[n%1024]
}

//go:noinline
func faultWith(p *[16]byte) (recovered bool) {
	defer func() { recovered = recover() != nil }()
	loadAndFault(p)
	return false
}

func trial() (found []int) {
	grow(256) // ensure the stack extends far below the frames we use
	// Keep every copy of the pattern off the stack: seed and pattern are
	// heap allocated, and the pattern is written byte by byte.
	seed, pat, buf := new([16]byte), new([16]byte), new([16]byte)
	rand.Read(seed[:])
	for i := range pat {
		pat[i] = seed[i] ^ 0x5c
		buf[i] = seed[i] ^ 0x5c
	}
	var anchor byte
	if !faultWith(buf) {
		fmt.Println("no fault?")
		os.Exit(2)
	}
	top := uintptr(unsafe.Pointer(&anchor))
	const window = 64 << 10
	mem := unsafe.Slice((*byte)(unsafe.Pointer(top-window)), window)
	for i := 0; ; {
		j := bytes.Index(mem[i:], pat[:])
		if j < 0 {
			break
		}
		found = append(found, window-(i+j)) // bytes below anchor
		i += j + 1
	}
	runtime.KeepAlive(&anchor)
	return found
}

func main() {
	fmt.Println(runtime.GOOS + "/" + runtime.GOARCH)
	c := make(chan []int)
	for i := range 5 {
		go func() { c <- trial() }()
		f := <-c
		fmt.Printf("trial %d: %d copies of the register-only pattern in the goroutine stack, at %v bytes below the recovering frame\n", i, len(f), f)
	}
}
