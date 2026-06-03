package packs

import (
	"github.com/aykutssert/inspector/internal/analyzers/reacthint"
	"github.com/aykutssert/inspector/internal/core"
)

type reactPack struct{}

func React() Pack { return reactPack{} }

func (reactPack) ID() string { return "react" }

func (reactPack) Detect(ctx core.ProjectContext) Detection {
	if containsLanguage(ctx, "javascript") {
		return Detection{Matched: true, Reason: "React-capable JavaScript/TypeScript scan enabled"}
	}
	return Detection{}
}

func (reactPack) Coverage() Coverage {
	return Coverage{Security: false, Hints: true, Context: false}
}

func (reactPack) Toolchains() []Toolchain {
	return nil
}

func (reactPack) ScanAdapters(string) []core.LanguageAdapter {
	return nil
}

func (reactPack) ContextAdapters() []core.LanguageAdapter {
	return nil
}

func (reactPack) Analyzers() []core.Analyzer {
	return []core.Analyzer{reacthint.New()}
}
