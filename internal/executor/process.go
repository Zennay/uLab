package executor

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
)

type Process struct {
	Shell string
}

func (p Process) Run(ctx context.Context, command string, env map[string]string) (Result, error) {
	shell := p.Shell
	if shell == "" {
		shell = defaultShell()
	}

	cmd := exec.CommandContext(ctx, shell, shellCommandArg(), command)
	configureCommandCancellation(cmd)
	cmd.Env = append([]string{}, os.Environ()...)
	for key, value := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	err := cmd.Run()
	return Result{Output: output.String()}, err
}
