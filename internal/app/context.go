package app

import (
	"errors"
	"path/filepath"

	inspectctx "github.com/aykutssert/inspector/internal/context"
	"github.com/aykutssert/inspector/internal/packs"
	"github.com/aykutssert/inspector/internal/scan"
)

type ContextOptions struct {
	Root   string
	Target string
}

func Context(opts ContextOptions, registry *packs.Registry) (inspectctx.Context, error) {
	if registry == nil {
		registry = packs.Default()
	}
	root := opts.Root
	if root == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return inspectctx.Context{}, err
	}
	files, err := scan.Discover(absRoot, false, registry.ContextAdapters())
	if err != nil {
		return inspectctx.Context{}, err
	}
	providers := registry.ContextProviders()
	if len(providers) == 0 {
		return inspectctx.Context{}, errors.New("no context providers registered")
	}
	return providers[0].GetContext(absRoot, files, opts.Target)
}
