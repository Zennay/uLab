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

func TestRuntimeProofPreservesAuditMetadata(t *testing.T) {
	data, err := os.ReadFile("../../scripts/validate-gitea-reference.sh")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"preserve_work_artifacts",
		"proof-summary.json",
		"docker image inspect",
		"docker_server",
		"durations_seconds",
		"PROOF.txt",
		"json.Unmarshal",
		"SHA256SUMS",
		"PROOF_COMPLETE",
		"verify-gitea-proof.sh",
		"Gitea runtime proof requires a clean working tree",
		"Gitea runtime proof must run from canonical main",
		"a reachable Docker daemon is required for the Gitea runtime proof",
		"no immutable repo digest recorded for Gitea",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("runtime proof does not preserve audit contract %q", want)
		}
	}
}

func TestReferenceAssertsLiveSourceAndTargetVersions(t *testing.T) {
	setup, err := os.ReadFile("scripts/setup.sh")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(setup), `assert_gitea_version "$ULAB_SOURCE_VERSION"`) {
		t.Fatal("setup must assert the live source release before seeding state")
	}

	verify, err := os.ReadFile("scripts/verify.sh")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(verify), `assert_gitea_version "$ULAB_TARGET_VERSION"`) {
		t.Fatal("verify must assert the live target release after upgrade")
	}

	common, err := os.ReadFile("scripts/common.sh")
	if err != nil {
		t.Fatal(err)
	}
	commonText := string(common)
	if !strings.Contains(commonText, `grep -Fq "\"version\":\"$expected_version\""`) {
		t.Fatal("version assertion must match the complete reported release value")
	}
	if !strings.Contains(commonText, "verified live Gitea version: $expected_version") {
		t.Fatal("successful live-version assertions must be visible in run evidence")
	}
}

func TestArchivedProofVerifierChecksIntegrityAndBundleLinkage(t *testing.T) {
	data, err := os.ReadFile("../../scripts/verify-gitea-proof.sh")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"sha256sum -c SHA256SUMS",
		"manifest_sha256",
		"config hash mismatch",
		"tool_commit",
		"json_status",
		"expected one passed and one failed invocation bundle",
		"verified live Gitea version: 1.26.0",
		"verified live Gitea version: 1.27.3",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("archived proof verifier does not enforce %q", want)
		}
	}
}
