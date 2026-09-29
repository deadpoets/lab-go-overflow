// Temporary probe; copied into go/src/runtime by run.sh and removed after.

//go:build !windows

package runtime_test

func committedRanges(lo, hi uintptr) [][2]uintptr { return [][2]uintptr{{lo, hi}} }
