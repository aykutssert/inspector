package reacthint

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

func collectReactFileInfos(root string, files []string, resolver *reactImportResolver) map[string]*reactFileInfo {
	infos := map[string]*reactFileInfo{}
	for _, rel := range files {
		info, err := parseReactFileInfo(filepath.Join(root, rel), rel, resolver)
		if err != nil {
			continue
		}
		infos[rel] = info
	}
	return infos
}

type reactFileInfo struct {
	Memoized        map[string]bool
	DefaultMemoized bool
	Imports         map[string]reactImportBinding
	ReExports       map[string][]reactImportBinding
	ExportStars     []string
}

type reactImportBinding struct {
	Source   string
	Imported string
	Target   string
}

func parseReactFileInfo(abs, rel string, resolver *reactImportResolver) (*reactFileInfo, error) {
	if info, err := os.Stat(abs); err == nil && info.Size() > maxFileBytes {
		return nil, err
	}
	src, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	if len(src) > maxFileBytes {
		return nil, nil
	}
	lang := langForPath(abs)
	parser := sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(lang)
	pctx, cancel := context.WithTimeout(context.Background(), parseTimeout)
	defer cancel()
	tree, err := parser.ParseCtx(pctx, nil, src)
	if err != nil {
		return nil, err
	}
	defer tree.Close()
	info := &reactFileInfo{
		Memoized:  memoizedComponents(tree.RootNode(), src, nil),
		Imports:   map[string]reactImportBinding{},
		ReExports: map[string][]reactImportBinding{},
	}
	info.DefaultMemoized = defaultMemoizedExport(tree.RootNode(), src)
	walkReact(tree.RootNode(), func(node *sitter.Node) {
		switch node.Type() {
		case "import_statement":
			collectReactImport(info, resolver, rel, node, src)
		case "export_statement":
			collectReactReExport(info, resolver, rel, node, src)
		}
	})
	return info, nil
}

func collectReactImport(info *reactFileInfo, resolver *reactImportResolver, rel string, node *sitter.Node, src []byte) {
	source := unquoteReact(lastChildText(node, "string", src))
	target := resolveReactImport(resolver, rel, source)
	if target == "" {
		return
	}
	if defaultName := defaultImportName(node, src); defaultName != "" {
		info.Imports[defaultName] = reactImportBinding{Source: source, Imported: "default", Target: target}
	}
	walkReact(node, func(spec *sitter.Node) {
		if spec.Type() != "import_specifier" {
			return
		}
		ids := directIdentifiers(spec, src)
		if len(ids) == 1 {
			info.Imports[ids[0]] = reactImportBinding{Source: source, Imported: ids[0], Target: target}
		}
		if len(ids) >= 2 {
			info.Imports[ids[len(ids)-1]] = reactImportBinding{Source: source, Imported: ids[0], Target: target}
		}
	})
}

func collectReactReExport(info *reactFileInfo, resolver *reactImportResolver, rel string, node *sitter.Node, src []byte) {
	source := unquoteReact(lastChildText(node, "string", src))
	if source == "" {
		return
	}
	target := resolveReactImport(resolver, rel, source)
	if target == "" {
		return
	}
	clause := firstNamedChildOfType(node, "export_clause")
	if clause == nil {
		info.ExportStars = append(info.ExportStars, target)
		return
	}
	walkReact(clause, func(spec *sitter.Node) {
		if spec.Type() != "export_specifier" {
			return
		}
		ids := directIdentifiers(spec, src)
		switch len(ids) {
		case 0:
			return
		case 1:
			info.ReExports[ids[0]] = append(info.ReExports[ids[0]], reactImportBinding{Source: source, Imported: ids[0], Target: target})
		default:
			info.ReExports[ids[len(ids)-1]] = append(info.ReExports[ids[len(ids)-1]], reactImportBinding{Source: source, Imported: ids[0], Target: target})
		}
	})
}

func externalMemoizedComponents(rel string, infos map[string]*reactFileInfo) map[string]bool {
	info := infos[rel]
	if info == nil {
		return nil
	}
	out := map[string]bool{}
	for local, binding := range info.Imports {
		if exportMemoized(infos, binding.Target, binding.Imported, map[string]bool{}) {
			out[local] = true
		}
	}
	return out
}

