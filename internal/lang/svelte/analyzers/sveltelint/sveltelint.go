package sveltelint

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aykutssert/scout/internal/core"
	"github.com/aykutssert/scout/internal/execx"
	"github.com/aykutssert/scout/internal/toolchain"
)

// Analyzer wraps eslint + eslint-plugin-svelte, the proven Svelte linter, run
// from a managed toolchain that scout ships under _toolchains/svelte. We do not
// author Svelte rules ourselves; we wrap the ecosystem's linter.
type Analyzer struct{}

func New() *Analyzer { return &Analyzer{} }

var _ core.Analyzer = (*Analyzer)(nil)

func (a *Analyzer) Name() string { return "svelte-lint" }

// Available is always true: this analyzer is conditional, not mandatory. Whether
// it has work to do (Svelte files present) and whether its toolchain is
// installed are decided in Scan, so a non-Svelte repo is never flagged as a
// missing-scanner error by the fail-closed orchestrator.
func (a *Analyzer) Available() bool { return true }

func (a *Analyzer) Scan(ctx core.ProjectContext) ([]core.Finding, error) {
	var targets []string
	for _, f := range ctx.Files {
		if strings.EqualFold(filepath.Ext(f), ".svelte") {
			targets = append(targets, f)
		}
	}
	if len(targets) == 0 {
		return nil, nil // no Svelte files in this scan
	}

	dir, ok := toolchain.Dir("svelte")
	if !ok {
		// Svelte files exist but the toolchain is not installed: surface partial
		// coverage honestly instead of silently passing.
		return []core.Finding{{
			Analyzer:   a.Name(),
			RuleID:     "toolchain-not-installed",
			Severity:   core.SeverityInfo,
			Level:      core.SeverityInfo.String(),
			Category:   "quality",
			Confidence: core.ConfidenceHint,
			Message:    "Svelte files found but the svelte-lint toolchain is not installed; these files were not linted.",
			Fix:        "cd _toolchains/svelte && npm install",
		}}, nil
	}

	bin := filepath.Join(dir, "node_modules", ".bin", "eslint")
	cfg := filepath.Join(dir, "eslint.config.mjs")

	args := []string{"--config", cfg, "--format", "json"}
	args = append(args, targets...)
	cmd := exec.Command(bin, args...)
	cmd.Dir = ctx.Root // base path so eslint does not ignore the files
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
			if isUnknownRuleNotice(m.Message) {
				continue // project source disables a rule we don't load; not our finding
			}
			sev := mapSeverity(m.Severity)
			findings = append(findings, core.Finding{
				Analyzer:   a.Name(),
				RuleID:     m.RuleID,
				Severity:   sev,
				Level:      sev.String(),
				Category:   classify(m.RuleID, sev),
				Confidence: core.ConfidenceRule,
				File:       f.FilePath,
				Line:       m.Line,
				Message:    m.Message,
			})
		}
	}
	return findings, nil
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

// isUnknownRuleNotice reports whether an eslint message is the core
// "Definition for rule '<id>' was not found." notice. eslint emits this when
// project source carries an inline `eslint-disable` directive for a rule that
// scout's curated config doesn't load. It is the project's lint setup
// leaking, not a defect we detected, so we drop it to stay deterministic.
func isUnknownRuleNotice(msg string) bool {
	return strings.HasPrefix(msg, "Definition for rule ") && strings.HasSuffix(msg, "was not found.")
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

// classify maps a Svelte rule id to a finding category. no-at-html-tags flags
// {@html ...}, an XSS sink, so it is security regardless of severity. a11y rules
// are quality; otherwise severity decides: errors are likely bugs, warnings
// quality.
func classify(ruleID string, sev core.Severity) string {
	if strings.Contains(ruleID, "no-at-html-tags") {
		return "security"
	}
	if strings.Contains(ruleID, "a11y") {
		return "quality"
	}
	if sev == core.SeverityError {
		return "bug"
	}
	return "quality"
}
