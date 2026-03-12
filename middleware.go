package cadence

import (
	"fmt"
	"runtime"
	"sync"
)

// Recover wraps a Job to recover from panics, logging the panic value
// and a stack trace.
func Recover(logger Logger) JobWrapper {
	return func(j Job) Job {
		return FuncJob(func() {
			defer func() {
				if r := recover(); r != nil {
					const size = 64 << 10
					buf := make([]byte, size)
					n := runtime.Stack(buf, false)
					logger.Error(
						fmt.Errorf("%v", r),
						"panic",
						"stack", string(buf[:n]),
					)
				}
			}()
			j.Run()
		})
	}
}

// SkipIfStillRunning skips the job invocation if the previous run is
// still in progress. Only one instance of the job will run at a time.
func SkipIfStillRunning(logger Logger) JobWrapper {
	return func(j Job) Job {
		var mu sync.Mutex
		running := false
		return FuncJob(func() {
			mu.Lock()
			if running {
				mu.Unlock()
				logger.Info("skip", "reason", "still running")
				return
			}
			running = true
			mu.Unlock()

			defer func() {
				mu.Lock()
				running = false
				mu.Unlock()
			}()
			j.Run()
		})
	}
}

// DelayIfStillRunning delays the job invocation until the previous run
// completes. It serialises overlapping runs using a mutex.
func DelayIfStillRunning(logger Logger) JobWrapper {
	return func(j Job) Job {
		var mu sync.Mutex
		return FuncJob(func() {
			mu.Lock()
			defer mu.Unlock()
			j.Run()
		})
	}
}
