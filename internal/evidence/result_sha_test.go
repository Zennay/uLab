package evidence

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestWriteBundleBindsExactStoredResultBytes(t *testing.T) {
	paths, err := WriteBundle(BundleInput{
		Root:   t.TempDir(),
		ID:     "result-receipt",
		Config: []byte("{}\n"),
		Result: map[string]any{
			"status": "failed",
			"runs":   []any{map[string]any{"source": "v1", "status": "failed"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	resultBytes, err := os.ReadFile(paths.Result)
	if err != nil {
		t.Fatal(err)
	}
	metadataBytes, err := os.ReadFile(paths.Metadata)
	if err != nil {
		t.Fatal(err)
	}
	var receipt BundleMetadata
	if err := json.Unmarshal(metadataBytes, &receipt); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(resultBytes)
	want := "sha256:" + hex.EncodeToString(digest[:])
	if receipt.ResultSHA256 != want {
		t.Fatalf("result receipt %q does not bind persisted bytes (%q)", receipt.ResultSHA256, want)
	}
	if receipt.ConfigSHA256 == "" {
		t.Fatal("result digest must not replace existing config digest")
	}
}
