//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package executor

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

func defaultShell() string {
	return "/bin/sh"
}

func shellCommandArg() string {
	return "-c"
}

func configureCommandCancellation(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
}
