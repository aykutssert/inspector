package osv

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aykutssert/inspector/internal/core"
)

// dependencyManifests are the files whose change warrants a dependency rescan.
var dependencyManifests = map[string]bool{
	"package.json": true, "package-lock.json": true, "yarn.lock": true,
	"pnpm-lock.yaml": true, "npm-shrinkwrap.json": true,
	"go.mod": true, "go.sum": true,
	"requirements.txt": true, "Pipfile.lock": true, "poetry.lock": true,
	"Cargo.lock": true, "composer.lock": true, "Gemfile.lock": true,
}

func manifestChanged(changed []string) bool {
	for _, f := range changed {
		if dependencyManifests[filepath.Base(strings.TrimSpace(f))] {
			return true
		}
	}
	return false
}

type Analyzer struct{}

func New() *Analyzer { return &Analyzer{} }

var _ core.Analyzer = (*Analyzer)(nil)

func (a *Analyzer) Name() string { return "osv" }

func (a *Analyzer) InstallHint() string { return "install osv-scanner" }

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
	// Dependencies are project-global, so a diff scan only makes sense when a
	// manifest actually changed; otherwise the lockfile CVEs are unchanged.
	if ctx.DiffOnly && !manifestChanged(ctx.Changed) {
		return nil, nil
	}
	cmd := exec.Command("osv-scanner", "scan", "--format", "json", "-r", ctx.Root)
	cmd.Dir = ctx.Root
	out, err := cmd.Output()
	if err != nil {
		// Exit 128 = no packages/lockfiles found: nothing to scan, not a failure.
		// Exit 1 = vulnerabilities found, with JSON on stdout — parse it below.
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 128 {
			return nil, nil
		}
		if len(out) == 0 {
			return nil, err
		}
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
					Analyzer:   a.Name(),
					RuleID:     v.ID,
					Severity:   core.SeverityError,
					Level:      core.SeverityError.String(),
					Category:   "security",
					Confidence: core.ConfidenceRule,
					File:       file,
					Message:    pkg.Package.Name + "@" + pkg.Package.Version + ": " + v.Summary,
					Fix:        "Upgrade " + pkg.Package.Name + " to a version without " + v.ID,
				})
			}
		}
	}
	return findings, nil
}
