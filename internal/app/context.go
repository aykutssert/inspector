package app

import (
	"errors"
	"path/filepath"

	inspectctx "github.com/aykutssert/scout/internal/context"
	"github.com/aykutssert/scout/internal/registry"
	"github.com/aykutssert/scout/internal/scan"
)

type ContextOptions struct {
	Root   string
	Target string
}

func Context(opts ContextOptions, reg *registry.Registry) (inspectctx.Context, error) {
	if reg == nil {
		reg = registry.Default()
	}
	root := opts.Root
	if root == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return inspectctx.Context{}, err
	}
	files, err := scan.Discover(absRoot, false, reg.ContextAdapters())
	if err != nil {
		return inspectctx.Context{}, err
	}
	providers := reg.ContextProviders()
	if len(providers) == 0 {
		return inspectctx.Context{}, errors.New("no context providers registered")
	}
	return providers[0].GetContext(absRoot, files, opts.Target)
}

type MapOptions struct {
	Root string
}

// Map builds a RepoMap from all registered language parsers.
func Map(opts MapOptions, reg *registry.Registry) (inspectctx.RepoMap, error) {
	if reg == nil {
		reg = registry.Default()
	}
	root := opts.Root
	if root == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return inspectctx.RepoMap{}, err
	}
	files, err := scan.Discover(absRoot, false, reg.ContextAdapters())
	if err != nil {
		return inspectctx.RepoMap{}, err
	}
	parsers := reg.ContextParsers()
	if len(parsers) == 0 {
		return inspectctx.RepoMap{}, errors.New("no file parsers registered")
	}
	return inspectctx.BuildRepoMap(absRoot, files, inspectctx.NewMultiLangParser(parsers...))
}
