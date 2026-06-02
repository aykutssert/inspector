package osv

import (
	"encoding/json"
	"os/exec"
	"path/filepath"

	"github.com/aykutssert/inspector/internal/core"
)

type Analyzer struct{}

func New() *Analyzer { return &Analyzer{} }

var _ core.Analyzer = (*Analyzer)(nil)

func (a *Analyzer) Name() string { return "osv" }

func (a *Analyzer) Available() bool {
	_, err := exec.LookPath("osv-scanner")
	return err == nil
}

type osvOut struct {
	Results []struct {
		Source struct {
			Path string `json:"path"`
		} `json:"source"`
		Packages []struct {
			Package struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"package"`
			Vulnerabilities []struct {
				ID      string `json:"id"`
				Summary string `json:"summary"`
			} `json:"vulnerabilities"`
		} `json:"packages"`
	} `json:"results"`
}

func (a *Analyzer) Scan(ctx core.ProjectContext) ([]core.Finding, error) {
	cmd := exec.Command("osv-scanner", "--format", "json", "-r", ctx.Root)
	cmd.Dir = ctx.Root
	out, err := cmd.Output()
	if err != nil && len(out) == 0 {
		return nil, err
	}
	var parsed osvOut
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, err
	}
	var findings []core.Finding
	for _, res := range parsed.Results {
		file := res.Source.Path
		if rel, err := filepath.Rel(ctx.Root, file); err == nil {
			file = rel
		}
		for _, pkg := range res.Packages {
			for _, v := range pkg.Vulnerabilities {
				findings = append(findings, core.Finding{
					Analyzer: a.Name(),
					RuleID:   v.ID,
					Severity: core.SeverityError,
					Level:    core.SeverityError.String(),
					File:     file,
					Message:  pkg.Package.Name + "@" + pkg.Package.Version + ": " + v.Summary,
					Fix:      "Upgrade " + pkg.Package.Name + " to a version without " + v.ID,
				})
			}
		}
	}
	return findings, nil
}
