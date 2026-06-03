// Package tsc wraps `tsc --noEmit` to surface real TypeScript type errors —
// the class of bug oxlint cannot see because it has no type information.
package tsc

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/aykutssert/inspector/internal/core"
	"github.com/aykutssert/inspector/internal/execx"
	"github.com/aykutssert/inspector/internal/toolchain"
)

type Analyzer struct{}

func New() *Analyzer { return &Analyzer{} }

var _ core.Analyzer = (*Analyzer)(nil)

func (a *Analyzer) Name() string { return "tsc" }

func (a *Analyzer) Available() bool { return true }

// tsc emits "path(line,col): error TS1234: message".
var diagRe = regexp.MustCompile(`^(.+?)\((\d+),(\d+)\): (error|warning) (TS\d+): (.+)$`)

func hasTSFiles(files []string) bool {
	for _, f := range files {
		switch strings.ToLower(filepath.Ext(f)) {
		case ".ts", ".tsx", ".mts", ".cts":
			return true
		}
	}
	return false
}

func (a *Analyzer) Scan(ctx core.ProjectContext) ([]core.Finding, error) {
	if !hasTSFiles(ctx.Files) {
		return nil, nil
	}
	tsconfig := filepath.Join(ctx.Root, "tsconfig.json")
	if _, err := os.Stat(tsconfig); err != nil {
		return nil, nil // not a tsconfig-driven project; tsc needs one
	}
	// Type checking resolves third-party types from the repo's node_modules.
	// Without it, every external import errors as "cannot find module" and floods
	// the report, so we skip and say so rather than emit noise.
	if _, err := os.Stat(filepath.Join(ctx.Root, "node_modules")); err != nil {
		return []core.Finding{skipNotice(a.Name(),
			"TypeScript project found but node_modules is missing; type checking was skipped to avoid false module-resolution errors.")}, nil
	}

	bin := resolveTSC(ctx.Root)
	if bin == "" {
		return []core.Finding{skipNotice(a.Name(),
			"TypeScript project found but no tsc binary is available (repo or toolchain); type checking was skipped.")}, nil
	}

	cmd := exec.Command(bin, "--noEmit", "--pretty", "false", "-p", "tsconfig.json")
	cmd.Dir = ctx.Root
	out, err := cmd.Output()
	if err != nil {
		// tsc exits non-zero when it reports errors; that is expected. A genuine
		// failure (bad config, crash) produces no diagnostics on stdout.
		if _, ok := err.(*exec.ExitError); !ok {
			return nil, execx.Err(err)
		}
		if len(out) == 0 {
			return nil, execx.Err(err)
		}
	}

	var findings []core.Finding
	for _, line := range strings.Split(string(out), "\n") {
		m := diagRe.FindStringSubmatch(strings.TrimRight(line, "\r"))
		if m == nil {
			continue
		}
		ln, _ := strconv.Atoi(m[2])
		sev := core.SeverityError
		if m[4] == "warning" {
			sev = core.SeverityWarning
		}
		findings = append(findings, core.Finding{
			Analyzer:   a.Name(),
			RuleID:     m[5], // TS error code
			Severity:   sev,
			Level:      sev.String(),
			Category:   "bug",
			Confidence: core.ConfidenceRule,
			File:       m[1],
			Line:       ln,
			Message:    m[6],
		})
	}
	return findings, nil
}

// resolveTSC prefers the repo's own tsc (matching the project's TS version) and
// falls back to inspector's managed toolchain.
func resolveTSC(root string) string {
	local := filepath.Join(root, "node_modules", ".bin", "tsc")
	if _, err := os.Stat(local); err == nil {
		return local
	}
	if dir, ok := toolchain.Dir("typescript"); ok {
		managed := filepath.Join(dir, "node_modules", ".bin", "tsc")
		if _, err := os.Stat(managed); err == nil {
			return managed
		}
	}
	return ""
}

func skipNotice(analyzer, msg string) core.Finding {
	return core.Finding{
		Analyzer:   analyzer,
		RuleID:     "type-check-skipped",
		Severity:   core.SeverityInfo,
		Level:      core.SeverityInfo.String(),
		Category:   "quality",
		Confidence: core.ConfidenceHint,
		Message:    msg,
		Fix:        "run `npm install` in the project so types resolve",
	}
}
