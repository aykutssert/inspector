package javascript

import (
	inspectctx "github.com/aykutssert/inspector/internal/context"
	"github.com/aykutssert/inspector/internal/core"
	"github.com/aykutssert/inspector/internal/lang/javascript/analyzers/reacthint"
)

type reactPack struct{}

func React() core.Pack { return reactPack{} }

func (reactPack) ID() string { return "react" }

func (reactPack) Detect(ctx core.ProjectContext) core.Detection {
	if core.ContainsLanguage(ctx, "javascript") {
		return core.Detection{Matched: true, Reason: "React-capable JavaScript/TypeScript scan enabled"}
	}
	return core.Detection{}
}

func (reactPack) Coverage() core.Coverage {
	return core.Coverage{Security: false, Hints: true, Context: false}
}

func (reactPack) Toolchains() []core.Toolchain {
	return nil
}

func (reactPack) ScanAdapters(string) []core.LanguageAdapter {
	return nil
}

func (reactPack) ContextAdapters() []core.LanguageAdapter {
	return nil
}

func (reactPack) ContextProviders() []inspectctx.Provider {
	return nil
}

func (reactPack) Analyzers() []core.Analyzer {
	return []core.Analyzer{reacthint.New()}
}
