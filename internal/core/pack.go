package core

import inspectctx "github.com/aykutssert/scout/internal/context"

// Pack describes one product capability slice: a language, framework, or domain
// area that contributes adapters, analyzers, or context support. Concrete packs
// live next to the code they drive (internal/lang/<language>); the registry that
// assembles them lives in internal/registry. The contract sits in core so a pack
// can implement it without importing the registry (which would be a cycle).
type Pack interface {
	ID() string
	Detect(ctx ProjectContext) Detection
	Coverage() Coverage
	Toolchains() []Toolchain
	ScanAdapters(rulesDir string) []LanguageAdapter
	ContextAdapters() []LanguageAdapter
	ContextParsers() []inspectctx.FileParser
	ContextProviders() []inspectctx.Provider
	Analyzers() []Analyzer
}

type Detection struct {
	Matched bool
	Reason  string
}

type Coverage struct {
	Security bool
	Hints    bool
	Context  bool
}

type Toolchain struct {
	Name string
	Path string
}

// ContainsLanguage reports whether the project context detected the given
// language. Packs use it in Detect to gate themselves on the languages actually
// present in the scanned repo.
func ContainsLanguage(ctx ProjectContext, lang string) bool {
	for _, got := range ctx.Languages {
		if got == lang {
			return true
		}
	}
	return false
}
