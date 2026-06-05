package app

import (
	"strings"
	"testing"
)

func TestDiagnose(t *testing.T) {
	diag := Diagnose()

	// Overall status should not be empty
	if diag.OverallStatus == "" {
		t.Error("OverallStatus should not be empty")
	}

	// Find node and git results and check correctness
	var hasNode, hasGit bool
	for _, res := range diag.Results {
		if res.Name == "node" {
			hasNode = true
			if res.Status != StatusOk {
				t.Errorf("node is expected to be StatusOk, got %v (error: %s)", res.Status, res.Error)
			}
			if res.Path == "" {
				t.Error("node path should not be empty")
			}
			if res.Version == "" || res.Version == "unknown" {
				t.Error("node version should not be empty/unknown")
			}
		}
		if res.Name == "git" {
			hasGit = true
			if res.Status != StatusOk {
				t.Errorf("git is expected to be StatusOk, got %v (error: %s)", res.Status, res.Error)
			}
			if res.Path == "" {
				t.Error("git path should not be empty")
			}
			if !strings.Contains(res.Version, "git version") {
				t.Errorf("git version expected to contain 'git version', got %q", res.Version)
			}
		}
	}

	if !hasNode {
		t.Error("Diagnostics results missing check for 'node'")
	}
	if !hasGit {
		t.Error("Diagnostics results missing check for 'git'")
	}
}
