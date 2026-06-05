// Package tseslint wraps eslint + typescript-eslint type-aware rules. These
// catch semantic bugs that need the type checker — unhandled/misused promises,
// awaiting non-thenables — which oxlint (no type info) cannot detect.
package tseslint

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aykutssert/scout/internal/core"
	"github.com/aykutssert/scout/internal/execx"
	"github.com/aykutssert/scout/internal/toolchain"
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
	// A monorepo can hold several TS projects; type-aware lint needs at least one
	// tsconfig governing the scanned files. projectService (see the toolchain's
	// eslint.config.mjs) then auto-discovers the right tsconfig per file, so a
	// single eslint run from the repo root still covers every workspace.
	dirs := tsConfigDirs(ctx)
	if len(dirs) == 0 {
		return nil, nil // no tsconfig governs the scanned files; nothing to do
	}
	// Type-aware linting resolves types via node_modules — the workspace root's
	// hoisted copy, or a project's own. Without any, the parser cannot build a
	// program and would error on every file.
	if !hasNodeModules(ctx.Root) && !anyHasNodeModules(dirs) {
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
		exit, ok := err.(*exec.ExitError)
		if !ok {
			return nil, execx.Err(err)
		}
		if len(out) == 0 {
			return []core.Finding{eslintRunFailure(a.Name(), string(exit.Stderr))}, nil
		}
	}

	var parsed []eslintFile
	if err := json.Unmarshal(out, &parsed); err != nil {
		return []core.Finding{eslintRunFailure(a.Name(), string(out))}, nil
	}

	return eslintFindings(a.Name(), parsed), nil
}

func eslintFindings(analyzer string, parsed []eslintFile) []core.Finding {
	var findings []core.Finding
	for _, f := range parsed {
		for _, m := range f.Messages {
			if isUnknownRuleNotice(m.Message) {
				continue // project source disables a rule we don't load; not our finding
			}
			ruleID := strings.TrimPrefix(m.RuleID, "@typescript-eslint/")
			confidence := core.ConfidenceRule
			if ruleID == "" {
				ruleID = "parser-error"
				confidence = core.ConfidenceHint
			}
			sev := mapSeverity(m.Severity)
			findings = append(findings, core.Finding{
				Analyzer:   analyzer,
				RuleID:     ruleID,
				Severity:   sev,
				Level:      sev.String(),
				Category:   classify(sev),
				Confidence: confidence,
				File:       f.FilePath,
				Line:       m.Line,
				Message:    m.Message,
			})
		}
	}
	return findings
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

// tsConfigDirs returns, for the scanned TS files, the directory of the nearest
// tsconfig.json walking up to root — one entry per workspace in a monorepo.
// Empty when no tsconfig governs any scanned file. Sorted and de-duplicated.
func tsConfigDirs(ctx core.ProjectContext) []string {
	seen := map[string]bool{}
	for _, f := range ctx.Files {
		switch strings.ToLower(filepath.Ext(f)) {
		case ".ts", ".tsx", ".mts", ".cts":
		default:
			continue
		}
		dir := filepath.Dir(f)
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(ctx.Root, dir)
		}
		if d := nearestTSConfigDir(dir, ctx.Root); d != "" {
			seen[d] = true
		}
	}
	out := make([]string, 0, len(seen))
	for d := range seen {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

// nearestTSConfigDir walks up from dir to root (inclusive) and returns the first
// directory that contains a tsconfig.json, or "" if none does.
func nearestTSConfigDir(dir, root string) string {
	for {
		if _, err := os.Stat(filepath.Join(dir, "tsconfig.json")); err == nil {
			return dir
		}
		if dir == root || !strings.HasPrefix(dir, root) {
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func hasNodeModules(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "node_modules"))
	return err == nil
}

func anyHasNodeModules(dirs []string) bool {
	for _, d := range dirs {
		if hasNodeModules(d) {
			return true
		}
	}
	return false
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

// isUnknownRuleNotice reports whether an eslint message is the core
// "Definition for rule '<id>' was not found." notice. eslint emits this (with
// severity 2 and a null nodeType) when project source carries an inline
// `eslint-disable` directive for a plugin rule that scout's curated config
// doesn't load (e.g. testing-library). It is the project's lint setup leaking,
// not a defect we detected, so we drop it to keep output deterministic.
func isUnknownRuleNotice(msg string) bool {
	return strings.HasPrefix(msg, "Definition for rule ") && strings.HasSuffix(msg, "was not found.")
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

func eslintRunFailure(analyzer, output string) core.Finding {
	msg := strings.TrimSpace(output)
	if msg == "" {
		msg = "eslint exited non-zero but produced no JSON output."
	}
	return core.Finding{
		Analyzer:   analyzer,
		RuleID:     "type-aware-lint-failed",
		Severity:   core.SeverityWarning,
		Level:      core.SeverityWarning.String(),
		Category:   "quality",
		Confidence: core.ConfidenceHint,
		Message:    "Type-aware lint could not complete: " + firstLine(msg),
		Fix:        "Run the project's type-aware ESLint command and fix the parser/config problem.",
	}
}

func firstLine(text string) string {
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		return text[:i]
	}
	return text
}
