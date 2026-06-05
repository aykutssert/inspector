package app

import (
	"os/exec"
	"strings"

	"github.com/aykutssert/inspector/internal/toolchain"
)

type DiagnosticStatus string

const (
	StatusOk      DiagnosticStatus = "OK"
	StatusWarning DiagnosticStatus = "WARNING"
	StatusError   DiagnosticStatus = "ERROR"
)

type CheckResult struct {
	Name        string           `json:"name"`
	Type        string           `json:"type"` // "system" or "toolchain"
	Status      DiagnosticStatus `json:"status"`
	Path        string           `json:"path,omitempty"`
	Version     string           `json:"version,omitempty"`
	InstallHint string           `json:"install_hint,omitempty"`
	Error       string           `json:"error,omitempty"`
}

type Diagnostics struct {
	OverallStatus DiagnosticStatus `json:"overall_status"`
	Results       []CheckResult    `json:"results"`
}

func Diagnose() Diagnostics {
	var results []CheckResult
	overall := StatusOk

	// 1. System Dependencies
	systems := []struct {
		name        string
		critical    bool
		installHint string
	}{
		{"node", true, "install Node.js (https://nodejs.org)"},
		{"git", true, "install Git (https://git-scm.com)"},
		{"semgrep", false, "install semgrep (pip install semgrep)"},
		{"oxlint", false, "install oxlint (npm i -g oxlint)"},
		{"osv-scanner", false, "install osv-scanner (https://github.com/google/osv-scanner)"},
	}

	for _, s := range systems {
		path, err := exec.LookPath(s.name)
		if err != nil {
			status := StatusWarning
			if s.critical {
				status = StatusError
				overall = StatusError
			} else if overall != StatusError {
				overall = StatusWarning
			}
			results = append(results, CheckResult{
				Name:        s.name,
				Type:        "system",
				Status:      status,
				InstallHint: s.installHint,
				Error:       "not found in PATH",
			})
			continue
		}

		// Try to run version check
		version := "unknown"
		cmd := exec.Command(path, "--version")
		if output, err := cmd.CombinedOutput(); err == nil {
			version = strings.TrimSpace(string(output))
			// Clean up version string to keep it neat
			if s.name == "osv-scanner" {
				// osv-scanner outputs multiline version; extract first line
				lines := strings.Split(version, "\n")
				if len(lines) > 0 {
					version = strings.TrimSpace(lines[0])
				}
			} else if s.name == "oxlint" {
				// oxlint prints "Version: X.Y.Z"
				version = strings.TrimPrefix(version, "Version: ")
			}
		}

		results = append(results, CheckResult{
			Name:    s.name,
			Type:    "system",
			Status:  StatusOk,
			Path:    path,
			Version: version,
		})
	}

	// 2. Managed Toolchains
	toolchains := []struct {
		name        string
		binary      string
		installHint string
	}{
		{"knip", "knip", "run 'npm install' inside _toolchains/knip"},
		{"typescript", "eslint", "run 'npm install' inside _toolchains/typescript"},
		{"svelte", "eslint", "run 'npm install' inside _toolchains/svelte"},
		{"tailwind", "eslint", "run 'npm install' inside _toolchains/tailwind"},
	}

	for _, tc := range toolchains {
		path, ok := toolchain.Bin(tc.name, tc.binary)
		if !ok {
			if overall != StatusError {
				overall = StatusWarning
			}
			results = append(results, CheckResult{
				Name:        tc.name,
				Type:        "toolchain",
				Status:      StatusWarning,
				InstallHint: tc.installHint,
				Error:       "binary not found in toolchain path",
			})
			continue
		}

		// Try to run version check
		version := "unknown"
		cmd := exec.Command(path, "--version")
		if output, err := cmd.CombinedOutput(); err == nil {
			version = strings.TrimSpace(string(output))
		}

		results = append(results, CheckResult{
			Name:    tc.name,
			Type:    "toolchain",
			Status:  StatusOk,
			Path:    path,
			Version: version,
		})
	}

	return Diagnostics{
		OverallStatus: overall,
		Results:       results,
	}
}
