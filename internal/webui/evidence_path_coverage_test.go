package webui

import (
	"encoding/json"
	"testing"

	"github.com/Zennay/ulab/internal/config"
	"github.com/Zennay/ulab/internal/engine"
	"github.com/Zennay/ulab/internal/evidence"
	"github.com/Zennay/ulab/internal/matrix"
)

func writeCoverageBundle(t *testing.T, sources []string, target string, runs []engine.RunResult, status engine.Status) Bundle {
	t.Helper()
	cfg := config.Config{
		Versions: config.Versions{From: config.VersionList(sources), To: target},
		Upgrade:  config.Hook{Command: "./upgrade.sh"},
		Verify:   config.Hook{Command: "./verify.sh"},
		Policy:   config.Policy{RequireAllPaths: true},
	}
	cfgBytes, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	_, err = evidence.WriteBundle(evidence.BundleInput{
		Root: root, ID: "run",
		Config:   cfgBytes,
		Result:   matrix.Result{TargetVersion: target, Status: status, Runs: runs},
		Metadata: evidence.BundleMetadata{SourceVersions: sources, TargetVersion: target},
	})
	if err != nil {
		t.Fatal(err)
	}
	bundles, err := LoadBundles(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(bundles) != 1 {
		t.Fatalf("bundle count=%d", len(bundles))
	}
	return bundles[0]
}

func TestMissingRequiredUpgradePathMustNotBeReady(t *testing.T) {
	b := writeCoverageBundle(t, []string{"v1", "v2"}, "v3",
		[]engine.RunResult{{SourceVersion: "v1", TargetVersion: "v3", Status: engine.StatusPassed}},
		engine.StatusPassed)
	if got := releaseReadiness(b).Label; got == "READY" {
		t.Fatal("one passing path was treated READY despite required v2->v3 missing")
	}
}

func TestDuplicateResultSourceCannotReplaceMissingPath(t *testing.T) {
	b := writeCoverageBundle(t, []string{"v1", "v2"}, "v3",
		[]engine.RunResult{
			{SourceVersion: "v1", TargetVersion: "v3", Status: engine.StatusPassed},
			{SourceVersion: "v1", TargetVersion: "v3", Status: engine.StatusPassed},
		}, engine.StatusPassed)
	if got := releaseReadiness(b).Label; got == "READY" {
		t.Fatal("duplicate v1 result impersonated required v2 coverage")
	}
}

func TestUnexpectedPassedTargetDoesNotCountAsRequiredUpgradePath(t *testing.T) {
	b := writeCoverageBundle(t, []string{"v1"}, "v3",
		[]engine.RunResult{{SourceVersion: "v1", TargetVersion: "v2", Status: engine.StatusPassed}},
		engine.StatusPassed)
	if got := releaseReadiness(b).Label; got == "READY" {
		t.Fatal("passing result for wrong target shown as release READY")
	}
}

func TestExactMatchingUpgradePathKeepsReady(t *testing.T) {
	b := writeCoverageBundle(t, []string{"v1", "v2"}, "v3",
		[]engine.RunResult{
			{SourceVersion: "v1", TargetVersion: "v3", Status: engine.StatusPassed, Phases: phaseInventory(engine.PhasePrepare, engine.PhaseUpgrade, engine.PhaseVerify, engine.PhaseCleanup)},
			{SourceVersion: "v2", TargetVersion: "v3", Status: engine.StatusPassed, Phases: phaseInventory(engine.PhasePrepare, engine.PhaseUpgrade, engine.PhaseVerify, engine.PhaseCleanup)},
		}, engine.StatusPassed)
	if got := releaseReadiness(b).Label; got != "READY" {
		t.Fatalf("complete matching upgrade paths lost READY: %q", got)
	}
}

func TestFrozenMetadataCannotMisstateRequiredPaths(t *testing.T) {
	baseline := writeCoverageBundle(t, []string{"v1", "v2"}, "v3",
		[]engine.RunResult{
			{SourceVersion: "v1", TargetVersion: "v3", Status: engine.StatusPassed},
			{SourceVersion: "v2", TargetVersion: "v3", Status: engine.StatusPassed},
		}, engine.StatusPassed)
	for _, tc := range []struct {
		name   string
		mutate func(*Bundle)
	}{
		{"wrong metadata target", func(b *Bundle) { b.Metadata.TargetVersion = "v2" }},
		{"missing metadata source", func(b *Bundle) { b.Metadata.SourceVersions = []string{"v1"} }},
		{"duplicate metadata source", func(b *Bundle) { b.Metadata.SourceVersions = []string{"v1", "v1"} }},
		{"wrong aggregate result target", func(b *Bundle) { b.Result.TargetVersion = "v2" }},
		{"unrecognized result source", func(b *Bundle) { b.Result.Runs[0].SourceVersion = "v9" }},
		{"extra result run", func(b *Bundle) {
			b.Result.Runs = append(b.Result.Runs, engine.RunResult{SourceVersion: "v9", TargetVersion: "v3", Status: engine.StatusPassed})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := baseline
			b.Metadata.SourceVersions = append([]string(nil), baseline.Metadata.SourceVersions...)
			b.Result.Runs = append([]engine.RunResult(nil), baseline.Result.Runs...)
			tc.mutate(&b)
			if got := releaseReadiness(b).Label; got != "UNVERIFIED" {
				t.Fatalf("inconsistent trusted metadata got %s; want UNVERIFIED", got)
			}
		})
	}
}

func TestPartialCoverageRemainsUnverifiedWhenPolicyDoesNotRequireAll(t *testing.T) {
	b := writeCoverageBundle(t, []string{"v1", "v2"}, "v3",
		[]engine.RunResult{{SourceVersion: "v1", TargetVersion: "v3", Status: engine.StatusPassed}}, engine.StatusPassed)
	b.Config.Policy.RequireAllPaths = false
	if got := releaseReadiness(b).Label; got != "UNVERIFIED" {
		t.Fatalf("partial coverage with relaxed policy got %s", got)
	}
}
