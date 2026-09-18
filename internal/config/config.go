package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/Zennay/ulab/internal/engine"
)

type Hook struct {
	Command string `json:"command"`
}

type VersionList []string

func (v *VersionList) UnmarshalJSON(data []byte) error {
	var many []string
	if err := json.Unmarshal(data, &many); err == nil {
		*v = many
		return nil
	}

	var one string
	if err := json.Unmarshal(data, &one); err != nil {
		return errors.New("versions.from must be a string or array of strings")
	}
	*v = VersionList{one}
	return nil
}

type Versions struct {
	From VersionList `json:"from"`
	To   string      `json:"to"`
}

type Runner struct {
	Type        string `json:"type,omitempty"`
	ComposeFile string `json:"compose_file,omitempty"`
}

type Policy struct {
	RequireAllPaths bool `json:"require_all_paths"`
}

type Config struct {
	Runner   Runner   `json:"runner,omitempty"`
	Versions Versions `json:"versions"`
	Setup    Hook     `json:"setup"`
	Upgrade  Hook     `json:"upgrade"`
	Verify   Hook     `json:"verify"`
	Policy   Policy   `json:"policy,omitempty"`
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if len(c.Versions.From) == 0 {
		return errors.New("versions.from requires at least one source version")
	}
	seenSources := make(map[string]struct{}, len(c.Versions.From))
	for i, version := range c.Versions.From {
		if version == "" {
			return fmt.Errorf("versions.from[%d] cannot be empty", i)
		}
		if _, exists := seenSources[version]; exists {
			return fmt.Errorf("versions.from[%d] duplicates source version %q", i, version)
		}
		seenSources[version] = struct{}{}
	}
	if c.Versions.To == "" {
		return errors.New("versions.to is required")
	}
	if c.Upgrade.Command == "" {
		return errors.New("upgrade.command is required")
	}
	if c.Verify.Command == "" {
		return errors.New("verify.command is required")
	}
	switch c.Runner.Type {
	case "", "process":
	case "docker-compose":
		if c.Runner.ComposeFile == "" {
			return errors.New("runner.compose_file is required for docker-compose")
		}
	default:
		return fmt.Errorf("unsupported runner.type %q", c.Runner.Type)
	}
	return nil
}

func (c Config) Plans() []engine.Plan {
	plans := make([]engine.Plan, 0, len(c.Versions.From))
	for _, source := range c.Versions.From {
		plans = append(plans, engine.Plan{
			SourceVersion:  source,
			TargetVersion:  c.Versions.To,
			SetupCommand:   c.Setup.Command,
			UpgradeCommand: c.Upgrade.Command,
			VerifyCommand:  c.Verify.Command,
		})
	}
	return plans
}
