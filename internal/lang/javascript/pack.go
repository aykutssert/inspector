package javascript

import (
	inspectctx "github.com/aykutssert/inspector/internal/context"
	"github.com/aykutssert/inspector/internal/core"
	"github.com/aykutssert/inspector/internal/lang/javascript/analyzers/archhint"
	"github.com/aykutssert/inspector/internal/lang/javascript/analyzers/importcycle"
	"github.com/aykutssert/inspector/internal/lang/javascript/analyzers/jsquality"
	"github.com/aykutssert/inspector/internal/lang/javascript/analyzers/knip"
	"github.com/aykutssert/inspector/internal/lang/javascript/analyzers/nesthint"
	"github.com/aykutssert/inspector/internal/lang/javascript/analyzers/nexthint"
	"github.com/aykutssert/inspector/internal/lang/javascript/analyzers/oxlint"
	"github.com/aykutssert/inspector/internal/lang/javascript/analyzers/policycoverage"
	"github.com/aykutssert/inspector/internal/lang/javascript/analyzers/validationcoverage"
	jscontext "github.com/aykutssert/inspector/internal/lang/javascript/context"
)

type javascriptPack struct{}

func JavaScript() core.Pack { return javascriptPack{} }

func (javascriptPack) ID() string { return "javascript" }

func (javascriptPack) Detect(ctx core.ProjectContext) core.Detection {
	if core.ContainsLanguage(ctx, "javascript") {
		return core.Detection{Matched: true, Reason: "JavaScript/TypeScript source files detected"}
	}
	return core.Detection{}
}

func (javascriptPack) Coverage() core.Coverage {
	return core.Coverage{Security: true, Hints: true, Context: true}
}

func (javascriptPack) Toolchains() []core.Toolchain {
	return []core.Toolchain{{Name: "knip", Path: "_toolchains/knip"}}
}

func (javascriptPack) ScanAdapters(rulesDir string) []core.LanguageAdapter {
	return []core.LanguageAdapter{New(rulesDir)}
}

func (javascriptPack) ContextAdapters() []core.LanguageAdapter {
	return []core.LanguageAdapter{New("")}
}

func (javascriptPack) ContextProviders() []inspectctx.Provider {
	return []inspectctx.Provider{jscontext.NewProvider()}
}

func (javascriptPack) Analyzers() []core.Analyzer {
	return []core.Analyzer{oxlint.New(), importcycle.New(), knip.New(), nesthint.New(), nexthint.New(), archhint.New(), jsquality.New(), policycoverage.New(), validationcoverage.New()}
}
