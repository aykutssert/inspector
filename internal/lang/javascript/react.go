package javascript

import (
	inspectctx "github.com/aykutssert/inspector/internal/context"
	"github.com/aykutssert/inspector/internal/core"
	"github.com/aykutssert/inspector/internal/lang/javascript/analyzers/reacthint"
	"github.com/aykutssert/inspector/internal/lang/javascript/jsproject"
)

type reactPack struct{}

func React() core.Pack { return reactPack{} }

func (reactPack) ID() string { return "react" }

// Detect gates the React hint pack on a real React/Next.js signal — a JSX/TSX
// file or a react/next dependency — using the same definition oxlint uses to
// enable its React plugins. Without this gate, the pack ran on every JS/TS
// project and surfaced React-shaped hints on plain Node/backend code.
func (reactPack) Detect(ctx core.ProjectContext) core.Detection {
	if jsproject.IsReact(ctx) || jsproject.IsNext(ctx) {
		return core.Detection{Matched: true, Reason: "React/Next.js project detected (JSX/TSX file or react dependency)"}
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
