package javascript

import (
	inspectctx "github.com/aykutssert/scout/internal/context"
	"github.com/aykutssert/scout/internal/core"
	"github.com/aykutssert/scout/internal/lang/javascript/analyzers/rnhint"
	"github.com/aykutssert/scout/internal/lang/javascript/jsproject"
)

type reactNativePack struct{}

func ReactNative() core.Pack { return reactNativePack{} }

func (reactNativePack) ID() string { return "react-native" }

func (reactNativePack) Detect(ctx core.ProjectContext) core.Detection {
	if jsproject.IsReactNative(ctx) {
		return core.Detection{Matched: true, Reason: "react-native dependency detected"}
	}
	return core.Detection{}
}

func (reactNativePack) Coverage() core.Coverage {
	return core.Coverage{Security: false, Hints: true, Context: false}
}

func (reactNativePack) Toolchains() []core.Toolchain { return nil }

func (reactNativePack) ScanAdapters(string) []core.LanguageAdapter { return nil }

func (reactNativePack) ContextAdapters() []core.LanguageAdapter { return nil }

func (reactNativePack) ContextParsers() []inspectctx.FileParser { return nil }

func (reactNativePack) ContextProviders() []inspectctx.Provider { return nil }

func (reactNativePack) Analyzers() []core.Analyzer {
	return []core.Analyzer{rnhint.New()}
}
