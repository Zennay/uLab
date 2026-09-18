package config

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestValidateRequiresUpgradeAndVerify(t *testing.T) {
	cfg := Config{Versions: Versions{From: VersionList{"v1"}, To: "v2"}}
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
		Versions: Versions{From: VersionList{"v1"}, To: "v2"},
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

func TestVersionListAcceptsStringAndArray(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want VersionList
	}{
		{name: "string", raw: `{"versions":{"from":"v1","to":"v2"},"upgrade":{"command":"u"},"verify":{"command":"v"}}`, want: VersionList{"v1"}},
		{name: "array", raw: `{"versions":{"from":["v1","v1.1"],"to":"v2"},"upgrade":{"command":"u"},"verify":{"command":"v"}}`, want: VersionList{"v1", "v1.1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var cfg Config
			if err := json.Unmarshal([]byte(tc.raw), &cfg); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(cfg.Versions.From, tc.want) {
				t.Fatalf("from = %#v, want %#v", cfg.Versions.From, tc.want)
			}
		})
	}
}

func TestPlansPreserveSourceOrder(t *testing.T) {
	cfg := Config{
		Versions: Versions{From: VersionList{"v1", "v1.1", "v1.2"}, To: "v2"},
		Upgrade:  Hook{Command: "upgrade"},
		Verify:   Hook{Command: "verify"},
	}
	plans := cfg.Plans()
	got := []string{plans[0].SourceVersion, plans[1].SourceVersion, plans[2].SourceVersion}
	want := []string{"v1", "v1.1", "v1.2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sources = %#v, want %#v", got, want)
	}
}

func TestValidateRejectsDuplicateSourceVersions(t *testing.T) {
	cfg := Config{
		Versions: Versions{From: VersionList{"v1", "v1"}, To: "v2"},
		Upgrade:  Hook{Command: "upgrade"},
		Verify:   Hook{Command: "verify"},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected duplicate source version to be rejected")
	}
}
