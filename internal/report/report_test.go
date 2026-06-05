package report

import (
	"bytes"
	"strings"
	"testing"

	"github.com/aykutssert/inspector/internal/core"
)

func TestReportTerminal(t *testing.T) {
	rep := core.Report{
		Summary: core.Summary{
			Total:    1,
			Counts:   map[string]int{"error": 1},
			TopFiles: []core.FileCount{{File: "main.go", Count: 1}},
			Score:    95,
		},
		Findings: []core.Finding{
			{
				Analyzer: "semgrep",
				File:     "main.go",
				Line:     10,
				Level:    "error",
				Message:  "Null pointer check missing",
			},
		},
	}

	var buf bytes.Buffer
	Terminal(&buf, rep)
	output := buf.String()

	// Verify that the health score is printed in Terminal output
	if !strings.Contains(output, "Health Score: 95/100") {
		t.Errorf("Terminal output missing Health Score: %q", output)
	}

	// Verify other basic elements
	if !strings.Contains(output, "1 finding(s)") {
		t.Errorf("Terminal output missing finding count: %q", output)
	}
}

func TestReportJSON(t *testing.T) {
	rep := core.Report{
		Summary: core.Summary{
			Total:    1,
			Counts:   map[string]int{"error": 1},
			TopFiles: []core.FileCount{{File: "main.go", Count: 1}},
			Score:    95,
		},
		Findings: []core.Finding{
			{
				Analyzer: "semgrep",
				File:     "main.go",
				Line:     10,
				Level:    "error",
				Message:  "Null pointer check missing",
			},
		},
	}

	var buf bytes.Buffer
	err := JSON(&buf, rep)
	if err != nil {
		t.Fatalf("JSON serialization failed: %v", err)
	}
	output := buf.String()

	// Verify that the health score is included in JSON output
	if !strings.Contains(output, `"score": 95`) {
		t.Errorf("JSON output missing score field: %q", output)
	}
}
