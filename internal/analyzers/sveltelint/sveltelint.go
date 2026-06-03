package sveltelint

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aykutssert/inspector/internal/core"
)

// Analyzer wraps eslint + eslint-plugin-svelte, the proven Svelte linter, run
// from a managed toolchain that inspector ships under linters/svelte. We do not
// author Svelte rules ourselves; we wrap the ecosystem's linter.
type Analyzer struct{}

func New() *Analyzer { return &Analyzer{} }

var _ core.Analyzer = (*Analyzer)(nil)

func (a *Analyzer) Name() string { return "svelte-lint" }

func (a *Analyzer) InstallHint() string {
	return "install the Svelte toolchain: cd linters/svelte && npm install"
}

// Available reports true only when the managed toolchain is installed; a missing
// node_modules makes the orchestrator skip this analyzer instead of failing.
func (a *Analyzer) Available() bool {
	return toolchainDir() != ""
}

// toolchainDir locates the managed Svelte lint toolchain by looking for an
// installed eslint binary, in priority order: $INSPECTOR_HOME, the directory of
// the running executable, then the current working directory (dev checkout).
// Returns "" when none has node_modules installed.
func toolchainDir() string {
	var bases []string
	if home := os.Getenv("INSPECTOR_HOME"); home != "" {
		bases = append(bases, home)
	}
	if exe, err := os.Executable(); err == nil {
		bases = append(bases, filepath.Dir(exe))
	}
	if wd, err := os.Getwd(); err == nil {
		bases = append(bases, wd)
	}
	for _, b := range bases {
		dir := filepath.Join(b, "linters", "svelte")
		if _, err := os.Stat(filepath.Join(dir, "node_modules", ".bin", "eslint")); err == nil {
			return dir
		}
	}
	return ""
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

	dir := toolchainDir()
	if dir == "" {
		return nil, nil // Available() already gates this; defensive
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
			return nil, err
		}
		if len(out) == 0 {
			return nil, err
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

// classify maps a Svelte rule id to a finding category. a11y rules are quality;
// otherwise severity decides: errors are likely bugs, warnings quality.
func classify(ruleID string, sev core.Severity) string {
	if strings.Contains(ruleID, "a11y") {
		return "quality"
	}
	if sev == core.SeverityError {
		return "bug"
	}
	return "quality"
}
