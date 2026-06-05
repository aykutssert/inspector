package tailwindlint

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aykutssert/scout/internal/core"
	"github.com/aykutssert/scout/internal/execx"
	"github.com/aykutssert/scout/internal/toolchain"
)

// Analyzer wraps eslint + eslint-plugin-tailwindcss, the proven Tailwind linter,
// run from a managed toolchain that scout ships under _toolchains/tailwind.
// We do not author Tailwind rules ourselves; we wrap the ecosystem's linter,
// which is config/version-aware in a way a hand-rolled regex can never be.
type Analyzer struct{}

func New() *Analyzer { return &Analyzer{} }

var _ core.Analyzer = (*Analyzer)(nil)

func (a *Analyzer) Name() string { return "tailwind-lint" }

// Available is always true: this analyzer is conditional, not mandatory. Whether
// it has work (Tailwind in use) and whether its toolchain is installed are
// decided in Scan, so a non-Tailwind repo is never flagged as a missing-scanner
// error by the fail-closed orchestrator.
func (a *Analyzer) Available() bool { return true }

// lintExts are the JS/TS source files whose className/class attributes the
// plugin inspects.
var lintExts = map[string]bool{
	".js": true, ".jsx": true, ".mjs": true, ".cjs": true,
	".ts": true, ".tsx": true, ".mts": true, ".cts": true,
}

func (a *Analyzer) Scan(ctx core.ProjectContext) ([]core.Finding, error) {
	var targets []string
	for _, f := range ctx.Files {
		if lintExts[strings.ToLower(filepath.Ext(f))] {
			targets = append(targets, f)
		}
	}
	if len(targets) == 0 {
		return nil, nil // no JS/TS files in this scan
	}

	// eslint-plugin-tailwindcss resolves tailwindcss from the scan root's
	// node_modules and crashes ("Could not find tailwindcss") when it is absent.
	// So only run for projects that actually have Tailwind installed; a plain JS
	// repo without Tailwind is silently out of scope (no noise).
	if !hasTailwindInstalled(ctx.Root) {
		return nil, nil
	}

	dir, ok := toolchain.Dir("tailwind")
	if !ok {
		// Tailwind is in use but our toolchain is not installed: surface partial
		// coverage honestly instead of silently passing.
		return []core.Finding{{
			Analyzer:   a.Name(),
			RuleID:     "toolchain-not-installed",
			Severity:   core.SeverityInfo,
			Level:      core.SeverityInfo.String(),
			Category:   "quality",
			Confidence: core.ConfidenceHint,
			Message:    "Tailwind is in use but the tailwind-lint toolchain is not installed; class checks were skipped.",
			Fix:        "cd _toolchains/tailwind && npm install",
		}}, nil
	}

	bin := filepath.Join(dir, "node_modules", ".bin", "eslint")
	cfg := filepath.Join(dir, "eslint.config.mjs")

	args := []string{"--config", cfg, "--format", "json"}
	args = append(args, targets...)
	cmd := exec.Command(bin, args...)
	cmd.Dir = ctx.Root // so eslint does not ignore the files and the plugin can resolve the project's tailwindcss + config
	out, err := cmd.Output()
	// eslint exits non-zero when it reports problems; that is not a failure.
	// A real failure produces no JSON on stdout.
	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return nil, execx.Err(err)
		}
		if len(out) == 0 {
			return nil, execx.Err(err)
		}
	}

	var parsed []eslintFile
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, err
	}

	var findings []core.Finding
	for _, f := range parsed {
		for _, m := range f.Messages {
			if m.RuleID == "" {
				continue // parser/ignore notices, not a rule violation
			}
			sev := mapSeverity(m.Severity)
			findings = append(findings, core.Finding{
				Analyzer:   a.Name(),
				RuleID:     strings.TrimPrefix(m.RuleID, "tailwindcss/"),
				Severity:   sev,
				Level:      sev.String(),
				Category:   classify(m.RuleID),
				Confidence: confidence(m.RuleID),
				File:       f.FilePath,
				Line:       m.Line,
				Message:    m.Message,
			})
		}
	}
	return findings, nil
}

// hasTailwindInstalled reports whether tailwindcss is resolvable from root's
// node_modules — the precondition the plugin needs to run without crashing.
func hasTailwindInstalled(root string) bool {
	info, err := os.Stat(filepath.Join(root, "node_modules", "tailwindcss"))
	return err == nil && info.IsDir()
}

type eslintFile struct {
	FilePath string `json:"filePath"`
	Messages []struct {
		RuleID   string `json:"ruleId"`
		Severity int    `json:"severity"`
		Message  string `json:"message"`
		Line     int    `json:"line"`
	} `json:"messages"`
}

func mapSeverity(s int) core.Severity {
	switch s {
	case 2:
		return core.SeverityError
	case 1:
		return core.SeverityWarning
	default:
		return core.SeverityInfo
	}
}

// classify maps a Tailwind rule to a finding category. Conflicting classnames
// are a real visual bug (ambiguous outcome); shorthand collapsing is a
// consistency/design concern.
func classify(ruleID string) string {
	if strings.Contains(ruleID, "no-contradicting-classname") {
		return "bug"
	}
	return "quality"
}

// confidence: a conflicting-classname is deterministic (the conflict is real),
// so it is a rule. Shorthand collapsing is a stylistic suggestion the agent
// should weigh, so it is a hint.
func confidence(ruleID string) string {
	if strings.Contains(ruleID, "no-contradicting-classname") {
		return core.ConfidenceRule
	}
	return core.ConfidenceHint
}
