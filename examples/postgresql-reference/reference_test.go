package postgresqlreference

import (
	"os"
	"strings"
	"testing"

	"github.com/Zennay/ulab/internal/config"
)

func TestReferenceConfigUsesContrastingPostgreSQLMajors(t *testing.T) {
	cfg, err := config.Load("ulab.json")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Runner.Type != "docker-compose" {
		t.Fatalf("runner = %q", cfg.Runner.Type)
	}
	if cfg.Runner.ComposeFile != "examples/postgresql-reference/compose.yaml" {
		t.Fatalf("compose file = %q", cfg.Runner.ComposeFile)
	}
	if len(cfg.Versions.From) != 2 || cfg.Versions.From[0] != "16.15" || cfg.Versions.From[1] != "17.11" {
		t.Fatalf("source versions = %#v", cfg.Versions.From)
	}
	if cfg.Versions.To != "18.6" {
		t.Fatalf("target version = %q", cfg.Versions.To)
	}
}

func TestComposeKeepsSourceTargetAndTransferStateSeparate(t *testing.T) {
	data, err := os.ReadFile("compose.yaml")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"postgres:${ULAB_SOURCE_VERSION",
		"postgres:${ULAB_TARGET_VERSION",
		"source-data:/var/lib/postgresql/data",
		"target-data:/var/lib/postgresql",
		"transfer:/transfer",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("compose file does not contain %q", want)
		}
	}
}

func TestHooksStayProjectOwnedAndPerformLogicalMigration(t *testing.T) {
	for _, name := range []string{"setup.sh", "upgrade.sh", "verify.sh", "verify-broken.sh"} {
		data, err := os.ReadFile("scripts/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "internal/") {
			t.Fatalf("%s reaches into uLab internals", name)
		}
	}

	setup, err := os.ReadFile("scripts/setup.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"pg_dump",
		"/transfer/upgrade.dump",
		"CREATE TABLE accounts",
		"CREATE VIEW account_totals",
	} {
		if !strings.Contains(string(setup), want) {
			t.Fatalf("setup does not contain %q", want)
		}
	}

	upgrade, err := os.ReadFile("scripts/upgrade.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"docker compose stop source", "pg_restore", "--no-owner"} {
		if !strings.Contains(string(upgrade), want) {
			t.Fatalf("upgrade does not contain %q", want)
		}
	}
}

func TestVerificationChecksRelationalInvariantsAndPostMigrationSequence(t *testing.T) {
	data, err := os.ReadFile("scripts/verify.sh")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"assert_postgres_version target",
		"JOIN accounts",
		"héllo 🌍",
		"20.50",
		"account_totals",
		"RETURNING id",
		"expected restored sequence to continue at 3",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("verify does not contain %q", want)
		}
	}

	broken, err := os.ReadFile("scripts/verify-broken.sh")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(broken), "DELETE FROM events") {
		t.Fatal("broken verifier must corrupt an invariant after migration")
	}
}
