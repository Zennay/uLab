package webui

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Zennay/ulab/internal/config"
	"github.com/Zennay/ulab/internal/engine"
	"github.com/Zennay/ulab/internal/evidence"
	"github.com/Zennay/ulab/internal/matrix"
)

type Bundle struct {
	ResultChecksumValid bool
	Dir                 string
	Config              config.Config
	Metadata            evidence.BundleMetadata
	Result              matrix.Result
}

type Server struct {
	EvidenceRoot string
}

func (s Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Evidence pages can contain private upgrade outputs and reproduction
		// commands. Never let browsers or shared caches retain the response,
		// including failures and unknown-run routes.
		w.Header().Set("Cache-Control", "no-store, max-age=0")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		// Local upgrade evidence may contain private project commands and logs.
		// Forbid embedding and Referer leakage on every success/error response.
		w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		s.serveIndex(w, r)
	})
}

func (s Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	bundles, err := LoadBundles(s.EvidenceRoot)
	if err != nil {
		// Preserve detailed errors in LoadBundles for local diagnostics, but never
		// echo evidence metadata or filesystem paths to HTTP clients.
		http.Error(w, "evidence unavailable", http.StatusInternalServerError)
		return
	}

	// Query() silently discards malformed percent escapes. Never substitute
	// the newest READY evidence for an invalid shared run/source deep-link.
	// ParseQuery makes decoding failure explicit before selecting any evidence.
	selectors, queryErr := url.ParseQuery(r.URL.RawQuery)
	if queryErr != nil {
		http.Error(w, "invalid evidence selection", http.StatusNotFound)
		return
	}
	// Duplicate run/source parameters are ambiguous evidence identities.
	// Values.Get silently takes the first and could show READY for the
	// wrong shared link. A valid selection has at most one value per key.
	if len(selectors["run"]) > 1 || len(selectors["source"]) > 1 {
		http.Error(w, "invalid evidence selection", http.StatusNotFound)
		return
	}

	// An explicitly empty selector is not permission to silently open the
	// newest evidence and show a potentially unrelated READY verdict.
	if (selectors.Has("run") && selectors.Get("run") == "") ||
		(selectors.Has("source") && selectors.Get("source") == "") {
		http.Error(w, "invalid evidence selection", http.StatusNotFound)
		return
	}

	// An explicit run or source is a navigation request, not the empty
	// evidence overview. Never report HTTP 200 for a missing selection.
	if len(bundles) == 0 && (selectors.Has("run") || selectors.Has("source")) {
		http.Error(w, "no evidence run available", http.StatusNotFound)
		return
	}

	data := pageData{
		Bundles: bundles,
	}
	if len(bundles) > 0 {
		selected := 0
		if requested := selectors.Get("run"); requested != "" {
			selected = -1
			for i := range bundles {
				if bundles[i].Metadata.InvocationID == requested {
					selected = i
					break
				}
			}
			if selected < 0 {
				http.Error(w, "unknown evidence run", http.StatusNotFound)
				return
			}
		}
		data.Selected = &bundles[selected]
		data.Readiness = releaseReadiness(*data.Selected)

		selectedRun, err := selectRun(data.Selected.Result.Runs, selectors.Get("source"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		data.SelectedRun = selectedRun
	}

	writeEvidencePage(w, pageTemplate, data)
}

// Never stream a partial HTML page or template evaluation errors to the browser.
// Error strings and partially rendered content may contain private evidence
// metadata, hook output or local paths. Buffer until the render fully succeeds.
func writeEvidencePage(w http.ResponseWriter, view *template.Template, data pageData) {
	var page bytes.Buffer
	if err := view.Execute(&page, data); err != nil {
		http.Error(w, "evidence unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(page.Bytes())
}

func LoadBundles(root string) ([]Bundle, error) {
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	bundles := make([]Bundle, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		dir := filepath.Join(root, entry.Name())
		var metadata evidence.BundleMetadata
		if err := readJSON(filepath.Join(dir, "metadata.json"), &metadata, maxEvidenceMetadataBytes); err != nil {
			return nil, fmt.Errorf("%s metadata: %w", entry.Name(), err)
		}
		if metadata.SchemaVersion != evidence.BundleSchemaVersion {
			return nil, fmt.Errorf("%s metadata: unsupported schema version %d", entry.Name(), metadata.SchemaVersion)
		}
		if metadata.InvocationID != entry.Name() {
			return nil, fmt.Errorf("%s metadata: invocation id is %q", entry.Name(), metadata.InvocationID)
		}
		if metadata.ConfigSnapshot != "config.json" || metadata.ResultSnapshot != "result.json" {
			return nil, fmt.Errorf("%s metadata: noncanonical evidence snapshot names", entry.Name())
		}

		// Bind the exact config bytes used for this display to the original
		// evidence receipt. A valid but modified policy cannot reinterpret
		// an older recorded compatibility result as a new release decision.
		configBytes, err := readEvidenceSnapshot(filepath.Join(dir, "config.json"), maxEvidenceConfigBytes)
		if err != nil {
			return nil, fmt.Errorf("%s config: %w", entry.Name(), err)
		}
		hash := sha256.Sum256(configBytes)
		if metadata.ConfigSHA256 != "sha256:"+hex.EncodeToString(hash[:]) {
			return nil, fmt.Errorf("%s config: evidence snapshot hash mismatch", entry.Name())
		}
		var cfg config.Config
		if err := json.Unmarshal(configBytes, &cfg); err != nil {
			return nil, fmt.Errorf("%s config: %w", entry.Name(), err)
		}
		if err := cfg.Validate(); err != nil {
			return nil, fmt.Errorf("%s config: %w", entry.Name(), err)
		}

		resultBytes, err := readEvidenceSnapshot(filepath.Join(dir, "result.json"), maxEvidenceResultBytes)
		if err != nil {
			return nil, fmt.Errorf("%s result: %w", entry.Name(), err)
		}
		validResultHash := false
		if metadata.ResultSHA256 != "" {
			hash := sha256.Sum256(resultBytes)
			if metadata.ResultSHA256 != "sha256:"+hex.EncodeToString(hash[:]) {
				return nil, fmt.Errorf("%s result: evidence snapshot hash mismatch", entry.Name())
			}
			validResultHash = true
		}
		var result matrix.Result
		if err := json.Unmarshal(resultBytes, &result); err != nil {
			return nil, fmt.Errorf("%s result: %w", entry.Name(), err)
		}

		bundles = append(bundles, Bundle{
			ResultChecksumValid: validResultHash,
			Dir:                 dir,
			Config:              cfg,
			Metadata:            metadata,
			Result:              result,
		})
	}

	sort.Slice(bundles, func(i, j int) bool {
		if bundles[i].Metadata.CreatedAt.Equal(bundles[j].Metadata.CreatedAt) {
			return bundles[i].Metadata.InvocationID > bundles[j].Metadata.InvocationID
		}
		return bundles[i].Metadata.CreatedAt.After(bundles[j].Metadata.CreatedAt)
	})
	return bundles, nil
}

func readJSON(path string, target any, limit int64) error {
	data, err := readEvidenceSnapshot(path, limit)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, target); err != nil {
		return err
	}
	return nil
}

type readiness struct {
	Label  string
	Class  string
	Detail string
	Passed int
	Total  int
}

func releaseReadiness(bundle Bundle) readiness {
	total := len(bundle.Result.Runs)
	passed := 0
	for _, run := range bundle.Result.Runs {
		if run.Status == engine.StatusPassed {
			passed++
		}
	}
	if !bundle.ResultChecksumValid {
		return readiness{
			Label: "UNVERIFIED", Class: "unknown",
			Detail: "This historical result lacks a stored result checksum; rerun the upgrade before assessing release readiness.",
			Passed: passed, Total: total,
		}
	}

	if !hasExactPathCoverage(bundle) {
		return readiness{
			Label: "UNVERIFIED", Class: "unknown",
			Detail: "Stored run paths do not exactly match the configured source and target versions; do not use this evidence for release readiness.",
			Passed: passed, Total: total,
		}
	}

	if !hasCoherentPassingPhases(bundle) {
		return readiness{
			Label: "UNVERIFIED", Class: "unknown",
			Detail: "A passing run contradicts its recorded phase status or failure marker; review the original upgrade evidence.",
			Passed: passed, Total: total,
		}
	}

	switch {
	case total == 0:
		return readiness{
			Label:  "NO DATA",
			Class:  "unknown",
			Detail: "No upgrade paths were recorded in this evidence bundle.",
			Passed: passed,
			Total:  total,
		}
	case passed == total && bundle.Result.Status == engine.StatusPassed:
		return readiness{
			Label:  "READY",
			Class:  "passed",
			Detail: fmt.Sprintf("%d of %d tested upgrade paths passed.", passed, total),
			Passed: passed,
			Total:  total,
		}
	case bundle.Config.Policy.RequireAllPaths:
		return readiness{
			Label:  "BLOCKED",
			Class:  "failed",
			Detail: fmt.Sprintf("%d of %d tested upgrade paths passed; require_all_paths blocks the release gate.", passed, total),
			Passed: passed,
			Total:  total,
		}
	default:
		return readiness{
			Label:  "ATTENTION",
			Class:  "attention",
			Detail: fmt.Sprintf("%d of %d tested upgrade paths passed; policy does not require every path, so review non-passing evidence before release.", passed, total),
			Passed: passed,
			Total:  total,
		}
	}
}

func selectRun(runs []engine.RunResult, source string) (*engine.RunResult, error) {
	if len(runs) == 0 {
		return nil, nil
	}
	if source != "" {
		for i := range runs {
			if runs[i].SourceVersion == source {
				return &runs[i], nil
			}
		}
		return nil, fmt.Errorf("unknown source version %q", source)
	}
	for i := range runs {
		if runs[i].Status != engine.StatusPassed {
			return &runs[i], nil
		}
	}
	return &runs[0], nil
}

type pageData struct {
	Bundles     []Bundle
	Selected    *Bundle
	SelectedRun *engine.RunResult
	Readiness   readiness
}

func failedPhase(run engine.RunResult) string {
	for _, phase := range run.Phases {
		if phase.Status == engine.StatusFailed || phase.Status == engine.StatusCanceled {
			return string(phase.Phase)
		}
	}
	if run.Status == engine.StatusCanceled {
		return "canceled"
	}
	return ""
}

func failureKind(run engine.RunResult) string {
	switch run.FailureKind {
	case engine.FailureRunner:
		return "runner / infrastructure"
	case engine.FailureHook:
		return "project hook"
	case engine.FailureCanceled:
		return "canceled"
	default:
		return "—"
	}
}

func failureNote(run engine.RunResult) string {
	switch run.FailureKind {
	case engine.FailureRunner:
		return "The runner failed before uLab could establish a passing compatibility result."
	case engine.FailureHook:
		return "A project-owned hook failed. uLab records the failing phase but does not infer whether the root cause is application behavior or the test definition."
	case engine.FailureCanceled:
		return "The run was canceled; uLab does not present cancellation as an application incompatibility."
	default:
		return ""
	}
}

func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "—"
	}
	if d < time.Second {
		return d.Round(time.Millisecond).String()
	}
	return d.Round(100 * time.Millisecond).String()
}

// Display a copyable POSIX shell command without reinterpreting untrusted
// evidence arguments as shell operators or splitting paths containing spaces.
func reproduceCommand(args []string) string {
	var command strings.Builder
	for i, arg := range args {
		if strings.ContainsRune(arg, 0) {
			return "unavailable: command argument contains NUL"
		}
		if i > 0 {
			command.WriteByte(' ')
		}
		safe := len(arg) > 0
		for _, char := range arg {
			if (char >= 'a' && char <= 'z') ||
				(char >= 'A' && char <= 'Z') ||
				(char >= '0' && char <= '9') ||
				strings.ContainsRune("_@%+=:,./-", char) {
				continue
			}
			safe = false
			break
		}
		if safe {
			command.WriteString(arg)
		} else {
			command.WriteByte(39)
			command.WriteString(strings.ReplaceAll(arg, "'", "'\"'\"'"))
			command.WriteByte(39)
		}
	}
	return command.String()
}

func statusGlyph(status engine.Status) string {
	switch status {
	case engine.StatusPassed:
		return "✓"
	case engine.StatusFailed:
		return "✕"
	case engine.StatusCanceled:
		return "∥"
	default:
		return "?"
	}
}

func nonPassing(status engine.Status) bool {
	return status != engine.StatusPassed
}

var pageTemplate = template.Must(template.New("index").Funcs(template.FuncMap{
	"duration":    formatDuration,
	"failed":      failedPhase,
	"failure":     failureKind,
	"failureNote": failureNote,
	"glyph":       statusGlyph,
	"nonpassing":  nonPassing,
	"reproduce":   reproduceCommand,
}).Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>uLab compatibility evidence</title>
<style>
:root { font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; color: #16181d; background: #f5f6f8; }
body { margin: 0; }
main { max-width: 1120px; margin: 0 auto; padding: 32px 20px 56px; }
h1 { margin: 0 0 6px; font-size: clamp(26px, 4vw, 34px); letter-spacing: -.035em; }
h2 { margin: 0 0 14px; font-size: 20px; }
h3 { margin: 0; font-size: 15px; }
p { line-height: 1.5; }
a { color: inherit; }
.muted { color: #646b78; }
.toolbar, .card { background: white; border: 1px solid #dfe3e8; border-radius: 12px; }
.toolbar { display: flex; gap: 12px; align-items: center; justify-content: space-between; padding: 14px 16px; margin: 24px 0 16px; }
select { max-width: 100%; padding: 8px 10px; border: 1px solid #cbd1d8; border-radius: 8px; background: white; }
.toolbar form { display: flex; gap: 9px; align-items: center; flex-wrap: wrap; }
.toolbar button { min-height: 40px; padding: 8px 14px; border: 1px solid #344563; border-radius: 8px; background: #344563; color: white; cursor: pointer; font: inherit; font-weight: 650; }
.toolbar button:hover { background: #233451; }
.card { padding: 20px; margin-top: 16px; }
.readiness { display: flex; gap: 18px; align-items: center; flex-wrap: wrap; }
.readiness-copy { flex: 1 1 420px; }
.status { display: inline-flex; align-items: center; gap: 5px; padding: 4px 9px; border-radius: 999px; font-size: 13px; font-weight: 700; text-transform: uppercase; letter-spacing: .03em; }
.status-passed { background: #e8f7ed; color: #166534; }
.status-failed { background: #fdecec; color: #991b1b; }
.status-canceled, .status-attention { background: #fff3d6; color: #92400e; }
.status-unknown { background: #eef0f3; color: #4b5563; }
.readiness-status { font-size: 15px; padding: 7px 12px; }
.meta { display: grid; grid-template-columns: repeat(auto-fit,minmax(210px,1fr)); gap: 12px; margin-top: 18px; }
.meta div { min-width: 0; }
.meta strong { display: block; font-size: 12px; color: #646b78; margin-bottom: 4px; text-transform: uppercase; letter-spacing: .04em; }
code { overflow-wrap: anywhere; }
table { width: 100%; border-collapse: collapse; margin-top: 12px; }
th, td { text-align: left; padding: 11px 10px; border-bottom: 1px solid #e7e9ed; vertical-align: top; }
th { color: #646b78; font-size: 12px; text-transform: uppercase; letter-spacing: .04em; }
.inspect { font-weight: 650; white-space: nowrap; }
.timeline { display: grid; grid-template-columns: repeat(auto-fit,minmax(140px,1fr)); gap: 10px; margin: 14px 0 18px; padding: 0; list-style: none; }
.phase { border: 1px solid #dfe3e8; border-radius: 10px; padding: 12px; min-width: 0; }
.phase-passed { border-color: #bddfc7; }
.phase-failed { border-color: #efb7b7; }
.phase-canceled { border-color: #edd79e; }
.phase .status { margin-top: 9px; }
.evidence details { border-top: 1px solid #e7e9ed; padding: 12px 0; }
.evidence summary { cursor: pointer; font-weight: 650; }
pre { white-space: pre-wrap; overflow-wrap: anywhere; background: #f7f8fa; border: 1px solid #e4e7eb; border-radius: 8px; padding: 10px; max-height: 320px; overflow: auto; }
.error { color: #991b1b; }
.notice { border-left: 3px solid #cbd1d8; padding-left: 12px; }
.empty { padding: 28px; text-align: center; }
.task-nav { display: flex; gap: 8px; flex-wrap: wrap; margin: 24px 0 6px; }
.task-nav a, .task-link { display: inline-flex; align-items: center; gap: 6px; border: 1px solid #cbd1d8; border-radius: 9px; padding: 9px 12px; background: #fff; font-weight: 650; text-decoration: none; }
.task-nav a:hover, .task-link:hover { border-color: #465a77; background: #f1f5f9; }
a:focus-visible, select:focus-visible, button:focus-visible, summary:focus-visible { outline: 3px solid #2563eb; outline-offset: 3px; }
.verdict-actions { display: flex; align-items: center; flex-wrap: wrap; gap: 12px; margin-top: 12px; }
.scope-note { margin-top: 16px; padding: 12px 14px; background: #f4f7fb; border: 1px solid #dbe5f0; border-radius: 9px; }
.scope-note strong { display: block; margin-bottom: 4px; }
.scope-note p { margin: 0; }
.attention-row { background: #fffaf0; }
.attention-row td:first-child { border-left: 3px solid #d97706; }
.task-link { width: fit-content; }
@media (max-width: 700px) { main { padding: 22px 14px 38px; } .card { padding: 16px; } .task-nav a { flex: 1 1 135px; } }
@media (max-width: 700px) { table { display: block; overflow-x: auto; } .toolbar { align-items: stretch; flex-direction: column; } }
</style>
</head>
<body>
<main>
	<h1>Upgrade check results</h1>
	<p class="muted">Understand the recorded release verdict, find an upgrade path that needs attention, and inspect the evidence. Read-only view of local evidence; filesystem paths stay private.</p>

	{{if .Selected}}
	<nav class="task-nav" aria-label="Explore upgrade check results">
		<a href="#verdict">1. Release verdict</a>
		<a href="#paths">2. Upgrade paths</a>
		<a href="#diagnosis">3. Failure details</a>
		<a href="#reproduce">4. Reproduce</a>
	</nav>
	{{end}}

	{{if .Selected}}
	<div class="toolbar">
		<div><strong>Recorded check</strong> <code>{{.Selected.Metadata.InvocationID}}</code></div>
		<form method="get">
			<label for="run" class="muted">Evidence bundle</label>
			<select id="run" name="run">
			{{range .Bundles}}
				<option value="{{.Metadata.InvocationID}}" {{if eq .Metadata.InvocationID $.Selected.Metadata.InvocationID}}selected{{end}}>{{.Metadata.CreatedAt.Format "2006-01-02 15:04:05 UTC"}} · {{.Result.TargetVersion}} · {{.Result.Status}}</option>
			{{end}}
			</select>
			<button type="submit">View check</button>
		</form>
	</div>

	<section class="card" id="verdict" aria-labelledby="verdict-title">
		<div class="readiness">
			<span class="status status-{{.Readiness.Class}} readiness-status">{{.Readiness.Label}}</span>
			<div class="readiness-copy">
				<h2 id="verdict-title">Release readiness · {{.Selected.Result.TargetVersion}}</h2>
				{{if eq .Readiness.Class "passed"}}
				<p>All recorded upgrade paths passed this check. A production release still needs its own approval.</p>
				{{else if eq .Readiness.Class "failed"}}
				<p>This check did not pass the configured release gate. Investigate the failing upgrade path before deciding what to release.</p>
				{{else if eq .Readiness.Class "attention"}}
				<p>One or more paths did not pass. The configured policy allows exceptions, but those failures still need review.</p>
				{{else}}
				<p>The recorded evidence cannot support a passing decision yet. Check the evidence and rerun if needed.</p>
				{{end}}
				<details><summary>How was this verdict calculated?</summary><p class="muted">{{.Readiness.Detail}}</p></details>
				<div class="verdict-actions">
					<a class="task-link" href="#paths">{{if lt .Readiness.Passed .Readiness.Total}}Find the path that needs attention →{{else}}Review tested upgrade paths →{{end}}</a>
				</div>
			</div>
		</div>
		<div class="meta">
			<div><strong>Coverage</strong>{{.Readiness.Passed}} / {{.Readiness.Total}} tested paths passed</div>
			<div><strong>Gate policy</strong>{{if .Selected.Config.Policy.RequireAllPaths}}all paths required{{else}}all paths not required{{end}}</div>
			<div><strong>Created</strong>{{.Selected.Metadata.CreatedAt.Format "2006-01-02 15:04:05 UTC"}}</div>
			<div><strong>Tool revision</strong><code>{{.Selected.Metadata.ToolVersion}} · {{.Selected.Metadata.ToolCommit}}</code></div>
		</div>
		<div class="scope-note">
			<strong>What this result does—and does not—prove</strong>
			<p>Only the recorded source-to-target paths and project-defined checks were tested. A passing result is not approval to deploy, and does not cover untested versions or application behavior outside those checks.</p>
		</div>
	</section>

	<section class="card" id="paths" aria-labelledby="paths-title">
		<h2 id="paths-title">Upgrade paths · compatibility matrix</h2>
		<p class="muted">Start with a highlighted path if one did not pass. Each path links to its recorded checks and failure details.</p>
		<table>
			<thead><tr><th scope="col">Existing version</th><th scope="col">New version</th><th scope="col">Result</th><th scope="col">Recorded failure</th><th scope="col">Phase</th><th scope="col">Duration</th><th scope="col">Details</th></tr></thead>
			<tbody>
			{{range .Selected.Result.Runs}}
			<tr {{if nonpassing .Status}}class="attention-row"{{end}}>
				<td><code>{{.SourceVersion}}</code></td>
				<td><code>{{.TargetVersion}}</code></td>
				<td><span class="status status-{{.Status}}">{{glyph .Status}} {{.Status}}</span></td>
				<td>{{failure .}}</td>
				<td>{{with failed .}}{{.}}{{else}}—{{end}}</td>
				<td>{{duration .Duration}}</td>
				<td><a class="inspect" href="?run={{urlquery $.Selected.Metadata.InvocationID}}&source={{urlquery .SourceVersion}}#diagnosis">{{if nonpassing .Status}}Investigate →{{else}}View evidence{{end}}</a></td>
			</tr>
			{{end}}
			</tbody>
		</table>
	</section>

	{{with .SelectedRun}}
	<section class="card" id="diagnosis" aria-labelledby="diagnosis-title">
		<h2 id="diagnosis-title">Run timeline · <code>{{.SourceVersion}}</code> → <code>{{.TargetVersion}}</code></h2>
		<p><span class="status status-{{.Status}}">{{glyph .Status}} {{.Status}}</span> &nbsp; Failure type: <strong>{{failure .}}</strong>{{with failed .}} · phase <strong>{{.}}</strong>{{end}}</p>
		{{with failureNote .}}<p class="muted notice">{{.}}</p>{{end}}
		<ol class="timeline">
			{{range .Phases}}
			<li class="phase phase-{{.Status}}">
				<h3>{{.Phase}}</h3>
				<span class="status status-{{.Status}}">{{glyph .Status}} {{.Status}}</span>
				<p class="muted">{{duration .Duration}}</p>
			</li>
			{{end}}
		</ol>

		<div class="evidence" id="evidence-details">
			<h2>Recorded phase evidence</h2>
			{{range .Phases}}
			<details {{if nonpassing .Status}}open{{end}}>
				<summary>{{.Phase}} · {{.Status}} · {{duration .Duration}}</summary>
				{{with .Command}}<p><strong>Command</strong><br><code>{{.}}</code></p>{{end}}
				{{with .Output}}<p><strong>Output</strong></p><pre>{{.}}</pre>{{end}}
				{{with .Error}}<p><strong>Error</strong></p><pre class="error">{{.}}</pre>{{end}}
				{{if and (not .Command) (not .Output) (not .Error)}}<p class="muted">No command output or error text was recorded for this phase.</p>{{end}}
			</details>
			{{end}}
		</div>
	</section>
	{{end}}

	<section class="card" id="reproduce" aria-labelledby="reproduce-title">
		<h2 id="reproduce-title">Reproduce and provenance</h2>
		<p class="muted">The recorded command and checksums are available for audit. Review the command before running it in a trusted environment.</p>
		<div class="meta">
			<div><strong>Config SHA-256</strong><code>{{.Selected.Metadata.ConfigSHA256}}</code></div>
            <div><strong>Result SHA-256</strong><code>{{if .Selected.Metadata.ResultSHA256}}{{.Selected.Metadata.ResultSHA256}}{{else}}not recorded (legacy){{end}}</code></div>
			<div><strong>Jobs</strong>{{.Selected.Metadata.Jobs}}</div>
			<div><strong>Runner</strong>{{if .Selected.Config.Runner.Type}}{{.Selected.Config.Runner.Type}}{{else}}process{{end}}</div>
			<div><strong>Evidence bundle</strong><code>{{.Selected.Metadata.InvocationID}}</code></div>
		</div>
		<p class="muted">Reproduction command captured with this check:</p>
		<code>{{reproduce .Selected.Metadata.ReproduceCommand}}</code>
	</section>
	{{else}}
	<section class="card empty">
		<h2>No upgrade checks recorded yet</h2>
		<p class="muted">Run <code>ulab test --config YOUR_CONFIG.json</code> for a configured upgrade scenario, then reload this page to review its release verdict and evidence.</p>
	</section>
	{{end}}
</main>
</body>
</html>`))
