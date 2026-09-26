package webui

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Zennay/ulab/internal/engine"
	"github.com/Zennay/ulab/internal/evidence"
	"github.com/Zennay/ulab/internal/matrix"
)

type Bundle struct {
	Dir      string
	Metadata evidence.BundleMetadata
	Result   matrix.Result
}

type Server struct {
	EvidenceRoot string
}

func (s Server) Handler() http.Handler {
	return http.HandlerFunc(s.serveIndex)
}

func (s Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	bundles, err := LoadBundles(s.EvidenceRoot)
	if err != nil {
		http.Error(w, "read evidence: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := pageData{
		EvidenceRoot: s.EvidenceRoot,
		Bundles:      bundles,
	}
	if len(bundles) > 0 {
		selected := 0
		if requested := r.URL.Query().Get("run"); requested != "" {
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
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := pageTemplate.Execute(w, data); err != nil {
		http.Error(w, "render evidence: "+err.Error(), http.StatusInternalServerError)
	}
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
		if err := readJSON(filepath.Join(dir, "metadata.json"), &metadata); err != nil {
			return nil, fmt.Errorf("%s metadata: %w", entry.Name(), err)
		}
		if metadata.SchemaVersion != evidence.BundleSchemaVersion {
			return nil, fmt.Errorf("%s metadata: unsupported schema version %d", entry.Name(), metadata.SchemaVersion)
		}
		if metadata.InvocationID != entry.Name() {
			return nil, fmt.Errorf("%s metadata: invocation id is %q", entry.Name(), metadata.InvocationID)
		}

		var result matrix.Result
		if err := readJSON(filepath.Join(dir, "result.json"), &result); err != nil {
			return nil, fmt.Errorf("%s result: %w", entry.Name(), err)
		}

		bundles = append(bundles, Bundle{
			Dir:      dir,
			Metadata: metadata,
			Result:   result,
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

func readJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, target); err != nil {
		return err
	}
	return nil
}

type pageData struct {
	EvidenceRoot string
	Bundles      []Bundle
	Selected     *Bundle
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

func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "—"
	}
	if d < time.Second {
		return d.Round(time.Millisecond).String()
	}
	return d.Round(100 * time.Millisecond).String()
}

func reproduceCommand(args []string) string {
	return strings.Join(args, " ")
}

var pageTemplate = template.Must(template.New("index").Funcs(template.FuncMap{
	"duration":  formatDuration,
	"failed":    failedPhase,
	"reproduce": reproduceCommand,
}).Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>uLab compatibility evidence</title>
<style>
:root { font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; color: #16181d; background: #f5f6f8; }
body { margin: 0; }
main { max-width: 1100px; margin: 0 auto; padding: 32px 20px 56px; }
h1 { margin: 0 0 6px; font-size: 28px; }
p { line-height: 1.5; }
.muted { color: #646b78; }
.toolbar, .card { background: white; border: 1px solid #dfe3e8; border-radius: 12px; }
.toolbar { display: flex; gap: 12px; align-items: center; justify-content: space-between; padding: 14px 16px; margin: 24px 0 16px; }
select { max-width: 100%; padding: 8px 10px; border: 1px solid #cbd1d8; border-radius: 8px; background: white; }
.card { padding: 20px; margin-top: 16px; }
.summary { display: flex; gap: 14px; flex-wrap: wrap; align-items: baseline; }
.status { display: inline-flex; align-items: center; padding: 4px 9px; border-radius: 999px; font-size: 13px; font-weight: 650; text-transform: uppercase; letter-spacing: .03em; }
.status-passed { background: #e8f7ed; color: #166534; }
.status-failed { background: #fdecec; color: #991b1b; }
.status-canceled { background: #fff3d6; color: #92400e; }
.meta { display: grid; grid-template-columns: repeat(auto-fit,minmax(210px,1fr)); gap: 12px; margin-top: 16px; }
.meta div { min-width: 0; }
.meta strong { display: block; font-size: 12px; color: #646b78; margin-bottom: 4px; text-transform: uppercase; letter-spacing: .04em; }
code { overflow-wrap: anywhere; }
table { width: 100%; border-collapse: collapse; margin-top: 12px; }
th, td { text-align: left; padding: 11px 10px; border-bottom: 1px solid #e7e9ed; vertical-align: top; }
th { color: #646b78; font-size: 12px; text-transform: uppercase; letter-spacing: .04em; }
.phase-list { margin: 0; padding-left: 18px; }
.phase-list li { margin: 4px 0; }
.error { color: #991b1b; white-space: pre-wrap; }
.empty { padding: 28px; text-align: center; }
@media (max-width: 700px) { table { display: block; overflow-x: auto; } .toolbar { align-items: stretch; flex-direction: column; } }
</style>
</head>
<body>
<main>
	<h1>Compatibility evidence</h1>
	<p class="muted">Read-only view of persisted uLab evidence. Evidence root: <code>{{.EvidenceRoot}}</code></p>

	{{if .Selected}}
	<div class="toolbar">
		<div><strong>Run</strong> <code>{{.Selected.Metadata.InvocationID}}</code></div>
		<form method="get">
			<label for="run" class="muted">Evidence bundle</label>
			<select id="run" name="run" onchange="this.form.submit()">
			{{range .Bundles}}
				<option value="{{.Metadata.InvocationID}}" {{if eq .Metadata.InvocationID $.Selected.Metadata.InvocationID}}selected{{end}}>{{.Metadata.CreatedAt.Format "2006-01-02 15:04:05 UTC"}} · {{.Result.TargetVersion}} · {{.Result.Status}}</option>
			{{end}}
			</select>
		</form>
	</div>

	<section class="card">
		<div class="summary">
			<span class="status status-{{.Selected.Result.Status}}">{{.Selected.Result.Status}}</span>
			<h2>{{len .Selected.Result.Runs}} upgrade path{{if ne (len .Selected.Result.Runs) 1}}s{{end}} → {{.Selected.Result.TargetVersion}}</h2>
		</div>
		<div class="meta">
			<div><strong>Created</strong>{{.Selected.Metadata.CreatedAt.Format "2006-01-02 15:04:05 UTC"}}</div>
			<div><strong>Tool revision</strong><code>{{.Selected.Metadata.ToolVersion}} · {{.Selected.Metadata.ToolCommit}}</code></div>
			<div><strong>Config SHA-256</strong><code>{{.Selected.Metadata.ConfigSHA256}}</code></div>
			<div><strong>Jobs</strong>{{.Selected.Metadata.Jobs}}</div>
		</div>
	</section>

	<section class="card">
		<h2>Compatibility matrix</h2>
		<table>
			<thead><tr><th>From</th><th>Target</th><th>Result</th><th>Failure phase</th><th>Duration</th><th>Phase evidence</th></tr></thead>
			<tbody>
			{{range .Selected.Result.Runs}}
			<tr>
				<td><code>{{.SourceVersion}}</code></td>
				<td><code>{{.TargetVersion}}</code></td>
				<td><span class="status status-{{.Status}}">{{.Status}}</span></td>
				<td>{{with failed .}}{{.}}{{else}}—{{end}}</td>
				<td>{{duration .Duration}}</td>
				<td>
					<details>
						<summary>{{len .Phases}} phases</summary>
						<ol class="phase-list">
						{{range .Phases}}
							<li><strong>{{.Phase}}</strong> — {{.Status}} · {{duration .Duration}}{{with .Error}}<div class="error">{{.}}</div>{{end}}</li>
						{{end}}
						</ol>
					</details>
				</td>
			</tr>
			{{end}}
			</tbody>
		</table>
	</section>

	<section class="card">
		<h2>Reproduce</h2>
		<p class="muted">Command captured with the evidence bundle:</p>
		<code>{{reproduce .Selected.Metadata.ReproduceCommand}}</code>
	</section>
	{{else}}
	<section class="card empty">
		<h2>No evidence bundles yet</h2>
		<p class="muted">Run <code>ulab test</code> first, then reload this page.</p>
	</section>
	{{end}}
</main>
</body>
</html>`))
