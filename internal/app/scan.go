package app

import (
	"path/filepath"

	"github.com/aykutssert/scout/internal/core"
	"github.com/aykutssert/scout/internal/lang"
	"github.com/aykutssert/scout/internal/registry"
	"github.com/aykutssert/scout/internal/scan"
)

type ScanOptions struct {
	Root     string
	DiffOnly bool
	RulesDir string
}

func Scan(opts ScanOptions, reg *registry.Registry) (core.Report, error) {
	if reg == nil {
		reg = registry.Default()
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
	adapters := reg.ScanAdapters(rulesDir)
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
		Languages: lang.NewRegistry(adapters...).Detect(files, absRoot),
		Changed:   changed,
	}

	orch := core.New(reg.Analyzers(ctx, customRuleDirs(adapters))...)
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
