package svelte

import (
	inspectctx "github.com/aykutssert/inspector/internal/context"
	"github.com/aykutssert/inspector/internal/core"
	"github.com/aykutssert/inspector/internal/lang/svelte/analyzers/sveltehint"
	"github.com/aykutssert/inspector/internal/lang/svelte/analyzers/sveltelint"
)

type sveltePack struct{}

func Svelte() core.Pack { return sveltePack{} }

func (sveltePack) ID() string { return "svelte" }

func (sveltePack) Detect(ctx core.ProjectContext) core.Detection {
	if core.ContainsLanguage(ctx, "svelte") {
		return core.Detection{Matched: true, Reason: "Svelte source files detected"}
	}
	return core.Detection{}
}

func (sveltePack) Coverage() core.Coverage {
	return core.Coverage{Security: true, Hints: true, Context: false}
}

func (sveltePack) Toolchains() []core.Toolchain {
	return []core.Toolchain{{Name: "svelte", Path: "_toolchains/svelte"}}
}

func (sveltePack) ScanAdapters(string) []core.LanguageAdapter {
	return []core.LanguageAdapter{New()}
}

func (sveltePack) ContextAdapters() []core.LanguageAdapter {
	return nil
}

func (sveltePack) ContextProviders() []inspectctx.Provider {
	return nil
}

func (sveltePack) Analyzers() []core.Analyzer {
	return []core.Analyzer{sveltelint.New(), sveltehint.New()}
}
