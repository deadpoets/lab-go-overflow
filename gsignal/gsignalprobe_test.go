// Temporary probe; copied into go/src/runtime by run.sh and removed after.

package runtime_test

import (
	"runtime"
	"sync"
	"testing"
)

func TestGsignalProbe(t *testing.T) {
	// Create a few extra Ms by blocking locked threads.
	var wg sync.WaitGroup
	release := make(chan struct{})
	for range 4 {
		wg.Add(1)
		go func() {
			runtime.LockOSThread()
			wg.Done()
			<-release
		}()
	}
	wg.Wait()
	ms, nils := runtime.GsignalStats()
	close(release)
	t.Logf("%s/%s: %d Ms, %d with nil gsignal", runtime.GOOS, runtime.GOARCH, ms, nils)
}
