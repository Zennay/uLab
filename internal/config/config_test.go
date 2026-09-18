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
