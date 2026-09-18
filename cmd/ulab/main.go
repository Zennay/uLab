package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Zennay/ulab/internal/config"
	"github.com/Zennay/ulab/internal/engine"
	"github.com/Zennay/ulab/internal/evidence"
	"github.com/Zennay/ulab/internal/executor"
	"github.com/Zennay/ulab/internal/runner"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "init":
		err = runInit(os.Args[2:])
	case "test":
		err = runTest(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "ulab:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: ulab <init|test>")
}

func runInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	path := fs.String("config", "ulab.json", "config path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if _, err := os.Stat(*path); err == nil {
		return fmt.Errorf("%s already exists", *path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	cfg := config.Config{
		Runner:   config.Runner{Type: "process"},
		Versions: config.Versions{From: "v1.0.0", To: "current"},
		Setup:    config.Hook{Command: "./ulab/setup.sh"},
		Upgrade:  config.Hook{Command: "./ulab/upgrade.sh"},
		Verify:   config.Hook{Command: "./ulab/verify.sh"},
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.WriteFile(*path, data, 0o644); err != nil {
		return err
	}
	fmt.Println("created", *path)
	return nil
}

func runTest(args []string) error {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	configPath := fs.String("config", "ulab.json", "config path")
	jsonOut := fs.String("json-out", "ulab-result.json", "result path")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	selectedRunner, err := buildRunner(cfg)
	if err != nil {
		return err
	}
	result := engine.Engine{Runner: selectedRunner}.Run(context.Background(), cfg.Plan())
	if err := evidence.WriteJSON(*jsonOut, result); err != nil {
		return err
	}

	fmt.Printf("%s -> %s: %s\n", result.SourceVersion, result.TargetVersion, result.Status)
	for _, phase := range result.Phases {
		fmt.Printf("  %-8s %s\n", phase.Phase, phase.Status)
	}
	if result.Status == engine.StatusFailed {
		return errors.New("upgrade path failed")
	}
	return nil
}

func buildRunner(cfg config.Config) (runner.Runner, error) {
	process := executor.Process{}
	switch cfg.Runner.Type {
	case "", "process":
		return runner.Process{Executor: process}, nil
	case "docker-compose":
		return runner.DockerCompose{
			Executor:    process,
			ComposeFile: cfg.Runner.ComposeFile,
			ProjectName: "ulab-" + sanitizeProjectName(cfg.Versions.From) + "-to-" + sanitizeProjectName(cfg.Versions.To),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported runner %q", cfg.Runner.Type)
	}
}

func sanitizeProjectName(value string) string {
	value = strings.ToLower(value)
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-_")
}
