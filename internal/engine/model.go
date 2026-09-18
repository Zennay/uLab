package engine

import "time"

type Phase string

const (
	PhasePrepare Phase = "prepare"
	PhaseSetup   Phase = "setup"
	PhaseUpgrade Phase = "upgrade"
	PhaseVerify  Phase = "verify"
	PhaseCleanup Phase = "cleanup"
)

type Status string

const (
	StatusPassed   Status = "passed"
	StatusFailed   Status = "failed"
	StatusCanceled Status = "canceled"
)

type FailureKind string

const (
	FailureNone     FailureKind = ""
	FailureHook     FailureKind = "hook_failed"
	FailureRunner   FailureKind = "runner_failed"
	FailureCanceled FailureKind = "canceled"
)

type Plan struct {
	SourceVersion  string
	TargetVersion  string
	SetupCommand   string
	UpgradeCommand string
	VerifyCommand  string
}

type PhaseResult struct {
	Phase     Phase         `json:"phase"`
	Status    Status        `json:"status"`
	Command   string        `json:"command,omitempty"`
	StartedAt time.Time     `json:"started_at"`
	Duration  time.Duration `json:"duration_ns"`
	Output    string        `json:"output,omitempty"`
	Error     string        `json:"error,omitempty"`
}

type RunResult struct {
	RunID         string        `json:"run_id"`
	SourceVersion string        `json:"source_version"`
	TargetVersion string        `json:"target_version"`
	Status        Status        `json:"status"`
	FailureKind   FailureKind   `json:"failure_kind,omitempty"`
	StartedAt     time.Time     `json:"started_at"`
	Duration      time.Duration `json:"duration_ns"`
	Phases        []PhaseResult `json:"phases"`
}
