package javascript

import (
	inspectctx "github.com/aykutssert/scout/internal/context"
	"github.com/aykutssert/scout/internal/core"
	"github.com/aykutssert/scout/internal/lang/javascript/analyzers/tailwindlint"
)

type tailwindPack struct{}

func Tailwind() core.Pack { return tailwindPack{} }

func (tailwindPack) ID() string { return "tailwind" }

func (tailwindPack) Detect(ctx core.ProjectContext) core.Detection {
	if core.ContainsLanguage(ctx, "javascript") {
		return core.Detection{Matched: true, Reason: "Tailwind class analysis available for JavaScript/TypeScript projects"}
	}
	return core.Detection{}
}

func (tailwindPack) Coverage() core.Coverage {
	return core.Coverage{Security: false, Hints: true, Context: false}
}

func (tailwindPack) Toolchains() []core.Toolchain {
	return []core.Toolchain{{Name: "tailwind", Path: "_toolchains/tailwind"}}
}

func (tailwindPack) ScanAdapters(string) []core.LanguageAdapter {
	return nil
}

func (tailwindPack) ContextAdapters() []core.LanguageAdapter {
	return nil
}

func (tailwindPack) ContextParsers() []inspectctx.FileParser {
	return nil
}

func (tailwindPack) ContextProviders() []inspectctx.Provider {
	return nil
}

func (tailwindPack) Analyzers() []core.Analyzer {
	return []core.Analyzer{tailwindlint.New()}
}
