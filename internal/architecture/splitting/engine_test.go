package splitting

import "testing"

func TestAnalyze(t *testing.T) {
	rule := Rule{
		ID:                    "component-splitting",
		MaxFileLines:          300,
		MaxComponentLines:     150,
		MaxExportedLargeComps: 1,
		Message:               "File contains multiple large exported components; split them into individual files.",
	}
	rules := []Rule{rule}

	tests := []struct {
		name      string
		files     []FileMetrics
		wantCount int
	}{
		{
			name: "file size below limit",
			files: []FileMetrics{
				{
					FilePath:   "SmallFile.tsx",
					TotalLines: 200,
					Components: []Component{
						{Name: "CompA", LineCount: 160, IsExported: true, StartLine: 10},
						{Name: "CompB", LineCount: 20, IsExported: true, StartLine: 180},
					},
				},
			},
			wantCount: 0,
		},
		{
			name: "file size above limit but only one large component",
			files: []FileMetrics{
				{
					FilePath:   "OneLargeFile.tsx",
					TotalLines: 400,
					Components: []Component{
						{Name: "CompA", LineCount: 250, IsExported: true, StartLine: 10},
						{Name: "CompB", LineCount: 50, IsExported: true, StartLine: 300},
					},
				},
			},
			wantCount: 0,
		},
		{
			name: "file size above limit and multiple large exported components",
			files: []FileMetrics{
				{
					FilePath:   "ViolatingFile.tsx",
					TotalLines: 400,
					Components: []Component{
						{Name: "CompA", LineCount: 180, IsExported: true, StartLine: 10},
						{Name: "CompB", LineCount: 160, IsExported: true, StartLine: 200},
					},
				},
			},
			wantCount: 1,
		},
		{
			name: "file size above limit, multiple large components, but one not exported",
			files: []FileMetrics{
				{
					FilePath:   "MixedFile.tsx",
					TotalLines: 400,
					Components: []Component{
						{Name: "CompA", LineCount: 180, IsExported: true, StartLine: 10},
						{Name: "CompB", LineCount: 160, IsExported: false, StartLine: 200},
					},
				},
			},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations := Analyze(tt.files, rules)
			if len(violations) != tt.wantCount {
				t.Fatalf("expected %d violations, got %d: %+v", tt.wantCount, len(violations), violations)
			}
			if tt.wantCount > 0 {
				v := violations[0]
				if v.RuleID != rule.ID {
					t.Errorf("expected rule ID %q, got %q", rule.ID, v.RuleID)
				}
				if v.File != tt.files[0].FilePath {
					t.Errorf("expected file %q, got %q", tt.files[0].FilePath, v.File)
				}
				if v.Message != rule.Message {
					t.Errorf("expected message %q, got %q", rule.Message, v.Message)
				}
			}
		})
	}
}
