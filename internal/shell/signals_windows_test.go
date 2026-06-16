//go:build windows

package shell

import "os"

var sigtermSignal os.Signal = os.Interrupt
