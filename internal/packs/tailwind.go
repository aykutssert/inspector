package packs

import (
	"github.com/aykutssert/inspector/internal/analyzers/tailwindlint"
	inspectctx "github.com/aykutssert/inspector/internal/context"
	"github.com/aykutssert/inspector/internal/core"
)

type tailwindPack struct{}

func Tailwind() Pack { return tailwindPack{} }

func (tailwindPack) ID() string { return "tailwind" }

func (tailwindPack) Detect(ctx core.ProjectContext) Detection {
	if containsLanguage(ctx, "javascript") {
		return Detection{Matched: true, Reason: "Tailwind class analysis available for JavaScript/TypeScript projects"}
	}
	return Detection{}
}

func (tailwindPack) Coverage() Coverage {
	return Coverage{Security: false, Hints: true, Context: false}
}

func (tailwindPack) Toolchains() []Toolchain {
	return []Toolchain{{Name: "tailwind", Path: "_toolchains/tailwind"}}
}

func (tailwindPack) ScanAdapters(string) []core.LanguageAdapter {
	return nil
}

func (tailwindPack) ContextAdapters() []core.LanguageAdapter {
	return nil
}

func (tailwindPack) ContextProviders() []inspectctx.Provider {
	return nil
}

func (tailwindPack) Analyzers() []core.Analyzer {
	return []core.Analyzer{tailwindlint.New()}
}
