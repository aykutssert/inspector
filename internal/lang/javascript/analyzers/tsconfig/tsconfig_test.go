package tsconfig

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/aykutssert/scout/internal/core"
)

func TestStripJSONComments(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "no comments",
			in:   `{"a": 1}`,
			want: `{"a": 1}`,
		},
		{
			name: "line comment",
			in:   `{"a": 1} // comment`,
			want: `{"a": 1} `,
		},
		{
			name: "line comment with newline",
			in:   "{\"a\": 1} // comment\n{\"b\": 2}",
			want: "{\"a\": 1} \n{\"b\": 2}",
		},
		{
			name: "block comment",
			in:   `{"a": /* comment */ 1}`,
			want: `{"a":  1}`,
		},
		{
			name: "string with slashes",
			in:   `{"url": "https://google.com", "comment": "/* value */"}`,
			want: `{"url": "https://google.com", "comment": "/* value */"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(stripJSONComments([]byte(tt.in)))
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTSConfigFindings(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		wantRuleIDs []string
	}{
		{
			name:        "empty options",
			config:      `{}`,
			wantRuleIDs: []string{"tsconfig-strict-disabled"},
		},
		{
			name: "strict false",
			config: `{
				"compilerOptions": { "strict": false }
			}`,
			wantRuleIDs: []string{"tsconfig-strict-disabled"},
		},
		{
			name: "strict true with comments",
			config: `{
				// strict check
				"compilerOptions": { "strict": true /* block comment */ }
			}`,
			wantRuleIDs: nil,
		},
		{
			name: "opt-in flags off are not flagged",
			config: `{
				"compilerOptions": {
					"strict": true,
					"noUncheckedIndexedAccess": false,
					"exactOptionalPropertyTypes": false
				}
			}`,
			wantRuleIDs: nil,
		},
		{
			name: "extends is skipped (inherited strict unknown)",
			config: `{
				"extends": "../tsconfig.base.json",
				"compilerOptions": {}
			}`,
			wantRuleIDs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(tt.config), 0o644)
			if err != nil {
				t.Fatal(err)
			}

			// We need a dummy TS file in the scan scope to trigger tsconfig check
			err = os.WriteFile(filepath.Join(dir, "index.ts"), []byte(""), 0o644)
			if err != nil {
				t.Fatal(err)
			}

			a := New()
			findings, err := a.Scan(core.ProjectContext{
				Root:  dir,
				Files: []string{"index.ts"},
			})
			if err != nil {
				t.Fatal(err)
			}

			var gotRuleIDs []string
			for _, f := range findings {
				gotRuleIDs = append(gotRuleIDs, f.RuleID)
			}

			if !reflect.DeepEqual(gotRuleIDs, tt.wantRuleIDs) {
				t.Errorf("got rule IDs %v, want %v", gotRuleIDs, tt.wantRuleIDs)
			}
		})
	}
}
