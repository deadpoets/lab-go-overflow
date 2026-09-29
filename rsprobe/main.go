package main

import (
	"fmt"
	"runtime"
	"runtime/secret"
	"time"
)

var acc uint64

// spin burns CPU with no function call, so the only way the scheduler can take
// the goroutine off its P is asynchronous preemption.
//
//go:noinline
func spin(n int) {
	for i := 0; i < n; i++ {
		acc += uint64(i) ^ acc>>3
	}
}

// gcLatency times a stop-the-world while another goroutine spins. If that
// goroutine cannot be asynchronously preempted, the STW has to wait for the
// spin to reach a cooperative safepoint, so the latency is the spin.
func gcLatency(inSecret bool, iters int) time.Duration {
	done := make(chan struct{})
	go func() {
		defer close(done)
		if inSecret {
			secret.Do(func() { spin(iters) })
		} else {
			spin(iters)
		}
	}()
	time.Sleep(50 * time.Millisecond) // let the spin get going
	start := time.Now()
	runtime.GC()
	d := time.Since(start)
	<-done
	return d
}

func main() {
	fmt.Println("GOOS/GOARCH:", runtime.GOOS+"/"+runtime.GOARCH, "GOMAXPROCS:", runtime.GOMAXPROCS(0))
	start := time.Now()
	spin(200_000_000)
	fmt.Println("spin(200M) takes", time.Since(start).Round(time.Millisecond))
	secret.Do(func() { acc++ })
	fmt.Println("secret.Do ran")
	for range 3 {
		out := gcLatency(false, 200_000_000)
		in := gcLatency(true, 200_000_000)
		fmt.Printf("STW latency: spin outside secret.Do %8s | inside %8s\n", out.Round(time.Millisecond), in.Round(time.Millisecond))
	}
}
