package packs

import (
	"github.com/aykutssert/inspector/internal/analyzers/gitlog"
	"github.com/aykutssert/inspector/internal/analyzers/osv"
	"github.com/aykutssert/inspector/internal/analyzers/semgrep"
	"github.com/aykutssert/inspector/internal/core"
)

// Pack describes one product capability slice: a language, framework, or domain
// area that contributes adapters, analyzers, or context support.
type Pack interface {
	ID() string
	Detect(ctx core.ProjectContext) Detection
	Coverage() Coverage
	Toolchains() []Toolchain
	ScanAdapters(rulesDir string) []core.LanguageAdapter
	ContextAdapters() []core.LanguageAdapter
	Analyzers() []core.Analyzer
}

type Detection struct {
	Matched bool
	Reason  string
}

type Coverage struct {
	Security bool
	Hints    bool
	Context  bool
}

type Toolchain struct {
	Name string
	Path string
}

type Registry struct {
	packs []Pack
}

func New(packs ...Pack) *Registry {
	return &Registry{packs: packs}
}

func Default() *Registry {
	return New(
		JavaScript(),
		React(),
		Svelte(),
		TypeScript(),
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

// Analyzers returns the analyzers to run for the scanned project. The global
// scanners (semgrep, osv, gitlog) always run: semgrep is a language-agnostic
// SAST, osv reads dependency manifests, gitlog inspects history. A pack's
// analyzers are included only when the pack detects its language in ctx — so a
// repo that lacks a pack's language never demands that pack's toolchain.
// Without this gate the fail-closed orchestrator would flag a missing JS linter
// on a pure Go/Python repo, blocking a scan that has nothing to do with JS.
func (r *Registry) Analyzers(ctx core.ProjectContext, customRuleDirs []string) []core.Analyzer {
	out := []core.Analyzer{semgrep.New("", customRuleDirs...)}
	for _, p := range r.packs {
		if p.Detect(ctx).Matched {
			out = append(out, p.Analyzers()...)
		}
	}
	return append(out, osv.New(), gitlog.New())
}

func (r *Registry) Toolchains() []Toolchain {
	var out []Toolchain
	for _, p := range r.packs {
		out = append(out, p.Toolchains()...)
	}
	return out
}
