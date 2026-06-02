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
		} `json:"extra"`
	} `json:"results"`
}

func (a *Analyzer) Scan(ctx core.ProjectContext) ([]core.Finding, error) {
	args := []string{"--json", "--quiet", "--metrics=off", "--config", a.config}
	if len(ctx.Files) > 0 {
		args = append(args, ctx.Files...)
	} else {
		args = append(args, ctx.Root)
	}
	cmd := exec.Command("semgrep", args...)
	cmd.Dir = ctx.Root
	out, err := cmd.Output()
	if err != nil {
		// semgrep exits non-zero when findings exist; only empty output is a real failure
		if len(out) == 0 {
			return nil, err
		}
	}
	var parsed semgrepOut
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, err
	}
	var findings []core.Finding
	for _, r := range parsed.Results {
		sev := mapSeverity(r.Extra.Severity)
		findings = append(findings, core.Finding{
			Analyzer: a.Name(),
			RuleID:   r.CheckID,
			Severity: sev,
			Level:    sev.String(),
			File:     r.Path,
			Line:     r.Start.Line,
			Message:  r.Extra.Message,
			Fix:      r.Extra.Fix,
		})
	}
	return findings, nil
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
