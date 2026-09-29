// Temporary probe; copied into go/src/runtime by run.sh and removed after.

package runtime

// GsignalStats reports how many Ms exist and how many have a nil gsignal.
func GsignalStats() (ms, nils int) {
	lock(&sched.lock)
	for mp := allm; mp != nil; mp = mp.alllink {
		ms++
		if mp.gsignal == nil {
			nils++
		}
	}
	unlock(&sched.lock)
	return
}
