package packs

import (
	"github.com/aykutssert/inspector/internal/analyzers/oxlint"
	"github.com/aykutssert/inspector/internal/analyzers/tailwindlint"
	"github.com/aykutssert/inspector/internal/core"
	"github.com/aykutssert/inspector/internal/lang/javascript"
)

type javascriptPack struct{}

func JavaScript() Pack { return javascriptPack{} }

func (javascriptPack) ID() string { return "javascript" }

func (javascriptPack) Detect(ctx core.ProjectContext) Detection {
	if containsLanguage(ctx, "javascript") {
		return Detection{Matched: true, Reason: "JavaScript/TypeScript source files detected"}
	}
	return Detection{}
}

func (javascriptPack) Coverage() Coverage {
	return Coverage{Security: true, Hints: true, Context: true}
}

func (javascriptPack) Toolchains() []Toolchain {
	return nil
}

func (javascriptPack) ScanAdapters(rulesDir string) []core.LanguageAdapter {
	return []core.LanguageAdapter{javascript.New(rulesDir)}
}

func (javascriptPack) ContextAdapters() []core.LanguageAdapter {
	return []core.LanguageAdapter{javascript.New("")}
}

func (javascriptPack) Analyzers() []core.Analyzer {
	return []core.Analyzer{oxlint.New(), tailwindlint.New()}
}
