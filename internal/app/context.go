package app

import (
	"path/filepath"

	jscontext "github.com/aykutssert/inspector/internal/context/providers/javascript"
	"github.com/aykutssert/inspector/internal/packs"
	"github.com/aykutssert/inspector/internal/scan"
)

type ContextOptions struct {
	Root   string
	Target string
}

func Context(opts ContextOptions, registry *packs.Registry) (jscontext.Context, error) {
	if registry == nil {
		registry = packs.Default()
	}
	root := opts.Root
	if root == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return jscontext.Context{}, err
	}
	files, err := scan.Discover(absRoot, false, registry.ContextAdapters())
	if err != nil {
		return jscontext.Context{}, err
	}
	g := jscontext.Build(absRoot, files)
	return g.GetContext(opts.Target), nil
}
