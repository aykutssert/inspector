package core

type ProjectContext struct {
	Root      string
	DiffOnly  bool
	Files     []string
	Languages []string
	// FailClosed turns a missing tool or analyzer error into an error-level
	// finding (non-zero exit) instead of a silent skip — for CI gating.
	FailClosed bool
}

// add a new analyzer by implementing this; the orchestrator never changes
type Analyzer interface {
	Name() string
	// false → orchestrator skips it instead of failing the whole scan
	Available() bool
	Scan(ctx ProjectContext) ([]Finding, error)
}

// add a language by implementing this; core stays untouched
type LanguageAdapter interface {
	Language() string
	Matches(path string) bool
	Rules() []string
}
