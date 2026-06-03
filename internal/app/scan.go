package app

import (
	"path/filepath"

	"github.com/aykutssert/inspector/internal/core"
	"github.com/aykutssert/inspector/internal/lang"
	"github.com/aykutssert/inspector/internal/packs"
	"github.com/aykutssert/inspector/internal/scan"
)

type ScanOptions struct {
	Root     string
	DiffOnly bool
	RulesDir string
}

func Scan(opts ScanOptions, registry *packs.Registry) (core.Report, error) {
	if registry == nil {
		registry = packs.Default()
	}
	root := opts.Root
	if root == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return core.Report{}, err
	}

	rulesDir := opts.RulesDir
	if rulesDir == "" {
		rulesDir = "rules"
	}
	adapters := registry.ScanAdapters(rulesDir)
	files, err := scan.Discover(absRoot, opts.DiffOnly, adapters)
	if err != nil {
		return core.Report{}, err
	}

	var changed []string
	if opts.DiffOnly {
		changed, err = scan.Changed(absRoot)
		if err != nil {
			return core.Report{}, err
		}
	}

	ctx := core.ProjectContext{
		Root:      absRoot,
		DiffOnly:  opts.DiffOnly,
		Files:     files,
		Languages: lang.NewRegistry(adapters...).Detect(files),
		Changed:   changed,
	}

	orch := core.New(registry.Analyzers(customRuleDirs(adapters))...)
	return orch.Run(ctx), nil
}

func customRuleDirs(adapters []core.LanguageAdapter) []string {
	var out []string
	for _, ad := range adapters {
		for _, d := range ad.Rules() {
			if abs, err := filepath.Abs(d); err == nil {
				out = append(out, abs)
			}
		}
	}
	return out
}
