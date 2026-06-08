package reacthint

import (
	"path/filepath"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/aykutssert/scout/internal/core"
)

// Analyzer finds React structural and design smells that lint rules miss —
// anti-patterns that are usually-but-not-always wrong, plus consistency and
// AI-slop signals. Every finding is a hint: the agent confirms it against
// context rather than treating it as a defect. This is the layer on top of
// oxlint, not a replacement for it.
type Analyzer struct{}

func New() *Analyzer { return &Analyzer{} }

var _ core.Analyzer = (*Analyzer)(nil)

func (a *Analyzer) Name() string { return "react-hint" }

// Available is always true: this analyzer is pure Go (tree-sitter), no external
// binary to install.
func (a *Analyzer) Available() bool { return true }

func (a *Analyzer) Scan(ctx core.ProjectContext) ([]core.Finding, error) {
	files := reactFiles(ctx.Files)
	resolver := loadReactImportResolver(ctx.Root)
	infos := collectReactFileInfos(ctx.Root, files, resolver)
	var findings []core.Finding
	for _, rel := range files {
		// A parse failure here is not an analyzer failure — semgrep already
		// surfaces syntax errors. Skip the file and keep scanning.
		fs, err := scanFileWithExternalMemoized(filepath.Join(ctx.Root, rel), rel, externalMemoizedComponents(rel, infos))
		if err != nil {
			continue
		}
		findings = append(findings, fs...)
	}
	return findings, nil
}

// detector inspects a parsed file and returns any hints it finds.
type detector func(root *sitter.Node, lang *sitter.Language, src []byte, file string) []core.Finding
type jsxDetector func(root *sitter.Node, lang *sitter.Language, src []byte, file string, externalMemoized map[string]bool) []core.Finding

// detectors run on every JS/TS file (no JSX nodes required).
var detectors = []detector{
	detectDerivedState,
	detectSetStateInEffectNoDeps,
	detectPreferUseReducer,
	detectGodComponent,
	detectComponentSplitting,
	detectUseEffectFetchSuggestQuery,
	detectUseEffectMissingCleanup,
	detectStableEmptyFallback,
	// effect anti-patterns (#78, #85, #86, #91, #92)
	detectInitializeState,
	detectMutableInDeps,
	detectPreferUseSyncExternalStore,
	detectCascadingSetState,
	detectSelfUpdatingEffect,
	// effect anti-patterns continued (#77, #82, #87)
	detectAdjustStateOnPropChange,
	detectEffectEventInDeps,
	detectPassDataToParent,
	// render hints (#49, #53, #76)
	detectHydrationNoFlicker,
	detectTransitionsScroll,
	detectEventHandlerRefs,
}


// jsxDetectors need JSX grammar (text/elements) and only run on js/jsx/tsx.
var jsxDetectors = []jsxDetector{
	detectMemoizedChildUnstableProp,
	detectEmDashInJSX,
	detectRenderTimeAllocation,
	detectCallbackPropContract,
	detectDeepPropDrilling,
	detectNoRenderInRender,
}

func hint(rule, cat string, sev core.Severity, file string, line int, msg, fix string) core.Finding {
	return core.Finding{
		Analyzer:   "react-hint",
		RuleID:     rule,
		Severity:   sev,
		Level:      sev.String(),
		Category:   cat,
		Confidence: core.ConfidenceHint,
		File:       file,
		Line:       line,
		Message:    msg,
		Fix:        fix,
	}
}
