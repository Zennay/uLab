package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
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
	"github.com/Zennay/ulab/internal/webui"
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
	case "view":
		err = runView(os.Args[2:])
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
	fmt.Fprintln(os.Stderr, "usage: ulab <init|test|view|version>")
}

func runView(args []string) error {
	fs := flag.NewFlagSet("view", flag.ContinueOnError)
	evidenceRoot := fs.String("evidence-root", filepath.Join(".ulab", "runs"), "persistent evidence root")
	addr := fs.String("addr", "127.0.0.1:8080", "HTTP listen address")
	if err := fs.Parse(args); err != nil {
		return err
	}

	server := &http.Server{
		Addr:              *addr,
		Handler:           webui.Server{EvidenceRoot: *evidenceRoot}.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	fmt.Printf("uLab evidence UI: http://%s\n", *addr)
	return server.ListenAndServe()
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
	ctx, stop := interruptContext()
	defer stop()
	return runTestContext(ctx, args)
}

func interruptContext() (context.Context, context.CancelFunc) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ctx.Done()
		stop()
	}()
	return ctx, stop
}

type jobsOption struct {
	value int
	auto  bool
	set   bool
}

func (o jobsOption) String() string {
	if o.auto {
		return "auto"
	}
	return strconv.Itoa(o.value)
}

func (o *jobsOption) Set(raw string) error {
	o.set = true
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "auto" {
		o.auto = true
		o.value = 0
		return nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fmt.Errorf("jobs must be a positive integer or auto")
	}
	if value < 1 {
		return errors.New("jobs must be at least 1")
	}
	o.auto = false
	o.value = value
	return nil
}

func runTestContext(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	configPath := fs.String("config", "ulab.json", "config path")
	jsonOut := fs.String("json-out", "ulab-result.json", "result path")
	evidenceRoot := fs.String("evidence-root", filepath.Join(".ulab", "runs"), "persistent evidence root")
	jobs := jobsOption{value: 1}
	fs.Var(&jobs, "jobs", "maximum concurrent upgrade paths, or auto for RAM-aware concurrency")
	memoryReserveMB := fs.Uint64("memory-reserve-mb", 0, "RAM to keep available in auto mode (MB)")
	memoryPerJobMB := fs.Uint64("memory-per-job-mb", 0, "estimated RAM per test in auto mode (MB)")
	if err := fs.Parse(args); err != nil {
		return err
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

	autoJobs := jobs.auto
	jobCount := jobs.value
	if !jobs.set && cfg.Policy.AutoConcurrency {
		autoJobs = true
		jobCount = matrix.DefaultAutoJobs()
	}
	if autoJobs && jobCount < 1 {
		jobCount = matrix.DefaultAutoJobs()
	}

	memoryPolicy := matrix.DefaultMemoryPolicy()
	if cfg.Policy.MemoryReserveMB > 0 {
		memoryPolicy.ReserveBytes = cfg.Policy.MemoryReserveMB * 1024 * 1024
	}
	if cfg.Policy.MemoryPerJobMB > 0 {
		memoryPolicy.PerJobBytes = cfg.Policy.MemoryPerJobMB * 1024 * 1024
	}
	if *memoryReserveMB > 0 {
		memoryPolicy.ReserveBytes = *memoryReserveMB * 1024 * 1024
	}
	if *memoryPerJobMB > 0 {
		memoryPolicy.PerJobBytes = *memoryPerJobMB * 1024 * 1024
	}

	m := matrix.Matrix{
		Jobs:   jobCount,
		Auto:   autoJobs,
		Memory: memoryPolicy,
		RunnerFactory: func(plan engine.Plan) (runner.Runner, error) {
			return buildRunner(cfg, plan)
		},
	}
	result := m.Run(ctx, cfg.Plans())

	invocationID := evidence.NewInvocationID(startedAt)
	workingDirectory, err := os.Getwd()
	if err != nil {
		return err
	}
	snapshotPath := filepath.Join(*evidenceRoot, invocationID, "config.json")
	jobsMode := "fixed"
	jobsArg := strconv.Itoa(jobCount)
	if autoJobs {
		jobsMode = "auto"
		jobsArg = "auto"
	}
	reproduceCommand := []string{
		"ulab", "test",
		"--config", snapshotPath,
		"--jobs", jobsArg,
		"--evidence-root", *evidenceRoot,
	}
	if *memoryReserveMB > 0 {
		reproduceCommand = append(reproduceCommand, "--memory-reserve-mb", strconv.FormatUint(*memoryReserveMB, 10))
	}
	if *memoryPerJobMB > 0 {
		reproduceCommand = append(reproduceCommand, "--memory-per-job-mb", strconv.FormatUint(*memoryPerJobMB, 10))
	}
	paths, err := evidence.WriteBundle(evidence.BundleInput{
		Root:       *evidenceRoot,
		ID:         invocationID,
		ConfigPath: *configPath,
		Config:     configBytes,
		Result:     result,
	memoryReserveRecord := uint64(0)
	memoryPerJobRecord := uint64(0)
	if autoJobs {
		memoryReserveRecord = memoryPolicy.ReserveBytes / (1024 * 1024)
		memoryPerJobRecord = memoryPolicy.PerJobBytes / (1024 * 1024)
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
			Jobs:             jobCount,
			JobsMode:         jobsMode,
			MemoryReserveMB:  memoryReserveRecord,
			MemoryPerJobMB:   memoryPerJobRecord,
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
	if err := evidence.WriteJSON(*jsonOut, result); err != nil {
		return fmt.Errorf("write json output: %w", err)
	}
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
