package jscontext

import (
	"path/filepath"

	"github.com/aykutssert/scout/internal/context"
)

// JSParser implements context.FileParser for JavaScript/TypeScript.
var _ context.FileParser = (*JSParser)(nil)

type JSParser struct{}

func (JSParser) Parse(root, path string) (*context.FileParse, error) {
	fp, err := ParseJS(filepath.Join(root, path))
	if err != nil {
		return nil, err
	}
	imports := make([]context.Import, len(fp.Imports))
	for i, im := range fp.Imports {
		imports[i] = context.Import{Source: im.Source, Line: im.Line}
	}
	defs := make([]context.Def, len(fp.Defs))
	for i, d := range fp.Defs {
		defs[i] = context.Def{Name: d.Name, Kind: d.Kind, Line: d.Line, EndLine: d.EndLine, Exported: d.Exported}
	}
	calls := make([]context.Call, len(fp.Calls))
	for i, c := range fp.Calls {
		calls[i] = context.Call{Name: c.Name, Recv: c.Recv, Line: c.Line}
	}
	return &context.FileParse{
		Path:     path,
		Imports:  imports,
		Defs:     defs,
		Calls:    calls,
		HasError: fp.HasError,
	}, nil
}
