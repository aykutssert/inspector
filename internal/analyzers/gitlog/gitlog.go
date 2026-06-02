package gitlog

import (
	"os/exec"
	"strings"

	"github.com/aykutssert/inspector/internal/core"
)

type Analyzer struct {
	RiskThreshold int
}

func New() *Analyzer { return &Analyzer{RiskThreshold: 3} }

var _ core.Analyzer = (*Analyzer)(nil)

func (a *Analyzer) Name() string { return "git-log" }

func (a *Analyzer) Available() bool {
	if _, err := exec.LookPath("git"); err != nil {
		return false
	}
	return true
}

func (a *Analyzer) Scan(ctx core.ProjectContext) ([]core.Finding, error) {
	// %x00 (NUL) separates commits; remaining lines are file paths.
	cmd := exec.Command("git", "log",
		"-i", "-E",
		"--grep=fix|bug|security|vuln|cve",
		"--name-only",
		"--pretty=format:%x00",
	)
	cmd.Dir = ctx.Root
	out, err := cmd.Output()
	if err != nil {
		// not a git repo / empty history → skip, not an error
		return nil, nil
	}

	counts := map[string]int{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "\x00" {
			continue
		}
		counts[line]++
	}

	var findings []core.Finding
	for file, n := range counts {
		if n < a.RiskThreshold {
			continue
		}
		findings = append(findings, core.Finding{
			Analyzer: a.Name(),
			RuleID:   "historically-risky-file",
			Severity: core.SeverityInfo,
			Level:    core.SeverityInfo.String(),
			File:     file,
			Message:  "This file had many past fix/bug/security commits — review changes here carefully.",
			Context:  "fix-related commits touching this file: " + itoa(n),
		})
	}
	return findings, nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
