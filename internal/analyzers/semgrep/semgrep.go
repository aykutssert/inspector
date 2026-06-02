package semgrep

import (
	"encoding/json"
	"os/exec"

	"github.com/aykutssert/inspector/internal/core"
)

type Analyzer struct {
	config string
}

func New(config string) *Analyzer {
	if config == "" {
		config = "auto"
	}
	return &Analyzer{config: config}
}

var _ core.Analyzer = (*Analyzer)(nil)

func (a *Analyzer) Name() string { return "semgrep" }

func (a *Analyzer) InstallHint() string { return "install semgrep (pip install semgrep)" }

func (a *Analyzer) Available() bool {
	_, err := exec.LookPath("semgrep")
	return err == nil
}

type semgrepOut struct {
	Results []struct {
		CheckID string `json:"check_id"`
		Path    string `json:"path"`
		Start   struct {
			Line int `json:"line"`
		} `json:"start"`
		Extra struct {
			Message  string `json:"message"`
			Severity string `json:"severity"`
			Fix      string `json:"fix"`
			Metadata struct {
				Category string          `json:"category"`
				Cwe      json.RawMessage `json:"cwe"`
				Owasp    json.RawMessage `json:"owasp"`
			} `json:"metadata"`
		} `json:"extra"`
	} `json:"results"`
}

// present reports whether a JSON metadata field carries a real value (semgrep
// emits cwe/owasp as a string or array, or omits them entirely).
func present(raw json.RawMessage) bool {
	return len(raw) > 0 && string(raw) != "null" && string(raw) != `""` && string(raw) != "[]"
}

func (a *Analyzer) Scan(ctx core.ProjectContext) ([]core.Finding, error) {
	// diff mode with no changed files: nothing to scan (never fall back to root)
	if ctx.DiffOnly && len(ctx.Files) == 0 {
		return nil, nil
	}
	args := []string{"--json", "--quiet", "--metrics=off", "--config", a.config}
	if len(ctx.Files) > 0 {
		args = append(args, ctx.Files...)
	} else {
		args = append(args, ctx.Root)
	}
	cmd := exec.Command("semgrep", args...)
	cmd.Dir = ctx.Root
	out, err := cmd.Output()
	// Without --error, semgrep exits 0 even when findings exist; a non-zero exit
	// is a real failure (config error, crash, partial run). Surface it.
	if err != nil {
		return nil, err
	}
	var parsed semgrepOut
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, err
	}
	var findings []core.Finding
	for _, r := range parsed.Results {
		sev := mapSeverity(r.Extra.Severity)
		cat := classify(r.Extra.Metadata.Category, present(r.Extra.Metadata.Cwe) || present(r.Extra.Metadata.Owasp))
		findings = append(findings, core.Finding{
			Analyzer: a.Name(),
			RuleID:   r.CheckID,
			Severity: sev,
			Level:    sev.String(),
			Category: cat,
			File:     r.Path,
			Line:     r.Start.Line,
			Message:  r.Extra.Message,
			Fix:      r.Extra.Fix,
		})
	}
	return findings, nil
}

// classify maps semgrep rule metadata to our finding category. A CWE/OWASP tag
// means security regardless of the declared category.
func classify(metaCategory string, hasSecurityTag bool) string {
	if hasSecurityTag {
		return "security"
	}
	switch metaCategory {
	case "security":
		return "security"
	case "performance":
		return "performance"
	case "correctness":
		return "bug"
	case "best-practice", "maintainability", "portability", "compatibility":
		return "quality"
	}
	return ""
}

func mapSeverity(s string) core.Severity {
	switch s {
	case "ERROR":
		return core.SeverityError
	case "WARNING":
		return core.SeverityWarning
	default:
		return core.SeverityInfo
	}
}
