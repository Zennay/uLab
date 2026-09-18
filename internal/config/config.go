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

type Versions struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type Config struct {
	Versions Versions `json:"versions"`
	Setup    Hook     `json:"setup"`
	Upgrade  Hook     `json:"upgrade"`
	Verify   Hook     `json:"verify"`
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
	if c.Versions.From == "" {
		return errors.New("versions.from is required")
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
	return nil
}

func (c Config) Plan() engine.Plan {
	return engine.Plan{
		SourceVersion:  c.Versions.From,
		TargetVersion:  c.Versions.To,
		SetupCommand:   c.Setup.Command,
		UpgradeCommand: c.Upgrade.Command,
		VerifyCommand:  c.Verify.Command,
	}
}
