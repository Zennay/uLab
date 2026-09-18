package config

import "testing"

func TestValidateRequiresUpgradeAndVerify(t *testing.T) {
	cfg := Config{Versions: Versions{From: "v1", To: "v2"}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}

	cfg.Upgrade.Command = "upgrade"
	cfg.Verify.Command = "verify"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestValidateDockerComposeRequiresComposeFile(t *testing.T) {
	cfg := Config{
		Runner:   Runner{Type: "docker-compose"},
		Versions: Versions{From: "v1", To: "v2"},
		Upgrade:  Hook{Command: "upgrade"},
		Verify:   Hook{Command: "verify"},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected compose file validation error")
	}
	cfg.Runner.ComposeFile = "compose.yaml"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
}
