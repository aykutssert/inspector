package nesthint

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

func collectImport(fi *fileInfo, n *sitter.Node, src []byte) {
	source := unquote(lastChildOfTypeText(n, "string", src))
	if source == "" {
		return
	}
	walk(n, func(c *sitter.Node) {
		if c.Type() != "import_specifier" {
			return
		}
		var ids []string
		for i := 0; i < int(c.NamedChildCount()); i++ {
			ch := c.NamedChild(i)
			if ch.Type() == "identifier" {
				ids = append(ids, text(ch, src))
			}
		}
		if len(ids) == 1 {
			fi.Imports[ids[0]] = importBinding{Source: source, Imported: ids[0]}
		}
		if len(ids) >= 2 {
			fi.Imports[ids[len(ids)-1]] = importBinding{Source: source, Imported: ids[0]}
		}
	})
}

func resolveLocal(rel string, fi *fileInfo, name string) symbolRef {
	if name == "" {
		return symbolRef{}
	}
	if _, ok := fi.Classes[name]; ok {
		return symbolRef{File: rel, Name: name}
	}
	if b, ok := fi.Imports[name]; ok {
		target := resolveImport(fi.Resolver, fi.Path, b.Source)
		if target != "" {
			imported := b.Imported
			if imported == "" {
				imported = name
			}
			if ref := resolveExported(fi.Resolver, target, imported, map[string]bool{}); ref.key() != "" {
				return ref
			}
			return symbolRef{File: target, Name: imported}
		}
	}
	return symbolRef{File: rel, Name: name}
}

