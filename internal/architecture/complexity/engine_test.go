package complexity

import "testing"

func TestAnalyze(t *testing.T) {
	rules := []Rule{
		{
			ID:        "god-class",
			Type:      "class",
			MaxLines:  200,
			MaxInputs: 10,
			MaxDeps:   8,
			Message:   "Class is too complex; refactor it.",
		},
		{
			ID:        "large-function",
			Type:      "function",
			MaxLines:  100,
			MaxInputs: 5,
			MaxDeps:   4,
			Message:   "Function is too complex; split it.",
		},
	}

	tests := []struct {
		name      string
		filePath  string
		entities  []Entity
		wantCount int
		wantRules []string
	}{
		{
			name:     "all entities below limits",
			filePath: "safe.ts",
			entities: []Entity{
				{
					Name:      "SafeClass",
					Type:      "class",
					LineCount: 80,
					Inputs:    4,
					Deps:      3,
					StartLine: 10,
				},
				{
					Name:      "safeFunc",
					Type:      "function",
					LineCount: 20,
					Inputs:    2,
					Deps:      1,
					StartLine: 100,
				},
			},
			wantCount: 0,
		},
		{
			name:     "class lines exceeded",
			filePath: "long-class.ts",
			entities: []Entity{
				{
					Name:      "LongClass",
					Type:      "class",
					LineCount: 250,
					Inputs:    2,
					Deps:      2,
					StartLine: 5,
				},
			},
			wantCount: 1,
			wantRules: []string{"god-class"},
		},
		{
			name:     "class inputs exceeded",
			filePath: "busy-class.ts",
			entities: []Entity{
				{
					Name:      "BusyClass",
					Type:      "class",
					LineCount: 50,
					Inputs:    15,
					Deps:      1,
					StartLine: 10,
				},
			},
			wantCount: 1,
			wantRules: []string{"god-class"},
		},
		{
			name:     "function moderately large and busy (60% threshold)",
			filePath: "busy-func.ts",
			entities: []Entity{
				{
					Name:      "BusyFunc",
					Type:      "function",
					LineCount: 70, // 70 >= 100 * 0.6
					Inputs:    4,  // 4 >= 5 * 0.6
					Deps:      1,
					StartLine: 20,
				},
			},
			wantCount: 1,
			wantRules: []string{"large-function"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations := Analyze(tt.filePath, tt.entities, rules)
			if len(violations) != tt.wantCount {
				t.Fatalf("expected %d violations, got %d: %+v", tt.wantCount, len(violations), violations)
			}
			for i, v := range violations {
				if v.RuleID != tt.wantRules[i] {
					t.Errorf("expected violation rule ID %q, got %q", tt.wantRules[i], v.RuleID)
				}
				if v.File != tt.filePath {
					t.Errorf("expected file %q, got %q", tt.filePath, v.File)
				}
			}
		})
	}
}
