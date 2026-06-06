package scan_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/aykutssert/scout/internal/app"
	"github.com/aykutssert/scout/internal/core"
)

func TestCorpusRegression(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping corpus regression tests in short mode")
	}

	// Determine paths relative to this test file
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	// Go back to project root
	root := filepath.Clean(filepath.Join(wd, "..", ".."))
	corpusSrc := filepath.Join(root, "_corpus", "src")
	corpusOut := filepath.Join(root, "_corpus", "out")

	// Skip test if corpus is not set up
	if _, err := os.Stat(corpusSrc); os.IsNotExist(err) {
		t.Skip("Corpus src directory not found, skipping regression tests")
	}
	if _, err := os.Stat(corpusOut); os.IsNotExist(err) {
		t.Skip("Corpus out directory not found, skipping regression tests")
	}

	// Read repos in src/
	entries, err := os.ReadDir(corpusSrc)
	if err != nil {
		t.Fatal(err)
	}

	updateExpected := os.Getenv("UPDATE_EXPECTED") == "true"

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		repoName := entry.Name()
		t.Run(repoName, func(t *testing.T) {
			repoPath := filepath.Join(corpusSrc, repoName)
			actualReport, err := app.Scan(app.ScanOptions{
				Root:     repoPath,
				DiffOnly: false,
				RulesDir: filepath.Join(root, "rules"),
			}, nil)
			if err != nil {
				t.Fatalf("scan failed for %s: %v", repoName, err)
			}

			baselinePath := filepath.Join(corpusOut, repoName+".json")

			if updateExpected {
				data, err := json.MarshalIndent(actualReport, "", "  ")
				if err != nil {
					t.Fatalf("failed to marshal report for update: %v", err)
				}
				if err := os.WriteFile(baselinePath, append(data, '\n'), 0644); err != nil {
					t.Fatalf("failed to update baseline file: %v", err)
				}
				t.Logf("Updated baseline for %s", repoName)
				return
			}

			// Read baseline
			baselineData, err := os.ReadFile(baselinePath)
			if err != nil {
				t.Fatalf("failed to read baseline for %s: %v", repoName, err)
			}

			var expectedReport core.Report
			if err := json.Unmarshal(baselineData, &expectedReport); err != nil {
				t.Fatalf("failed to unmarshal baseline for %s: %v", repoName, err)
			}

			// Compare findings count
			if len(actualReport.Findings) != len(expectedReport.Findings) {
				t.Errorf("Findings count mismatch for %s: got %d, expected %d",
					repoName, len(actualReport.Findings), len(expectedReport.Findings))
			}

			// Deep compare rule activations
			expectedRules := make(map[string]int)
			for _, f := range expectedReport.Findings {
				expectedRules[f.RuleID]++
			}

			actualRules := make(map[string]int)
			for _, f := range actualReport.Findings {
				actualRules[f.RuleID]++
			}

			// Assert rule counts match
			for ruleID, count := range expectedRules {
				if actualCount := actualRules[ruleID]; actualCount != count {
					t.Errorf("Rule %q activation count mismatch for %s: got %d, expected %d",
						ruleID, repoName, actualCount, count)
				}
			}

			for ruleID, count := range actualRules {
				if expectedCount := expectedRules[ruleID]; expectedCount == 0 {
					t.Errorf("Rule %q unexpectedly activated %d times for %s (not in baseline)",
						ruleID, count, repoName)
				}
			}
		})
	}
}
