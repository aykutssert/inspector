package javascript

import (
	inspectctx "github.com/aykutssert/scout/internal/context"
	"github.com/aykutssert/scout/internal/core"
	"github.com/aykutssert/scout/internal/lang/javascript/analyzers/tsc"
	"github.com/aykutssert/scout/internal/lang/javascript/analyzers/tsconfig"
	"github.com/aykutssert/scout/internal/lang/javascript/analyzers/tseslint"
)

type typescriptPack struct{}

func TypeScript() core.Pack { return typescriptPack{} }

func (typescriptPack) ID() string { return "typescript" }

func (typescriptPack) Detect(ctx core.ProjectContext) core.Detection {
	if core.ContainsLanguage(ctx, "javascript") {
		return core.Detection{Matched: true, Reason: "TypeScript extensions are handled by the JavaScript adapter"}
	}
	return core.Detection{}
}

func (typescriptPack) Coverage() core.Coverage {
	return core.Coverage{Security: true, Hints: true, Context: true}
}

func (typescriptPack) Toolchains() []core.Toolchain {
	return []core.Toolchain{{Name: "typescript", Path: "_toolchains/typescript"}}
}

func (typescriptPack) ScanAdapters(string) []core.LanguageAdapter {
	return nil
}

func (typescriptPack) ContextAdapters() []core.LanguageAdapter {
	return nil
}

func (typescriptPack) ContextProviders() []inspectctx.Provider {
	return nil
}

func (typescriptPack) Analyzers() []core.Analyzer {
	return []core.Analyzer{
		tsc.New(),
		tsconfig.New(),
		tseslint.New(),
	}
}
