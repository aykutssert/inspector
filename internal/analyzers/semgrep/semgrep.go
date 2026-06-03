package semgrep

import (
	"encoding/json"
	"os/exec"

	"github.com/aykutssert/inspector/internal/core"
)

type Analyzer struct {
	configs []string
}

// defaultConfigs are registry rule packs covering JS/TS/React plus a general
// security audit. We avoid "auto" because it requires metrics to be enabled
// (semgrep: "Cannot create auto config when metrics are off"), and we always
// run with --metrics=off for privacy.
var defaultConfigs = []string{
	"p/default",
	"p/javascript",
	"p/typescript",
	"p/react",
	"p/security-audit",
}

func New(config string) *Analyzer {
	if config == "" {
		return &Analyzer{configs: defaultConfigs}
	}
	return &Analyzer{configs: []string{config}}
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
	Errors []struct {
		Level string `json:"level"`
		// Type is ["KindString", [spans...]]; we read the kind from element 0.
		Type    []json.RawMessage `json:"type"`
		Message string            `json:"message"`
		Path    string            `json:"path"`
		Spans   []struct {
			File  string `json:"file"`
			Start struct {
				Line int `json:"line"`
			} `json:"start"`
		} `json:"spans"`
	} `json:"errors"`
}

// parseErrorKinds are the semgrep error kinds that mean source code did not
// parse — a likely syntax error. Other error kinds (rule errors, timeouts) are
// tool noise, not findings, so we ignore them.
var parseErrorKinds = map[string]bool{
	"PartialParsing": true,
	"SyntaxError":    true,
	"LexicalError":   true,
}

func errorKind(t []json.RawMessage) string {
	if len(t) == 0 {
		return ""
	}
	var kind string
	_ = json.Unmarshal(t[0], &kind)
	return kind
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
	args := []string{"--json", "--quiet", "--metrics=off"}
	for _, c := range a.configs {
		args = append(args, "--config", c)
	}
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
			Analyzer:   a.Name(),
			RuleID:     r.CheckID,
			Severity:   sev,
			Level:      sev.String(),
			Category:   cat,
			Confidence: core.ConfidenceRule,
			File:       r.Path,
			Line:       r.Start.Line,
			Message:    r.Extra.Message,
			Fix:        r.Extra.Fix,
		})
	}
	// A failed parse means semgrep could not read the file — likely a syntax
	// error, but possibly grammar it doesn't support. Surface it as a hint
	// (verify, don't trust) rather than a hard rule, to stay low-noise.
	for _, e := range parsed.Errors {
		if !parseErrorKinds[errorKind(e.Type)] {
			continue
		}
		file, line := e.Path, 0
		if len(e.Spans) > 0 {
			if file == "" {
				file = e.Spans[0].File
			}
			line = e.Spans[0].Start.Line
		}
		findings = append(findings, core.Finding{
			Analyzer:   a.Name(),
			RuleID:     "syntax-or-parse-error",
			Severity:   core.SeverityWarning,
			Level:      core.SeverityWarning.String(),
			Category:   "bug",
			Confidence: core.ConfidenceHint,
			File:       file,
			Line:       line,
			Message:    e.Message,
			Fix:        "Confirm this file parses — semgrep could not fully parse it (likely a syntax error).",
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
