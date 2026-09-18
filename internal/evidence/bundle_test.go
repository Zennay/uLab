package evidence

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestWriteBundlePersistsExactConfigResultAndMetadata(t *testing.T) {
	root := t.TempDir()
	config := []byte("{\n  \"versions\": {\"from\": \"v1\", \"to\": \"v2\"}\n}\n")
	created := time.Date(2026, 9, 18, 11, 30, 0, 0, time.FixedZone("test", 2*60*60))

	paths, err := WriteBundle(BundleInput{
		Root:       root,
		ID:         "run-123",
		ConfigPath: "ulab.json",
		Config:     config,
		Result:     map[string]any{"status": "passed"},
		Metadata: BundleMetadata{
			CreatedAt:        created,
			WorkingDirectory: "/work/repo",
			ReproduceCommand: []string{"ulab", "test", "--config", ".ulab/runs/run-123/config.json"},
			Jobs:             2,
			SourceVersions:   []string{"v1"},
			TargetVersion:    "v2",
			ToolVersion:      "v0.1.0",
			ToolCommit:       "abc123",
			GoVersion:        "go1.23.2",
		},
	})
	if err != nil {
		t.Fatalf("write bundle: %v", err)
	}

	gotConfig, err := os.ReadFile(paths.Config)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotConfig, config) {
		t.Fatalf("config snapshot changed: %q", gotConfig)
	}

	var result map[string]any
	resultBytes, err := os.ReadFile(paths.Result)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		t.Fatal(err)
	}
	if result["status"] != "passed" {
		t.Fatalf("status = %#v", result["status"])
	}

	var metadata BundleMetadata
	metadataBytes, err := os.ReadFile(paths.Metadata)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(config)
	wantHash := "sha256:" + hex.EncodeToString(hash[:])
	if metadata.ConfigSHA256 != wantHash {
		t.Fatalf("config hash = %q, want %q", metadata.ConfigSHA256, wantHash)
	}
	if metadata.SchemaVersion != BundleSchemaVersion {
		t.Fatalf("schema = %d", metadata.SchemaVersion)
	}
	if metadata.ConfigSnapshot != "config.json" || metadata.ResultSnapshot != "result.json" {
		t.Fatalf("snapshots = %q, %q", metadata.ConfigSnapshot, metadata.ResultSnapshot)
	}
	if metadata.CreatedAt.Location() != time.UTC {
		t.Fatalf("created_at location = %v, want UTC", metadata.CreatedAt.Location())
	}
}

func TestWriteBundleRefusesToOverwriteExistingBundle(t *testing.T) {
	root := t.TempDir()
	input := BundleInput{Root: root, ID: "same", Config: []byte("{}\n"), Result: struct{}{}}
	if _, err := WriteBundle(input); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if _, err := WriteBundle(input); err == nil {
		t.Fatal("expected duplicate bundle error")
	}
	if _, err := os.Stat(filepath.Join(root, "same", "metadata.json")); err != nil {
		t.Fatalf("original bundle was damaged: %v", err)
	}
}

func TestWriteBundleFailureDoesNotPublishPartialBundle(t *testing.T) {
	root := t.TempDir()
	_, err := WriteBundle(BundleInput{
		Root:   root,
		ID:     "broken",
		Config: []byte("{}\n"),
		Result: func() {},
	})
	if err == nil {
		t.Fatal("expected result serialization failure")
	}
	if _, statErr := os.Stat(filepath.Join(root, "broken")); !os.IsNotExist(statErr) {
		t.Fatalf("partial bundle was published: %v", statErr)
	}
	tempDirs, globErr := filepath.Glob(filepath.Join(root, ".bundle-tmp-*"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(tempDirs) != 0 {
		t.Fatalf("temporary bundle directories were not cleaned: %#v", tempDirs)
	}
}

func TestWriteBundleRejectsNestedID(t *testing.T) {
	root := t.TempDir()
	if _, err := WriteBundle(BundleInput{
		Root:   root,
		ID:     filepath.Join("nested", "run"),
		Config: []byte("{}\n"),
		Result: struct{}{},
	}); err == nil {
		t.Fatal("expected nested evidence id to be rejected")
	}
}

func TestWriteBundleRejectsAbsoluteID(t *testing.T) {
	root := t.TempDir()
	if _, err := WriteBundle(BundleInput{
		Root:   root,
		ID:     string(filepath.Separator),
		Config: []byte("{}\n"),
		Result: struct{}{},
	}); err == nil {
		t.Fatal("expected absolute evidence id to be rejected")
	}
}
