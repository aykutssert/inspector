package duplication

import (
	"testing"
)

func TestDuplicationEngine(t *testing.T) {
	literals := []Literal{
		{Value: `"auth"`, Kind: "string", Line: 5},
		{Value: `"auth"`, Kind: "string", Line: 10},
		{Value: `100`, Kind: "number", Line: 12},
		{Value: `"auth"`, Kind: "string", Line: 15},
		{Value: `100`, Kind: "number", Line: 20},
		{Value: `"auth"`, Kind: "string", Line: 25},
		{Value: `100`, Kind: "number", Line: 30},
	}

	rules := []Rule{
		{
			ID:              "repeated-magic-literal",
			ThresholdString: 4,
			ThresholdNumber: 3,
			MaxViolations:   5,
		},
	}

	violations := Analyze("test.js", literals, rules)

	if len(violations) != 2 {
		t.Fatalf("expected 2 violations, got %d", len(violations))
	}

	// Verify order by Line
	if violations[0].Line != 5 {
		t.Errorf("expected first violation at line 5, got %d", violations[0].Line)
	}
	if violations[1].Line != 12 {
		t.Errorf("expected second violation at line 12, got %d", violations[1].Line)
	}

	// Verify counts
	if violations[0].Count != 4 {
		t.Errorf("expected count 4 for string literal, got %d", violations[0].Count)
	}
	if violations[1].Count != 3 {
		t.Errorf("expected count 3 for number literal, got %d", violations[1].Count)
	}
}

func TestDuplicationEngineMaxViolationsLimit(t *testing.T) {
	literals := []Literal{
		{Value: `"a"`, Kind: "string", Line: 2},
		{Value: `"a"`, Kind: "string", Line: 3},
		{Value: `"b"`, Kind: "string", Line: 4},
		{Value: `"b"`, Kind: "string", Line: 5},
		{Value: `"a"`, Kind: "string", Line: 6},
		{Value: `"b"`, Kind: "string", Line: 7},
	}

	rules := []Rule{
		{
			ID:              "repeated-magic-literal",
			ThresholdString: 2,
			MaxViolations:   1, // Limit to 1 violation
		},
	}

	violations := Analyze("test.js", literals, rules)

	if len(violations) != 1 {
		t.Fatalf("expected 1 violation due to limit, got %d", len(violations))
	}

	// Expected to return the first one (first line 2)
	if violations[0].Value != `"a"` {
		t.Errorf("expected violation for 'a', got %s", violations[0].Value)
	}
}
