package core

import (
	"testing"
)

func TestBuildSummary(t *testing.T) {
	tests := []struct {
		name      string
		findings  []Finding
		fileCount int
		wantScore int
	}{
		{
			name:      "zero file count gives 100",
			findings:  []Finding{{Level: "error"}},
			fileCount: 0,
			wantScore: 100,
		},
		{
			name:      "no findings gives 100",
			findings:  []Finding{},
			fileCount: 10,
			wantScore: 100,
		},
		{
			name: "single error with 10 files gives 95",
			findings: []Finding{
				{Level: "error"},
			},
			fileCount: 10,
			wantScore: 95,
		},
		{
			name: "two errors with 1 file gives 0",
			findings: []Finding{
				{Level: "error"},
				{Level: "error"},
			},
			fileCount: 1,
			wantScore: 0,
		},
		{
			name: "three errors with 1 file capped at 0",
			findings: []Finding{
				{Level: "error"},
				{Level: "error"},
				{Level: "error"},
			},
			fileCount: 1,
			wantScore: 0,
		},
		{
			name: "mixed findings with rounding up",
			findings: []Finding{
				{Level: "error"},   // E = 1 (weight 5)
				{Level: "warning"}, // W = 2 (weight 2 * 2 = 4)
				{Level: "warning"},
				{Level: "info"}, // H = 3 (weight 0.5 * 3 = 1.5)
				{Level: "info"},
				{Level: "info"},
			}, // total weight = 10.5
			fileCount: 10, // denominator = 10 * 10 = 100
			// (1 - 10.5/100) * 100 = 89.5 -> rounds to 90
			wantScore: 90,
		},
		{
			name: "mixed findings with rounding down",
			findings: []Finding{
				{Level: "error"},   // E = 1 (weight 5)
				{Level: "warning"}, // W = 1 (weight 2)
				{Level: "info"},    // H = 1 (weight 0.5)
			}, // total weight = 7.5
			fileCount: 10, // denominator = 100
			// (1 - 7.5/100) * 100 = 92.5 -> rounds to 93
			wantScore: 93,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildSummary(tt.findings, tt.fileCount)
			if got.Score != tt.wantScore {
				t.Errorf("buildSummary() Score = %v, want %v", got.Score, tt.wantScore)
			}
		})
	}
}
