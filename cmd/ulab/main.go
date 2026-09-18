package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/Zennay/ulab/internal/config"
	"github.com/Zennay/ulab/internal/engine"
	"github.com/Zennay/ulab/internal/evidence"
	"github.com/Zennay/ulab/internal/executor"
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
		Versions: config.Versions{From: "v1.0.0", To: "current"},
		Setup:    config.Hook{Command: "./ulab/setup.sh"},
		Upgrade:  config.Hook{Command: "./ulab/upgrade.sh"},
		Verify:   config.Hook{Command: "./ulab/verify.sh"},
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '
')
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

	runner := engine.Engine{Executor: executor.Process{}}
	result := runner.Run(context.Background(), cfg.Plan())
	if err := evidence.WriteJSON(*jsonOut, result); err != nil {
		return err
	}

	fmt.Printf("%s -> %s: %s
", result.SourceVersion, result.TargetVersion, result.Status)
	for _, phase := range result.Phases {
		fmt.Printf("  %-7s %s
", phase.Phase, phase.Status)
	}
	if result.Status == engine.StatusFailed {
		return errors.New("upgrade path failed")
	}
	return nil
}