var reactResolveExts = []string{"", ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs", ".mts", ".cts"}

type reactImportResolver struct {
	Root    string
	BaseURL string
	Aliases []reactPathAlias
}

type reactPathAlias struct {
	Pattern string
	Targets []string
}

type reactTSConfig struct {
	CompilerOptions struct {
		BaseURL string              `json:"baseUrl"`
		Paths   map[string][]string `json:"paths"`
	} `json:"compilerOptions"`
}

func loadReactImportResolver(root string) *reactImportResolver {
	r := &reactImportResolver{Root: root, BaseURL: "."}
	for _, name := range []string{"tsconfig.base.json", "tsconfig.json"} {
		cfg := readReactTSConfig(filepath.Join(root, name))
		if cfg == nil {
			continue
		}
		if cfg.CompilerOptions.BaseURL != "" {
			r.BaseURL = filepath.Clean(cfg.CompilerOptions.BaseURL)
		}
		if len(cfg.CompilerOptions.Paths) > 0 {
			r.Aliases = mergeReactAliases(r.Aliases, cfg.CompilerOptions.Paths)
		}
	}
	return r
}

func readReactTSConfig(path string) *reactTSConfig {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	cleaned := stripReactJSONComments(raw)
	cleaned = stripReactTrailingCommas(cleaned)
	var cfg reactTSConfig
	if err := json.Unmarshal(cleaned, &cfg); err != nil {
		return nil
	}
	return &cfg
}

func mergeReactAliases(existing []reactPathAlias, paths map[string][]string) []reactPathAlias {
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
		if i, ok := byPattern[k]; ok {
			existing[i].Targets = paths[k]
			continue
		}
		existing = append(existing, reactPathAlias{Pattern: k, Targets: paths[k]})
	}
	sort.Slice(existing, func(i, j int) bool {
		return aliasSpecificity(existing[i].Pattern) > aliasSpecificity(existing[j].Pattern)
	})
	return existing
}

func aliasSpecificity(pattern string) int {
	return len(strings.ReplaceAll(pattern, "*", ""))
}

func resolveReactImport(resolver *reactImportResolver, fromFile, source string) string {
	if resolver == nil {
		return ""
	}
	if strings.HasPrefix(source, ".") {
		return resolveReactCandidate(resolver.Root, filepath.Join(filepath.Dir(fromFile), source))
	}
	for _, alias := range resolver.Aliases {
		capture, ok := matchReactAlias(alias.Pattern, source)
		if !ok {
			continue
		}
		for _, target := range alias.Targets {
			mapped := strings.ReplaceAll(target, "*", capture)
			if resolved := resolveReactCandidate(resolver.Root, filepath.Join(resolver.BaseURL, mapped)); resolved != "" {
				return resolved
			}
		}
	}
	if resolver.BaseURL != "." && resolver.BaseURL != "" {
		return resolveReactCandidate(resolver.Root, filepath.Join(resolver.BaseURL, source))
	}
	return ""
}

func matchReactAlias(pattern, source string) (string, bool) {
	if !strings.Contains(pattern, "*") {
		return "", pattern == source
	}
	parts := strings.SplitN(pattern, "*", 2)
	prefix, suffix := parts[0], parts[1]
	if !strings.HasPrefix(source, prefix) || !strings.HasSuffix(source, suffix) {
		return "", false
	}
	return source[len(prefix) : len(source)-len(suffix)], true
}

func resolveReactCandidate(root, base string) string {
	candidates := make([]string, 0, len(reactResolveExts)*2)
	for _, ext := range reactResolveExts {
		if ext == "" && base == "." {
			continue
		}
		candidate := base + ext
		if fileExists(filepath.Join(root, candidate)) {
			return filepath.Clean(candidate)
		}
		candidates = append(candidates, candidate)
	}
	for _, ext := range reactResolveExts[1:] {
		candidate := filepath.Join(base, "index"+ext)
		if fileExists(filepath.Join(root, candidate)) {
			return filepath.Clean(candidate)
		}
		candidates = append(candidates, candidate)
	}
	for _, c := range candidates {
		c = filepath.Clean(c)
		if c != "." && c != "" {
			return c
		}
	}
	return ""
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func exportMemoized(infos map[string]*reactFileInfo, file, name string, seen map[string]bool) bool {
	key := file + "#" + name
	if seen[key] {
		return false
	}
	seen[key] = true
	info := infos[file]
	if info == nil {
		return false
	}
	if name == "default" {
		return info.DefaultMemoized
	}
	if info.Memoized[name] {
		return true
	}
	for _, binding := range info.ReExports[name] {
		if exportMemoized(infos, binding.Target, binding.Imported, seen) {
			return true
		}
	}
	for _, target := range info.ExportStars {
		if exportMemoized(infos, target, name, seen) {
			return true
		}
	}
	return false
}

func defaultMemoizedExport(root *sitter.Node, src []byte) bool {
	found := false
	walkReact(root, func(node *sitter.Node) {
		if found || node.Type() != "export_statement" {
			return
		}
		if !strings.HasPrefix(strings.TrimSpace(nodeText(node, src)), "export default") {
			return
		}
		walkReact(node, func(child *sitter.Node) {
			if found || child == node || child.Type() != "call_expression" {
				return
			}
			found = isReactMemoCall(child, src)
		})
	})
	return found
}

func defaultImportName(node *sitter.Node, src []byte) string {
	clause := firstNamedChildOfType(node, "import_clause")
	if clause == nil {
		return ""
	}
	for i := 0; i < int(clause.NamedChildCount()); i++ {
		ch := clause.NamedChild(i)
		if ch.Type() == "identifier" {
			return nodeText(ch, src)
		}
		if ch.Type() == "named_imports" || ch.Type() == "namespace_import" {
			return ""
		}
	}
	return ""
}

func stripReactJSONComments(src []byte) []byte {
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

func stripReactTrailingCommas(src []byte) []byte {
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
