package evidence

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const BundleSchemaVersion = 1

type BundleMetadata struct {
	SchemaVersion    int       `json:"schema_version"`
	InvocationID     string    `json:"invocation_id"`
	CreatedAt        time.Time `json:"created_at"`
	WorkingDirectory string    `json:"working_directory"`
	ConfigPath       string    `json:"config_path"`
	ConfigSHA256     string    `json:"config_sha256"`
	ConfigSnapshot   string    `json:"config_snapshot"`
	ResultSnapshot   string    `json:"result_snapshot"`
	ReproduceCommand []string  `json:"reproduce_command"`
	Jobs             int       `json:"jobs"`
	SourceVersions   []string  `json:"source_versions"`
	TargetVersion    string    `json:"target_version"`
	ToolVersion      string    `json:"tool_version"`
	ToolCommit       string    `json:"tool_commit"`
	GoVersion        string    `json:"go_version"`
}

type BundleInput struct {
	Root       string
	ID         string
	ConfigPath string
	Config     []byte
	Result     any
	Metadata   BundleMetadata
}

type BundlePaths struct {
	Dir      string
	Config   string
	Result   string
	Metadata string
}

func NewInvocationID(now time.Time) string {
	var raw [4]byte
	if _, err := rand.Read(raw[:]); err == nil {
		return now.UTC().Format("20060102T150405.000000000Z") + "-" + hex.EncodeToString(raw[:])
	}
	return fmt.Sprintf("%s-%x", now.UTC().Format("20060102T150405.000000000Z"), now.UnixNano())
}

func WriteBundle(input BundleInput) (paths BundlePaths, err error) {
	if input.ID == "" {
		return BundlePaths{}, errors.New("evidence bundle id is required")
	}
	if filepath.IsAbs(input.ID) || filepath.Base(input.ID) != input.ID || input.ID == "." || input.ID == ".." {
		return BundlePaths{}, fmt.Errorf("invalid evidence bundle id %q", input.ID)
	}

	root := input.Root
	if root == "" {
		root = filepath.Join(".ulab", "runs")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return BundlePaths{}, fmt.Errorf("create evidence root: %w", err)
	}

	dir := filepath.Join(root, input.ID)
	if _, err := os.Stat(dir); err == nil {
		return BundlePaths{}, fmt.Errorf("create evidence bundle: %s already exists", dir)
	} else if !errors.Is(err, os.ErrNotExist) {
		return BundlePaths{}, fmt.Errorf("inspect evidence bundle: %w", err)
	}

	tempDir, err := os.MkdirTemp(root, ".bundle-tmp-")
	if err != nil {
		return BundlePaths{}, fmt.Errorf("create temporary evidence bundle: %w", err)
	}
	if err := os.Chmod(tempDir, 0o755); err != nil {
		_ = os.RemoveAll(tempDir)
		return BundlePaths{}, fmt.Errorf("set evidence bundle permissions: %w", err)
	}

	published := false
	defer func() {
		if !published {
			_ = os.RemoveAll(tempDir)
		}
	}()

	tempPaths := BundlePaths{
		Dir:      tempDir,
		Config:   filepath.Join(tempDir, "config.json"),
		Result:   filepath.Join(tempDir, "result.json"),
		Metadata: filepath.Join(tempDir, "metadata.json"),
	}
	if err := os.WriteFile(tempPaths.Config, input.Config, 0o644); err != nil {
		return BundlePaths{}, fmt.Errorf("write config snapshot: %w", err)
	}
	if err := WriteJSON(tempPaths.Result, input.Result); err != nil {
		return BundlePaths{}, fmt.Errorf("write result snapshot: %w", err)
	}

	metadata := input.Metadata
	metadata.SchemaVersion = BundleSchemaVersion
	metadata.InvocationID = input.ID
	metadata.ConfigPath = input.ConfigPath
	hash := sha256.Sum256(input.Config)
	metadata.ConfigSHA256 = "sha256:" + hex.EncodeToString(hash[:])
	metadata.ConfigSnapshot = "config.json"
	metadata.ResultSnapshot = "result.json"
	if metadata.CreatedAt.IsZero() {
		metadata.CreatedAt = time.Now().UTC()
	} else {
		metadata.CreatedAt = metadata.CreatedAt.UTC()
	}
	if err := WriteJSON(tempPaths.Metadata, metadata); err != nil {
		return BundlePaths{}, fmt.Errorf("write evidence metadata: %w", err)
	}

	if err := os.Rename(tempDir, dir); err != nil {
		return BundlePaths{}, fmt.Errorf("publish evidence bundle: %w", err)
	}
	published = true

	return BundlePaths{
		Dir:      dir,
		Config:   filepath.Join(dir, "config.json"),
		Result:   filepath.Join(dir, "result.json"),
		Metadata: filepath.Join(dir, "metadata.json"),
	}, nil
}
