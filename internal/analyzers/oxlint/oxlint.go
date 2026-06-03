package oxlint

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"

	"github.com/aykutssert/inspector/internal/core"
)

type Analyzer struct{}

func New() *Analyzer { return &Analyzer{} }

var _ core.Analyzer = (*Analyzer)(nil)

func (a *Analyzer) Name() string { return "oxlint" }

func (a *Analyzer) InstallHint() string { return "install oxlint (npm i -g oxlint)" }

func (a *Analyzer) Available() bool {
	_, err := exec.LookPath("oxlint")
	return err == nil
}

// curatedConfig enables the React/correctness rule sets that catch real bugs
// and codegen smells, while leaving out the stylistic/pedantic categories
// (magic numbers, comment casing, filename case) that would drown the signal.
// Written to a temp file per scan so the user's repo is never touched.
const curatedConfig = `{
  "plugins": ["react", "react-perf", "jsx-a11y"],
  "categories": { "correctness": "warn", "suspicious": "warn", "perf": "warn" },
  "rules": {
    "react/react-in-jsx-scope": "off"
  }
}`

type oxlintOut struct {
	Diagnostics []struct {
		Message  string `json:"message"`
		Code     string `json:"code"`
		Severity string `json:"severity"`
		Help     string `json:"help"`
		Filename string `json:"filename"`
		Labels   []struct {
			Span struct {
				Line int `json:"line"`
			} `json:"span"`
		} `json:"labels"`
	} `json:"diagnostics"`
}

func (a *Analyzer) Scan(ctx core.ProjectContext) ([]core.Finding, error) {
	if len(ctx.Files) == 0 {
		return nil, nil // diff mode with no JS files, or empty project
	}

	cfg, err := os.CreateTemp("", "inspector-oxlint-*.json")
	if err != nil {
		return nil, err
	}
	defer os.Remove(cfg.Name())
	if _, err := cfg.WriteString(curatedConfig); err != nil {
		cfg.Close()
		return nil, err
	}
	cfg.Close()

	args := []string{"-c", cfg.Name(), "--format", "json"}
	args = append(args, ctx.Files...)
	cmd := exec.Command("oxlint", args...)
	cmd.Dir = ctx.Root
	out, err := cmd.Output()
	// oxlint exits non-zero when it reports diagnostics; that is not a failure.
	// A real failure produces no JSON on stdout.
	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return nil, err
		}
		if len(out) == 0 {
			return nil, err
		}
	}

	var parsed oxlintOut
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, err
	}

	var findings []core.Finding
	for _, d := range parsed.Diagnostics {
		plugin, rule := splitCode(d.Code)
		sev := mapSeverity(d.Severity)
		line := 0
		if len(d.Labels) > 0 {
			line = d.Labels[0].Span.Line
		}
		findings = append(findings, core.Finding{
			Analyzer:   a.Name(),
			RuleID:     rule,
			Severity:   sev,
			Level:      sev.String(),
			Category:   classify(plugin),
			Confidence: core.ConfidenceRule,
			File:       d.Filename,
			Line:       line,
			Message:    d.Message,
			Fix:        d.Help,
		})
	}
	return findings, nil
}

// splitCode turns oxlint's "plugin(rule-name)" into its parts. A bare code with
// no parens returns ("", code).
func splitCode(code string) (plugin, rule string) {
	open := strings.IndexByte(code, '(')
	if open < 0 || !strings.HasSuffix(code, ")") {
		return "", code
	}
	return code[:open], code[open+1 : len(code)-1]
}

func classify(plugin string) string {
	switch plugin {
	case "react-perf":
		return "performance"
	case "jsx-a11y":
		return "quality"
	case "react", "typescript", "oxc":
		return "bug"
	default:
		return "quality"
	}
}

func mapSeverity(s string) core.Severity {
	switch s {
	case "error":
		return core.SeverityError
	case "warning":
		return core.SeverityWarning
	default:
		return core.SeverityInfo
	}
}
