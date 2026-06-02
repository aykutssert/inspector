package core

type ProjectContext struct {
	Root      string
	DiffOnly  bool
	Files     []string
	Languages []string
	// Changed is the raw list of git-changed paths in diff mode (unfiltered by
	// language), so analyzers like osv can decide whether a dependency manifest
	// actually changed. Empty when not in diff mode.
	Changed []string
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
