package webui

// hasExactPathCoverage checks the semantic identity of persisted upgrade
// evidence, not just integrity of its individually hashed JSON files.
// Unknown/mismatched paths may remain inspectable but never authorize READY.
func hasExactPathCoverage(bundle Bundle) bool {
	target := bundle.Config.Versions.To
	expected := bundle.Config.Versions.From
	if target == "" || len(expected) == 0 ||
		bundle.Metadata.TargetVersion != target ||
		bundle.Result.TargetVersion != target ||
		len(bundle.Metadata.SourceVersions) != len(expected) ||
		len(bundle.Result.Runs) != len(expected) {
		return false
	}

	expectedSet := make(map[string]bool, len(expected))
	for _, from := range expected {
		if from == "" || expectedSet[from] {
			return false
		}
		expectedSet[from] = true
	}

	metadataSeen := make(map[string]bool, len(expected))
	for _, from := range bundle.Metadata.SourceVersions {
		if !expectedSet[from] || metadataSeen[from] {
			return false
		}
		metadataSeen[from] = true
	}

	seen := make(map[string]bool, len(expected))
	for _, run := range bundle.Result.Runs {
		if run.TargetVersion != target || !expectedSet[run.SourceVersion] || seen[run.SourceVersion] {
			return false
		}
		seen[run.SourceVersion] = true
	}
	return len(seen) == len(expectedSet)
}
