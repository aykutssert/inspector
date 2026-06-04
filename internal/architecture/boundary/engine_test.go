package boundary

import "testing"

func TestAnalyzeReportsDirectBoundaryViolation(t *testing.T) {
	g := Graph{
		Files: []string{"user.controller.ts", "user.repository.ts"},
		Edges: map[string][]Edge{
			"user.controller.ts": {{From: "user.controller.ts", To: "user.repository.ts", Source: "./user.repository", Line: 3}},
		},
	}
	got := Analyze(g, testClassifier, []Rule{layerRule()})

	if len(got) != 1 {
		t.Fatalf("violations = %+v, want one", got)
	}
	if got[0].Line != 3 || got[0].ImportSource != "./user.repository" {
		t.Fatalf("violation import = %s:%d", got[0].ImportSource, got[0].Line)
	}
	if got[0].TransparentEntry {
		t.Fatalf("direct violation should not be marked transparent")
	}
}

func TestAnalyzeFollowsTransparentBarrel(t *testing.T) {
	g := Graph{
		Files: []string{"user.controller.ts", "data/index.ts", "data/user.repository.ts"},
		Edges: map[string][]Edge{
			"user.controller.ts": {{From: "user.controller.ts", To: "data/index.ts", Source: "./data", Line: 3}},
			"data/index.ts":      {{From: "data/index.ts", To: "data/user.repository.ts", Source: "./user.repository", Line: 1}},
		},
	}
	got := Analyze(g, testClassifier, []Rule{layerRule()})

	if len(got) != 1 {
		t.Fatalf("violations = %+v, want one", got)
	}
	if !got[0].TransparentEntry {
		t.Fatalf("barrel violation should be marked transparent")
	}
	if len(got[0].Chain) != 3 || got[0].Chain[1] != "data/index.ts" || got[0].Chain[2] != "data/user.repository.ts" {
		t.Fatalf("chain = %+v", got[0].Chain)
	}
}

func TestAnalyzeDoesNotFollowServiceLayerTransitively(t *testing.T) {
	g := Graph{
		Files: []string{"user.controller.ts", "user.service.ts", "user.repository.ts"},
		Edges: map[string][]Edge{
			"user.controller.ts": {{From: "user.controller.ts", To: "user.service.ts", Source: "./user.service", Line: 3}},
			"user.service.ts":    {{From: "user.service.ts", To: "user.repository.ts", Source: "./user.repository", Line: 2}},
		},
	}
	got := Analyze(g, testClassifier, []Rule{layerRule()})

	if len(got) != 0 {
		t.Fatalf("violations = %+v, want none", got)
	}
}

func layerRule() Rule {
	return Rule{
		ID:          "architecture.layered-boundary",
		SourceKind:  "controller",
		TargetKinds: []string{"repository"},
		PassThrough: func(file string) bool {
			return file == "data/index.ts"
		},
	}
}

func testClassifier(file string) []Tag {
	switch file {
	case "user.controller.ts":
		return []Tag{{Kind: "controller", Reason: "test", Confidence: "rule"}}
	case "user.repository.ts", "data/user.repository.ts":
		return []Tag{{Kind: "repository", Reason: "test", Confidence: "rule"}}
	default:
		return nil
	}
}
