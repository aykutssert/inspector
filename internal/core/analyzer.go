package core

type ProjectContext struct {
	Root      string
	DiffOnly  bool
	Files     []string
	Languages []string
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
