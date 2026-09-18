package giteareference

import (
	"os"
	"strings"
	"testing"

	"github.com/Zennay/ulab/internal/config"
)

func TestReferenceConfigStaysOutsideCoreAndUsesRealGiteaVersions(t *testing.T) {
	cfg, err := config.Load("ulab.json")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Runner.Type != "docker-compose" {
		t.Fatalf("runner = %q", cfg.Runner.Type)
	}
	if cfg.Runner.ComposeFile != "examples/gitea-reference/compose.yaml" {
		t.Fatalf("compose file = %q", cfg.Runner.ComposeFile)
	}
	if len(cfg.Versions.From) != 2 || cfg.Versions.From[0] != "1.26.0" || cfg.Versions.From[1] != "1.26.4" {
		t.Fatalf("source versions = %#v", cfg.Versions.From)
	}
	if cfg.Versions.To != "1.27.3" {
		t.Fatalf("target version = %q", cfg.Versions.To)
	}

	compose, err := os.ReadFile("compose.yaml")
	if err != nil {
		t.Fatal(err)
	}
	text := string(compose)
	for _, want := range []string{
		"docker.gitea.com/gitea:",
		"GITEA_IMAGE_TAG",
		"sqlite3",
		"gitea-data:/data",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("compose file does not contain %q", want)
		}
	}
}

func TestReferenceHooksUseOnlyPublicULabContract(t *testing.T) {
	for _, name := range []string{"setup.sh", "upgrade.sh", "verify.sh"} {
		data, err := os.ReadFile("scripts/" + name)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		if strings.Contains(text, "internal/") {
			t.Fatalf("%s reaches into uLab internals", name)
		}
	}
}

func TestSetupRunsGiteaAdminCLIAsContainerGitUser(t *testing.T) {
	data, err := os.ReadFile("scripts/setup.sh")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "docker compose exec -T --user git server") {
		t.Fatal("setup must run the Gitea admin CLI as the container git user")
	}
}
