//go:build windows

package executor

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
)

func defaultShell() string {
	return "cmd.exe"
}

func shellCommandArg() string {
	return "/C"
}

func configureCommandCancellation(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}

		treeKill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
		if err := treeKill.Run(); err == nil {
			return nil
		}

		if err := cmd.Process.Kill(); err != nil {
			if errors.Is(err, os.ErrProcessDone) {
				return os.ErrProcessDone
			}
			return err
		}
		return nil
	}
}
