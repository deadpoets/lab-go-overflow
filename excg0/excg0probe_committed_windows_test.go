// Temporary probe; copied into go/src/runtime by run.sh and removed after.

package runtime_test

import (
	"syscall"
	"unsafe"
)

var procVirtualQuery = syscall.NewLazyDLL("kernel32.dll").NewProc("VirtualQuery")

// committedRanges returns the committed, non-guard, readable parts of [lo, hi).
func committedRanges(lo, hi uintptr) (out [][2]uintptr) {
	type mbi struct {
		base, allocBase          uintptr
		allocProtect, partId     uint32
		regionSize               uintptr
		state, protect, typ, pad uint32
	}
	for a := lo; a < hi; {
		var m mbi
		r, _, _ := procVirtualQuery.Call(a, uintptr(unsafe.Pointer(&m)), unsafe.Sizeof(m))
		if r == 0 {
			break
		}
		end := min(m.base+m.regionSize, hi)
		const memCommit, pageGuard, pageNoAccess = 0x1000, 0x100, 0x01
		if m.state == memCommit && m.protect&(pageGuard|pageNoAccess) == 0 {
			out = append(out, [2]uintptr{max(a, m.base), end})
		}
		a = end
	}
	return out
}
