package packs

import (
	"github.com/aykutssert/inspector/internal/analyzers/tsc"
	"github.com/aykutssert/inspector/internal/analyzers/tseslint"
	"github.com/aykutssert/inspector/internal/core"
)

type typescriptPack struct{}

func TypeScript() Pack { return typescriptPack{} }

func (typescriptPack) ID() string { return "typescript" }

func (typescriptPack) Detect(ctx core.ProjectContext) Detection {
	if containsLanguage(ctx, "javascript") {
		return Detection{Matched: true, Reason: "TypeScript extensions are handled by the JavaScript adapter"}
	}
	return Detection{}
}

func (typescriptPack) Coverage() Coverage {
	return Coverage{Security: true, Hints: true, Context: true}
}

func (typescriptPack) Toolchains() []Toolchain {
	return []Toolchain{{Name: "typescript", Path: "_toolchains/typescript"}}
}

func (typescriptPack) ScanAdapters(string) []core.LanguageAdapter {
	return nil
}

func (typescriptPack) ContextAdapters() []core.LanguageAdapter {
	return nil
}

func (typescriptPack) Analyzers() []core.Analyzer {
	return []core.Analyzer{
		tsc.New(),
		tseslint.New(),
	}
}
