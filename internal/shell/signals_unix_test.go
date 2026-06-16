//go:build !windows

package shell

import "syscall"

var sigtermSignal = syscall.SIGTERM