var resolveExts = []string{"", ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs", ".mts", ".cts"}

type importResolver struct {
	Root    string
	BaseURL string
	Aliases []pathAlias
}

type pathAlias struct {
	Pattern string
	Targets []string
}

type tsconfigFile struct {
	CompilerOptions struct {
		BaseURL string              `json:"baseUrl"`
		Paths   map[string][]string `json:"paths"`
	} `json:"compilerOptions"`
}

func loadImportResolver(root string) *importResolver {
	r := &importResolver{Root: root}
	for _, name := range []string{"tsconfig.base.json", "tsconfig.json"} {
		cfg := readTSConfig(filepath.Join(root, name))
		if cfg == nil {
			continue
		}
		if cfg.CompilerOptions.BaseURL != "" {
			r.BaseURL = filepath.Clean(cfg.CompilerOptions.BaseURL)
		}
		if len(cfg.CompilerOptions.Paths) > 0 {
			r.Aliases = mergeAliases(r.Aliases, cfg.CompilerOptions.Paths)
		}
	}
	if r.BaseURL == "" {
		r.BaseURL = "."
	}
	return r
}

func readTSConfig(path string) *tsconfigFile {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	cleaned := stripJSONComments(raw)
	cleaned = stripTrailingCommas(cleaned)
	var cfg tsconfigFile
	if err := json.Unmarshal(cleaned, &cfg); err != nil {
		return nil
	}
	return &cfg
}

func stripJSONComments(src []byte) []byte {
	out := make([]byte, 0, len(src))
	inString := false
	escaped := false
	for i := 0; i < len(src); i++ {
		ch := src[i]
		if inString {
			out = append(out, ch)
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		if ch == '"' {
			inString = true
			out = append(out, ch)
			continue
		}
		if ch == '/' && i+1 < len(src) && src[i+1] == '/' {
			for i < len(src) && src[i] != '\n' {
				i++
			}
			if i < len(src) {
				out = append(out, src[i])
			}
			continue
		}
		if ch == '/' && i+1 < len(src) && src[i+1] == '*' {
			i += 2
			for i+1 < len(src) && !(src[i] == '*' && src[i+1] == '/') {
				i++
			}
			i++
			continue
		}
		out = append(out, ch)
	}
	return out
}

func stripTrailingCommas(src []byte) []byte {
	out := make([]byte, 0, len(src))
	inString := false
	escaped := false
	for i := 0; i < len(src); i++ {
		ch := src[i]
		if inString {
			out = append(out, ch)
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		if ch == '"' {
			inString = true
			out = append(out, ch)
			continue
		}
		if ch == ',' {
			j := i + 1
			for j < len(src) && (src[j] == ' ' || src[j] == '\n' || src[j] == '\r' || src[j] == '\t') {
				j++
			}
			if j < len(src) && (src[j] == '}' || src[j] == ']') {
				continue
			}
		}
		out = append(out, ch)
	}
	return out
}

func mergeAliases(existing []pathAlias, paths map[string][]string) []pathAlias {
	byPattern := map[string]int{}
	for i, a := range existing {
		byPattern[a.Pattern] = i
	}
	keys := make([]string, 0, len(paths))
	for k := range paths {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		targets := paths[k]
		if i, ok := byPattern[k]; ok {
			existing[i].Targets = targets
			continue
		}
		byPattern[k] = len(existing)
		existing = append(existing, pathAlias{Pattern: k, Targets: targets})
	}
	sort.Slice(existing, func(i, j int) bool {
		return aliasSpecificity(existing[i].Pattern) > aliasSpecificity(existing[j].Pattern)
	})
	return existing
}

func aliasSpecificity(pattern string) int {
	return len(strings.ReplaceAll(pattern, "*", ""))
}

func resolveImport(r *importResolver, fromFile, spec string) string {
	if r == nil {
		return ""
	}
	if strings.HasPrefix(spec, ".") {
		base := filepath.Join(filepath.Dir(fromFile), spec)
		return resolveCandidate(r.Root, base)
	}
	if target := resolveAlias(r, spec); target != "" {
		return target
	}
	if r.BaseURL != "." && r.BaseURL != "" {
		return resolveCandidate(r.Root, filepath.Join(r.BaseURL, spec))
	}
	return ""
}

func resolveAlias(r *importResolver, spec string) string {
	for _, alias := range r.Aliases {
		capture, ok := matchAlias(alias.Pattern, spec)
		if !ok {
			continue
		}
		for _, target := range alias.Targets {
			mapped := strings.ReplaceAll(target, "*", capture)
			if resolved := resolveCandidate(r.Root, filepath.Join(r.BaseURL, mapped)); resolved != "" {
				return resolved
			}
		}
	}
	return ""
}

func matchAlias(pattern, spec string) (string, bool) {
	if !strings.Contains(pattern, "*") {
		return "", pattern == spec
	}
	parts := strings.SplitN(pattern, "*", 2)
	prefix, suffix := parts[0], parts[1]
	if !strings.HasPrefix(spec, prefix) || !strings.HasSuffix(spec, suffix) {
		return "", false
	}
	return spec[len(prefix) : len(spec)-len(suffix)], true
}

func resolveCandidate(root, base string) string {
	var candidates []string
	for _, ext := range resolveExts {
		if ext == "" && base == "." {
			continue
		}
		candidates = append(candidates, base+ext)
	}
	for _, ext := range resolveExts[1:] {
		candidates = append(candidates, filepath.Join(base, "index"+ext))
	}
	for _, c := range candidates {
		c = filepath.Clean(c)
		if c == "." || c == "" {
			continue
		}
		if info, err := os.Stat(filepath.Join(root, c)); err == nil && !info.IsDir() {
			return c
		}
	}
	return ""
}

func resolveExported(r *importResolver, rel, name string, seen map[string]bool) symbolRef {
	if rel == "" || name == "" {
		return symbolRef{}
	}
	key := rel + "#" + name
	if seen[key] {
		return symbolRef{}
	}
	seen[key] = true

	abs := filepath.Join(r.Root, rel)
	src, err := os.ReadFile(abs)
	if err != nil || len(src) > maxFileBytes {
		return symbolRef{}
	}
	lang := langForPath(abs)
	parser := sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(lang)
	pctx, cancel := context.WithTimeout(context.Background(), parseTimeout)
	defer cancel()
	tree, err := parser.ParseCtx(pctx, nil, src)
	if err != nil {
		return symbolRef{}
	}
	defer tree.Close()

	direct := false
	walk(tree.RootNode(), func(n *sitter.Node) {
		if direct || n.Type() != "class_declaration" {
			return
		}
		if className(n, src) == name {
			direct = true
		}
	})
	if direct {
		return symbolRef{File: rel, Name: name}
	}

	var starSources []string
	var resolved symbolRef
	walk(tree.RootNode(), func(n *sitter.Node) {
		if resolved.key() != "" || n.Type() != "export_statement" {
			return
		}
		source := unquote(lastChildOfTypeText(n, "string", src))
		if source == "" || !strings.HasPrefix(source, ".") {
			return
		}
		if firstChildOfType(n, "export_clause") == nil {
			starSources = append(starSources, source)
			return
		}
		walk(n, func(spec *sitter.Node) {
			if resolved.key() != "" || spec.Type() != "export_specifier" {
				return
			}
			ids := identifiers(spec, src)
			if len(ids) == 1 && ids[0] == name {
				if target := resolveImport(r, rel, source); target != "" {
					resolved = resolveExported(r, target, name, seen)
				}
				return
			}
			if len(ids) >= 2 && ids[len(ids)-1] == name {
				if target := resolveImport(r, rel, source); target != "" {
					resolved = resolveExported(r, target, ids[0], seen)
				}
			}
		})
	})
	if resolved.key() != "" {
		return resolved
	}
	for _, source := range starSources {
		target := resolveImport(r, rel, source)
		if target == "" {
			continue
		}
		if ref := resolveExported(r, target, name, seen); ref.key() != "" {
			return ref
		}
	}
	return symbolRef{}
}

func identifiers(n *sitter.Node, src []byte) []string {
	var out []string
	for i := 0; i < int(n.NamedChildCount()); i++ {
		ch := n.NamedChild(i)
		if ch.Type() == "identifier" || ch.Type() == "type_identifier" {
			out = append(out, text(ch, src))
		}
	}
	return out
}
