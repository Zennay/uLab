package executor

import "context"

type Result struct {
	Output string
}

type Executor interface {
	Run(ctx context.Context, command string, env map[string]string) (Result, error)
}
