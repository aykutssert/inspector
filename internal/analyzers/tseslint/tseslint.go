// Package tseslint wraps eslint + typescript-eslint type-aware rules. These
// catch semantic bugs that need the type checker — unhandled/misused promises,
// awaiting non-thenables — which oxlint (no type info) cannot detect.
package tseslint

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aykutssert/inspector/internal/core"
	"github.com/aykutssert/inspector/internal/execx"
	"github.com/aykutssert/inspector/internal/toolchain"
)

type Analyzer struct{}

func New() *Analyzer { return &Analyzer{} }

var _ core.Analyzer = (*Analyzer)(nil)

func (a *Analyzer) Name() string { return "ts-eslint" }

func (a *Analyzer) Available() bool { return true }

func (a *Analyzer) Scan(ctx core.ProjectContext) ([]core.Finding, error) {
	var targets []string
	for _, f := range ctx.Files {
		switch strings.ToLower(filepath.Ext(f)) {
		case ".ts", ".tsx", ".mts", ".cts":
			targets = append(targets, f)
		}
	}
	if len(targets) == 0 {
		return nil, nil
	}
	if _, err := os.Stat(filepath.Join(ctx.Root, "tsconfig.json")); err != nil {
		return nil, nil // type-aware rules need a project; nothing to do
	}
	// Type-aware linting resolves types via the repo's node_modules; without it
	// the parser cannot build a program and would error on every file.
	if _, err := os.Stat(filepath.Join(ctx.Root, "node_modules")); err != nil {
		return []core.Finding{skipNotice(a.Name(),
			"TypeScript project found but node_modules is missing; type-aware lint was skipped.")}, nil
	}
	dir, ok := toolchain.Dir("typescript")
	if !ok {
		return []core.Finding{skipNotice(a.Name(),
			"TypeScript files found but the type-aware toolchain is not installed; type-aware lint was skipped.")}, nil
	}

	bin := filepath.Join(dir, "node_modules", ".bin", "eslint")
	cfg := filepath.Join(dir, "eslint.config.mjs")
	args := []string{"--config", cfg, "--format", "json"}
	args = append(args, targets...)
	cmd := exec.Command(bin, args...)
	cmd.Dir = ctx.Root
	out, err := cmd.Output()
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
				continue // parser notices (e.g. file outside project), not rules
			}
			sev := mapSeverity(m.Severity)
			findings = append(findings, core.Finding{
				Analyzer:   a.Name(),
				RuleID:     strings.TrimPrefix(m.RuleID, "@typescript-eslint/"),
				Severity:   sev,
				Level:      sev.String(),
				Category:   classify(sev),
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

func classify(sev core.Severity) string {
	if sev == core.SeverityError {
		return "bug"
	}
	return "quality"
}

func skipNotice(analyzer, msg string) core.Finding {
	return core.Finding{
		Analyzer:   analyzer,
		RuleID:     "type-aware-skipped",
		Severity:   core.SeverityInfo,
		Level:      core.SeverityInfo.String(),
		Category:   "quality",
		Confidence: core.ConfidenceHint,
		Message:    msg,
		Fix:        "run `npm install` in the project so types resolve",
	}
}
