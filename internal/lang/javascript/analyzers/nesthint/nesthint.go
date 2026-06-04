// Package nesthint performs NestJS-specific cross-file checks that Semgrep
// cannot model reliably. The first check builds a small @Module graph and
// verifies class-token constructor injections against providers exported by the
// owning module and its imports.
package nesthint

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/aykutssert/inspector/internal/core"
)

type Analyzer struct{}

func New() *Analyzer { return &Analyzer{} }

var _ core.Analyzer = (*Analyzer)(nil)

func (a *Analyzer) Name() string { return "nest-hint" }

func (a *Analyzer) Available() bool { return true }

type symbolRef struct {
	File string
	Name string
}

func (r symbolRef) key() string {
	if r.File == "" || r.Name == "" {
		return ""
	}
	return r.File + "#" + r.Name
}

type importBinding struct {
	Source   string
	Imported string
}

type injectionDep struct {
	Name string
	Ref  symbolRef
	Line int
}

type classInfo struct {
	Ref         symbolRef
	Line        int
	Decorators  map[string]bool
	Constructor []injectionDep
}

type moduleInfo struct {
	Ref             symbolRef
	File            string
	Line            int
	IsGlobal        bool
	Imports         map[string]bool
	Controllers     map[string]bool
	Providers       map[string]bool
	Exports         map[string]bool
	UnknownImports  bool
	UnknownProvider bool
	UnknownExports  bool
}

type fileInfo struct {
	Path     string
	Resolver *importResolver
	Imports  map[string]importBinding
	Classes  map[string]*classInfo
	Modules  []*moduleInfo
}

type project struct {
	Root    string
	Files   map[string]*fileInfo
	Classes map[string]*classInfo
	Modules map[string]*moduleInfo
}

var jsExt = map[string]bool{
	".js": true, ".jsx": true, ".ts": true, ".tsx": true,
	".mjs": true, ".cjs": true, ".mts": true, ".cts": true,
}

func (a *Analyzer) Scan(ctx core.ProjectContext) ([]core.Finding, error) {
	files := jstsFiles(ctx.Files)
	if len(files) == 0 {
		return nil, nil
	}
	p := &project{
		Root:    ctx.Root,
		Files:   map[string]*fileInfo{},
		Classes: map[string]*classInfo{},
		Modules: map[string]*moduleInfo{},
	}
	resolver := loadImportResolver(ctx.Root)
	for _, rel := range files {
		fi, err := parseFile(ctx.Root, rel, resolver)
		if err != nil {
			continue
		}
		p.Files[rel] = fi
		for _, c := range fi.Classes {
			p.Classes[c.Ref.key()] = c
		}
		for _, m := range fi.Modules {
			p.Modules[m.Ref.key()] = m
		}
	}
	if len(p.Modules) == 0 {
		return nil, nil
	}
	return p.findMissingProviders(), nil
}

func jstsFiles(files []string) []string {
	var out []string
	for _, f := range files {
		if jsExt[strings.ToLower(filepath.Ext(f))] {
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return out
}
