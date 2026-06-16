package shell

import (
	"os"
	"os/signal"
	"sync"

	"github.com/creack/pty"
)

var inheritPTYSize = pty.InheritSize

func startResizeWatcher(tty, ptmx *os.File, sigs chan os.Signal) func() {
	if tty == nil || ptmx == nil {
		return func() {}
	}

	_ = inheritPTYSize(tty, ptmx)

	managed := sigs == nil
	if sigs == nil {
		sigs = make(chan os.Signal, 1)
		signal.Notify(sigs, winchSignal)
	}

	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-sigs:
				_ = inheritPTYSize(tty, ptmx)
			case <-done:
				return
			}
		}
	}()

	var once sync.Once
	return func() {
		once.Do(func() {
			if managed {
				signal.Stop(sigs)
			}
			close(done)
		})
	}
}
