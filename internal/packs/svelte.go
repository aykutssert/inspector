package packs

import (
	"github.com/aykutssert/inspector/internal/analyzers/sveltelint"
	inspectctx "github.com/aykutssert/inspector/internal/context"
	"github.com/aykutssert/inspector/internal/core"
	"github.com/aykutssert/inspector/internal/lang/svelte"
)

type sveltePack struct{}

func Svelte() Pack { return sveltePack{} }

func (sveltePack) ID() string { return "svelte" }

func (sveltePack) Detect(ctx core.ProjectContext) Detection {
	if containsLanguage(ctx, "svelte") {
		return Detection{Matched: true, Reason: "Svelte source files detected"}
	}
	return Detection{}
}

func (sveltePack) Coverage() Coverage {
	return Coverage{Security: true, Hints: false, Context: false}
}

func (sveltePack) Toolchains() []Toolchain {
	return []Toolchain{{Name: "svelte", Path: "_toolchains/svelte"}}
}

func (sveltePack) ScanAdapters(string) []core.LanguageAdapter {
	return []core.LanguageAdapter{svelte.New()}
}

func (sveltePack) ContextAdapters() []core.LanguageAdapter {
	return nil
}

func (sveltePack) ContextProviders() []inspectctx.Provider {
	return nil
}

func (sveltePack) Analyzers() []core.Analyzer {
	return []core.Analyzer{sveltelint.New()}
}
