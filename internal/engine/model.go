package engine

import "time"

type Phase string

const (
	PhaseSetup   Phase = "setup"
	PhaseUpgrade Phase = "upgrade"
	PhaseVerify  Phase = "verify"
)

type Status string

const (
	StatusPassed Status = "passed"
	StatusFailed Status = "failed"
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
	Command   string        `json:"command"`
	StartedAt time.Time     `json:"started_at"`
	Duration  time.Duration `json:"duration_ns"`
	Output    string        `json:"output,omitempty"`
	Error     string        `json:"error,omitempty"`
}

type RunResult struct {
	SourceVersion string        `json:"source_version"`
	TargetVersion string        `json:"target_version"`
	Status        Status        `json:"status"`
	StartedAt     time.Time     `json:"started_at"`
	Duration      time.Duration `json:"duration_ns"`
	Phases        []PhaseResult `json:"phases"`
}
