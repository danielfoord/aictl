package shell

import (
	"os"
	"os/signal"
	"sync"

	"golang.org/x/term"
)

var (
	isTerminal      = term.IsTerminal
	makeRaw         = term.MakeRaw
	restoreTerminal = term.Restore
)

func enterRawMode(stdin *os.File) (func(), error) {
	if stdin == nil {
		return func() {}, nil
	}
	fd := int(stdin.Fd())
	if !isTerminal(fd) {
		return func() {}, nil
	}

	state, err := makeRaw(fd)
	if err != nil {
		return func() {}, err
	}

	var once sync.Once
	return func() {
		once.Do(func() { _ = restoreTerminal(fd, state) })
	}, nil
}

// watchTerminalRestore proactively restores the terminal as soon as aictl
// receives an interrupt/termination signal, before any hard-termination path
// pre-empts the deferred restore. Because restore is idempotent, calling it
// here is safe alongside the normal deferred restore. This guarantees a second
// interrupt cannot find the terminal still in raw mode. When sigs is nil the
// watcher registers its own signal channel; tests inject a channel instead.
func watchTerminalRestore(restore func(), sigs chan os.Signal) func() {
	managed := sigs == nil
	if sigs == nil {
		sigs = make(chan os.Signal, 1)
		signal.Notify(sigs, interruptSignals...)
	}

	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-sigs:
				restore()
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
