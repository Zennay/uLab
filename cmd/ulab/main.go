package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/Zennay/ulab/internal/config"
	"github.com/Zennay/ulab/internal/engine"
	"github.com/Zennay/ulab/internal/evidence"
	"github.com/Zennay/ulab/internal/executor"
	"github.com/Zennay/ulab/internal/matrix"
	"github.com/Zennay/ulab/internal/runner"
)

var (
	version = "dev"
	commit  = "unknown"
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
	case "version":
		fmt.Printf("ulab %s (%s)\n", version, commit)
		return
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
	fmt.Fprintln(os.Stderr, "usage: ulab <init|test|version>")
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
		Versions: config.Versions{From: config.VersionList{"v1.0.0"}, To: "current"},
		Setup:    config.Hook{Command: "./ulab/setup.sh"},
		Upgrade:  config.Hook{Command: "./ulab/upgrade.sh"},
		Verify:   config.Hook{Command: "./ulab/verify.sh"},
		Policy:   config.Policy{RequireAllPaths: true},
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runTestContext(ctx, args)
}

func runTestContext(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	configPath := fs.String("config", "ulab.json", "config path")
	jsonOut := fs.String("json-out", "ulab-result.json", "result path")
	evidenceRoot := fs.String("evidence-root", filepath.Join(".ulab", "runs"), "persistent evidence root")
	jobs := fs.Int("jobs", 1, "maximum concurrent upgrade paths")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *jobs < 1 {
		return errors.New("jobs must be at least 1")
	}

	startedAt := time.Now().UTC()
	configBytes, err := os.ReadFile(*configPath)
	if err != nil {
		return err
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	m := matrix.Matrix{
		Jobs: *jobs,
		RunnerFactory: func(plan engine.Plan) (runner.Runner, error) {
			return buildRunner(cfg, plan)
		},
	}
	result := m.Run(ctx, cfg.Plans())
	if err := evidence.WriteJSON(*jsonOut, result); err != nil {
		return err
	}

	invocationID := evidence.NewInvocationID(startedAt)
	workingDirectory, err := os.Getwd()
	if err != nil {
		return err
	}
	snapshotPath := filepath.Join(*evidenceRoot, invocationID, "config.json")
	reproduceCommand := []string{
		"ulab", "test",
		"--config", snapshotPath,
		"--jobs", strconv.Itoa(*jobs),
		"--evidence-root", *evidenceRoot,
	}
	paths, err := evidence.WriteBundle(evidence.BundleInput{
		Root:       *evidenceRoot,
		ID:         invocationID,
		ConfigPath: *configPath,
		Config:     configBytes,
		Result:     result,
		Metadata: evidence.BundleMetadata{
			CreatedAt:        startedAt,
			WorkingDirectory: workingDirectory,
			ReproduceCommand: reproduceCommand,
			Jobs:             *jobs,
			SourceVersions:   append([]string(nil), cfg.Versions.From...),
			TargetVersion:    cfg.Versions.To,
			ToolVersion:      version,
			ToolCommit:       commit,
			GoVersion:        runtime.Version(),
		},
	})
	if err != nil {
		return err
	}

	printMatrix(os.Stdout, result)
	fmt.Println("evidence:", paths.Dir)
	if ctx.Err() != nil {
		return fmt.Errorf("test run canceled: %w", ctx.Err())
	}
	if cfg.Policy.RequireAllPaths && result.Status == engine.StatusFailed {
		return errors.New("compatibility policy failed")
	}
	return nil
}

func printMatrix(w io.Writer, result matrix.Result) {
	fmt.Fprintf(w, "target %s: %s\n\n", result.TargetVersion, result.Status)
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "FROM\tTARGET\tRESULT")
	for _, run := range result.Runs {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", run.SourceVersion, run.TargetVersion, run.Status)
	}
	_ = tw.Flush()
}

func buildRunner(cfg config.Config, plan engine.Plan) (runner.Runner, error) {
	process := executor.Process{}
	switch cfg.Runner.Type {
	case "", "process":
		return runner.Process{Executor: process}, nil
	case "docker-compose":
		return runner.DockerCompose{
			Executor:    process,
			ComposeFile: cfg.Runner.ComposeFile,
			ProjectName: "ulab-" + sanitizeProjectName(plan.SourceVersion) + "-to-" + sanitizeProjectName(plan.TargetVersion),
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
