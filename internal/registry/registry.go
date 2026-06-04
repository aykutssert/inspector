// Package registry assembles the concrete packs into the scanner's default set
// and aggregates their adapters, analyzers, and context providers. It is the one
// place that knows every language pack, so adding a language means adding its
// constructor to Default below.
package registry

import (
	"strings"

	inspectctx "github.com/aykutssert/inspector/internal/context"
	"github.com/aykutssert/inspector/internal/core"
	"github.com/aykutssert/inspector/internal/global/gitlog"
	"github.com/aykutssert/inspector/internal/global/osv"
	"github.com/aykutssert/inspector/internal/global/semgrep"
	"github.com/aykutssert/inspector/internal/lang/javascript"
	"github.com/aykutssert/inspector/internal/lang/javascript/jsproject"
	"github.com/aykutssert/inspector/internal/lang/svelte"
)

type Registry struct {
	packs []core.Pack
}

func New(p ...core.Pack) *Registry {
	return &Registry{packs: p}
}

func Default() *Registry {
	return New(
		javascript.JavaScript(),
		javascript.React(),
		svelte.Svelte(),
		javascript.TypeScript(),
		javascript.Tailwind(),
	)
}

func (r *Registry) ScanAdapters(rulesDir string) []core.LanguageAdapter {
	var out []core.LanguageAdapter
	for _, p := range r.packs {
		out = append(out, p.ScanAdapters(rulesDir)...)
	}
	return out
}

func (r *Registry) ContextAdapters() []core.LanguageAdapter {
	var out []core.LanguageAdapter
	for _, p := range r.packs {
		out = append(out, p.ContextAdapters()...)
	}
	return out
}

func (r *Registry) ContextProviders() []inspectctx.Provider {
	var out []inspectctx.Provider
	for _, p := range r.packs {
		out = append(out, p.ContextProviders()...)
	}
	return out
}

// Analyzers returns the analyzers to run for the scanned project. The global
// scanners (semgrep, osv, gitlog) always run: semgrep is a language-agnostic
// SAST, osv reads dependency manifests, gitlog inspects history. A pack's
// analyzers are included only when the pack detects its language in ctx — so a
// repo that lacks a pack's language never demands that pack's toolchain.
// Without this gate the fail-closed orchestrator would flag a missing JS linter
// on a pure Go/Python repo, blocking a scan that has nothing to do with JS.
func (r *Registry) Analyzers(ctx core.ProjectContext, customRuleDirs []string) []core.Analyzer {
	out := []core.Analyzer{semgrep.New("", customRuleDirs...).WithDrop(dropInapplicableRules(ctx))}
	for _, p := range r.packs {
		if p.Detect(ctx).Matched {
			out = append(out, p.Analyzers()...)
		}
	}
	return append(out, osv.New(), gitlog.New())
}

// dropInapplicableRules suppresses custom semgrep rules that fire on the wrong
// project shape. semgrep loads every rule in the custom dirs, so framework-gated
// rules (e.g. Bun.* APIs) otherwise fire on plain Node code. Gating here keeps
// jsproject the single source of truth for the signal. Returns nil when nothing
// needs dropping, so the common path adds no overhead.
func dropInapplicableRules(ctx core.ProjectContext) func(core.Finding) bool {
	bun := jsproject.IsBun(ctx)
	vite := jsproject.IsVite(ctx)
	if bun && vite {
		return nil
	}
	return func(f core.Finding) bool {
		if !bun && strings.HasPrefix(f.RuleID, "bun.") {
			return true
		}
		if !vite && strings.HasPrefix(f.RuleID, "vite.") {
			return true
		}
		return false
	}
}

func (r *Registry) Toolchains() []core.Toolchain {
	var out []core.Toolchain
	for _, p := range r.packs {
		out = append(out, p.Toolchains()...)
	}
	return out
}
