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
