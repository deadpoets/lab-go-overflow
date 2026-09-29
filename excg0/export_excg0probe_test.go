// Temporary probe; copied into go/src/runtime by run.sh and removed after.

package runtime

func probeLoadAndFault(p *[16]byte)

func ProbeLoadAndFault(p *[16]byte) { probeLoadAndFault(p) }

// ProbeStacks returns the current goroutine's stack and every M's g0 stack.
func ProbeStacks() (cur [2]uintptr, g0s [][2]uintptr) {
	gp := getg()
	cur = [2]uintptr{gp.stack.lo, gp.stack.hi}
	lock(&sched.lock)
	for mp := allm; mp != nil; mp = mp.alllink {
		if mp.g0 != nil {
			g0s = append(g0s, [2]uintptr{mp.g0.stack.lo, mp.g0.stack.hi})
		}
	}
	unlock(&sched.lock)
	return
}
