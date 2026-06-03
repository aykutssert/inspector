// Package knip wraps the knip dead-code scanner to surface project-wide waste
// that single-file linters cannot see: files reachable from no entry point,
// exports nobody imports, and dependencies declared but never used (or used but
// never declared). knip walks the whole module graph, so it runs once per
// project on a full scan and is skipped in diff mode, where a whole-project
// dead-code report would flag code unrelated to the change.
package knip

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/aykutssert/inspector/internal/core"
	"github.com/aykutssert/inspector/internal/execx"
	"github.com/aykutssert/inspector/internal/toolchain"
)

type Analyzer struct{}

func New() *Analyzer { return &Analyzer{} }

var _ core.Analyzer = (*Analyzer)(nil)

func (a *Analyzer) Name() string { return "knip" }

func (a *Analyzer) Available() bool { return true }

func (a *Analyzer) Scan(ctx core.ProjectContext) ([]core.Finding, error) {
	// Whole-project analysis: a dead-code report scoped to changed files is
	// misleading, so knip only runs on a full scan.
	if ctx.DiffOnly {
		return nil, nil
	}
	// knip is anchored on the root package.json (it discovers workspaces from
	// there). No manifest → nothing for knip to analyze.
	if !exists(filepath.Join(ctx.Root, "package.json")) {
		return nil, nil
	}
	// knip resolves the dependency graph through node_modules; without it the
	// report is dominated by false "unresolved"/"unlisted" noise. Skip and say so.
	if !exists(filepath.Join(ctx.Root, "node_modules")) {
		return []core.Finding{skipNotice(a.Name(),
			"package.json found but node_modules is missing; dead-code analysis was skipped to avoid false unused/unresolved results.")}, nil
	}

	bin := resolveKnip(ctx.Root)
	if bin == "" {
		return []core.Finding{skipNotice(a.Name(),
			"package.json found but no knip binary is available (repo or toolchain); dead-code analysis was skipped.")}, nil
	}

	cmd := exec.Command(bin, "--reporter", "json", "--no-progress")
	cmd.Dir = ctx.Root
	out, err := cmd.Output()
	if err != nil {
		// knip exits non-zero when it reports issues; that is expected. A genuine
		// failure (bad config, crash) produces no JSON on stdout.
		exit, ok := err.(*exec.ExitError)
		if !ok {
			return nil, execx.Err(err)
		}
		if len(out) == 0 {
			return []core.Finding{runFailure(a.Name(), string(exit.Stderr))}, nil
		}
	}

	var report knipReport
	if err := json.Unmarshal(out, &report); err != nil {
		return []core.Finding{runFailure(a.Name(), string(out))}, nil
	}
	return report.findings(a.Name()), nil
}

type knipReport struct {
	Files  []string `json:"files"`
	Issues []struct {
		File         string       `json:"file"`
		Dependencies []namedIssue `json:"dependencies"`
		DevDeps      []namedIssue `json:"devDependencies"`
		OptionalPeer []namedIssue `json:"optionalPeerDependencies"`
		Unlisted     []namedIssue `json:"unlisted"`
		Exports      []namedIssue `json:"exports"`
		Types        []namedIssue `json:"types"`
	} `json:"issues"`
}

type namedIssue struct {
	Name string `json:"name"`
	Line int    `json:"line"`
}

func (r knipReport) findings(analyzer string) []core.Finding {
	var out []core.Finding
	for _, f := range r.Files {
		out = append(out, finding(analyzer, "unused-file", core.SeverityWarning, "quality", f, 0,
			"This file is reachable from no entry point, so it appears to be dead code that ships nothing. Confirm it is not loaded dynamically or by a tool config, then delete it.",
			"Remove the file, or add its real entry point to the knip/project config if it is used in a way static analysis cannot see."))
	}
	for _, is := range r.Issues {
		for _, e := range append(append([]namedIssue{}, is.Exports...), is.Types...) {
			out = append(out, finding(analyzer, "unused-export", core.SeverityWarning, "quality", is.File, e.Line,
				"The export '"+e.Name+"' is never imported anywhere in the project. Unused exports widen the public surface and keep dead code alive.",
				"Remove the export (and its definition if nothing else uses it), or make it internal if it is only used within this file."))
		}
		for _, d := range append(append(append([]namedIssue{}, is.Dependencies...), is.DevDeps...), is.OptionalPeer...) {
			out = append(out, finding(analyzer, "unused-dependency", core.SeverityWarning, "quality", is.File, 0,
				"The dependency '"+d.Name+"' is declared in package.json but never imported. Unused dependencies bloat installs and enlarge the security surface (every dep is a potential CVE).",
				"Remove '"+d.Name+"' from package.json, or confirm it is used indirectly (e.g. a build tool) and add it to the knip config."))
		}
		for _, u := range is.Unlisted {
			out = append(out, finding(analyzer, "unlisted-dependency", core.SeverityWarning, "bug", is.File, 0,
				"'"+u.Name+"' is imported but not declared in package.json. It works only because it is hoisted or installed transitively, so installs can break when that changes.",
				"Add '"+u.Name+"' to package.json dependencies (or devDependencies) so the import resolves reliably."))
		}
	}
	return out
}

func finding(analyzer, rule string, sev core.Severity, category, file string, line int, msg, fix string) core.Finding {
	return core.Finding{
		Analyzer:   analyzer,
		RuleID:     rule,
		Severity:   sev,
		Level:      sev.String(),
		Category:   category,
		Confidence: core.ConfidenceHint,
		File:       file,
		Line:       line,
		Message:    msg,
		Fix:        fix,
	}
}

// resolveKnip prefers a project-local knip (matching the project's knip config
// and version), then inspector's managed toolchain.
func resolveKnip(root string) string {
	local := filepath.Join(root, "node_modules", ".bin", "knip")
	if exists(local) {
		return local
	}
	if bin, ok := toolchain.Bin("knip", "knip"); ok {
		return bin
	}
	return ""
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func skipNotice(analyzer, msg string) core.Finding {
	return core.Finding{
		Analyzer:   analyzer,
		RuleID:     "dead-code-skipped",
		Severity:   core.SeverityInfo,
		Level:      core.SeverityInfo.String(),
		Category:   "quality",
		Confidence: core.ConfidenceHint,
		Message:    msg,
		Fix:        "run `npm install` in the project so knip can resolve the module graph",
	}
}

func runFailure(analyzer, output string) core.Finding {
	msg := firstLine(output)
	if msg == "" {
		msg = "knip exited non-zero but produced no JSON output."
	}
	return core.Finding{
		Analyzer:   analyzer,
		RuleID:     "dead-code-failed",
		Severity:   core.SeverityWarning,
		Level:      core.SeverityWarning.String(),
		Category:   "quality",
		Confidence: core.ConfidenceHint,
		Message:    "Dead-code analysis could not complete: " + msg,
		Fix:        "Run `knip` in the project and fix the reported config problem.",
	}
}

func firstLine(text string) string {
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			return text[:i]
		}
	}
	return text
}
