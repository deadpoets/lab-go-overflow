package main

import (
	"fmt"

	"golang.org/x/sys/cpu"
)

func main() {
	fmt.Println("AVX", cpu.X86.HasAVX, "AVX2", cpu.X86.HasAVX2, "AVX512F", cpu.X86.HasAVX512F)
}
