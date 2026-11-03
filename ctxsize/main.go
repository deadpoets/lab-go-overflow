// ctxsize reports what InitializeContext does with a nil buffer for several
// ContextFlags values on this host: the size it reports and the error it
// sets. It exists to settle what "the unsupported-XSTATE case" looks like
// in practice on each architecture and kernel.
package main

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

func main() {
	k32 := syscall.NewLazyDLL("kernel32.dll")
	ic := k32.NewProc("InitializeContext")
	gx := k32.NewProc("GetEnabledXStateFeatures")
	var arch uint32
	switch runtime.GOARCH {
	case "amd64":
		arch = 0x100000
	case "386":
		arch = 0x10000
	case "arm64":
		arch = 0x400000
	}
	fmt.Println(runtime.GOOS+"/"+runtime.GOARCH, "arch bits", fmt.Sprintf("%#x", arch))
	if gx.Find() == nil {
		m, _, _ := gx.Call()
		fmt.Printf("GetEnabledXStateFeatures = %#x\n", m)
	} else {
		fmt.Println("GetEnabledXStateFeatures: not present")
	}
	type f struct {
		name  string
		flags uint32
	}
	var cases []f
	switch runtime.GOARCH {
	case "amd64":
		cases = []f{{"CONTROL", 0x1}, {"ALL", 0x1f}, {"ALL|XSTATE(0x40)", 0x5f}, {"ALL|0x80 (unknown bit)", 0x9f}, {"ALL|0x100 (unknown bit)", 0x11f}, {"XSTATE alone", 0x40}}
	case "386":
		cases = []f{{"CONTROL", 0x1}, {"ALL", 0x3f}, {"ALL|XSTATE(0x40)", 0x7f}, {"ALL|0x80 (unknown bit)", 0xbf}}
	case "arm64":
		cases = []f{{"CONTROL", 0x3}, {"ALL(0x1f)", 0x1f}, {"ALL|XSTATE(0x20)", 0x3f}, {"ALL|FP_LOW(0x40)", 0x5f}, {"ALL|0x80 (unknown bit)", 0x9f}, {"ALL|0x100 (unknown bit)", 0x11f}, {"XSTATE alone", 0x20}}
	}
	cases = append(cases, f{"wrong arch bits (0x7f0000|1)", 0x7f0001}, f{"zero", 0})
	for _, c := range cases {
		var n uint32
		flags := c.flags
		if c.name != "wrong arch bits (0x7f0000|1)" && c.name != "zero" {
			flags |= arch
		}
		r, _, e := ic.Call(0, uintptr(flags), 0, uintptr(unsafe.Pointer(&n)))
		errno := e.(syscall.Errno)
		fmt.Printf("  %-30s flags=%#08x ret=%d errno=%d (%s) n=%d\n", c.name, flags, r, uintptr(errno), errno.Error(), n)
	}
}
