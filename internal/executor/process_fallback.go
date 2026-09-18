//go:build plan9 || js || wasip1

package executor

import "os/exec"

func defaultShell() string {
	return "/bin/sh"
}

func shellCommandArg() string {
	return "-c"
}

func configureCommandCancellation(_ *exec.Cmd) {}
