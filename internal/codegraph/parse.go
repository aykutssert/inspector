package codegraph

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/typescript/tsx"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

const (
	maxFileBytes = 1 << 20 // 1 MiB; larger files are reported as diagnostics, not parsed
	parseTimeout = 5 * time.Second
)

type Import struct {
	Source string `json:"source"`
	Line   int    `json:"line"`
}

type Def struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Line     int    `json:"line"`
	EndLine  int    `json:"end_line"`
	Exported bool   `json:"exported"`
}

type Call struct {
	Name string `json:"name"`
	Recv string `json:"recv,omitempty"` // receiver for method calls, e.g. "db" in db.query()
	Line int    `json:"line"`
}

// Binding maps a local name introduced by an import/require to its module
// specifier, e.g. `const db = require('./db')` -> {Local: "db", Source: "./db"}.
// Imported is the original exported symbol the local refers to; "" means the
// module's default/namespace value (default import, `require()` whole module).
type Binding struct {
	Local    string `json:"local"`
	Source   string `json:"source"`
	Imported string `json:"imported,omitempty"`
}

type FileParse struct {
	Path     string    `json:"path"`
	Imports  []Import  `json:"imports,omitempty"`
	Bindings []Binding `json:"bindings,omitempty"`
	Defs     []Def     `json:"defs,omitempty"`
	Calls    []Call    `json:"calls,omitempty"`
	// DefaultExport is the name behind `module.exports = X` / `export default X`,
	// used to resolve default imports (`const x = require('./m'); x()`) back to
	// the real definition.
	DefaultExport string `json:"default_export,omitempty"`
	HasError      bool   `json:"has_error,omitempty"`
}

// langForPath selects the tree-sitter grammar by file extension. The
// JavaScript grammar cannot parse TypeScript type syntax, so .ts/.tsx route
// to their own grammars.
func langForPath(path string) *sitter.Language {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ts", ".mts", ".cts":
		return typescript.GetLanguage()
	case ".tsx":
		return tsx.GetLanguage()
	default:
		return javascript.GetLanguage()
	}
}

const defsQuery = `
(function_declaration) @func
(class_declaration) @class
(method_definition) @method
(variable_declarator value: (arrow_function)) @arrow
(variable_declarator value: (function_expression)) @arrowfn
(assignment_expression
  left: (member_expression property: (property_identifier))
  right: (function_expression)) @assign
(assignment_expression
  left: (member_expression property: (property_identifier))
  right: (arrow_function)) @assign
`

const callsQuery = `
(call_expression function: (identifier) @id)
(call_expression function: (member_expression) @member)
`

const importsQuery = `
(import_statement source: (string) @src)
(call_expression
  function: (identifier) @fn
  arguments: (arguments (string) @arg))
`

const bindingsQuery = `
(import_statement
  (import_clause (identifier) @default)
  source: (string) @src)
(import_statement
  (import_clause (named_imports (import_specifier !alias name: (identifier) @named)))
  source: (string) @src)
(import_statement
  (import_clause (named_imports (import_specifier name: (identifier) @aliasname alias: (identifier) @alias)))
  source: (string) @src)
(import_statement
  (import_clause (namespace_import (identifier) @ns))
  source: (string) @src)
(variable_declarator
  name: (identifier) @rlocal
  value: (call_expression function: (identifier) @rfn arguments: (arguments (string) @rsrc)))
(variable_declarator
  name: (object_pattern (shorthand_property_identifier_pattern) @rprop)
  value: (call_expression function: (identifier) @rfn arguments: (arguments (string) @rsrc)))
(variable_declarator
  name: (object_pattern (pair_pattern key: (property_identifier) @rpairkey value: (identifier) @rpairval))
  value: (call_expression function: (identifier) @rfn arguments: (arguments (string) @rsrc)))
`

func ParseJS(path string) (*FileParse, error) {
	if info, err := os.Stat(path); err == nil && info.Size() > maxFileBytes {
		return nil, fmt.Errorf("file too large: %d bytes (limit %d)", info.Size(), maxFileBytes)
	}
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(src) > maxFileBytes {
		return nil, fmt.Errorf("file too large: %d bytes (limit %d)", len(src), maxFileBytes)
	}
	lang := langForPath(path)
	parser := sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(lang)
	ctx, cancel := context.WithTimeout(context.Background(), parseTimeout)
	defer cancel()
	tree, err := parser.ParseCtx(ctx, nil, src)
	if err != nil {
		return nil, err
	}
	defer tree.Close()
	root := tree.RootNode()

	fp := &FileParse{Path: path, HasError: root.HasError()}
	for _, collect := range []func(*FileParse, *sitter.Node, *sitter.Language, []byte) error{
		collectDefs, collectCalls, collectImports, collectBindings,
	} {
		if err := collect(fp, root, lang, src); err != nil {
			return nil, err
		}
	}
	markExported(fp, root, lang, src)
	return fp, nil
}

func runQuery(q string, root *sitter.Node, lang *sitter.Language, fn func(name string, node *sitter.Node)) error {
	query, err := sitter.NewQuery([]byte(q), lang)
	if err != nil {
		return err
	}
	defer query.Close()
	qc := sitter.NewQueryCursor()
	defer qc.Close()
	qc.Exec(query, root)
	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}
		for _, c := range m.Captures {
			fn(query.CaptureNameForId(c.Index), c.Node)
		}
	}
	return nil
}

// runMatches is like runQuery but hands each match as a whole, so patterns with
// correlated captures (e.g. left object + property of an assignment) can be
// inspected together rather than one capture at a time.
func runMatches(q string, root *sitter.Node, lang *sitter.Language, fn func(caps map[string]*sitter.Node)) error {
	query, err := sitter.NewQuery([]byte(q), lang)
	if err != nil {
		return err
	}
	defer query.Close()
	qc := sitter.NewQueryCursor()
	defer qc.Close()
	qc.Exec(query, root)
	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}
		caps := map[string]*sitter.Node{}
		for _, c := range m.Captures {
			caps[query.CaptureNameForId(c.Index)] = c.Node
		}
		fn(caps)
	}
	return nil
}

func collectDefs(fp *FileParse, root *sitter.Node, lang *sitter.Language, src []byte) error {
	return runQuery(defsQuery, root, lang, func(name string, node *sitter.Node) {
		var d Def
		d.Line = int(node.StartPoint().Row) + 1
		d.EndLine = int(node.EndPoint().Row) + 1
		switch name {
		case "func":
			d.Kind = "function"
			d.Name = fieldText(node, "name", src)
		case "class":
			d.Kind = "class"
			d.Name = fieldText(node, "name", src)
		case "method":
			d.Kind = "method"
			d.Name = fieldText(node, "name", src)
		case "arrow", "arrowfn":
			d.Kind = "function"
			d.Name = fieldText(node, "name", src)
		case "assign":
			d.Kind = "method"
			if left := node.ChildByFieldName("left"); left != nil {
				d.Name = fieldText(left, "property", src)
			}
		}
		if d.Name == "" {
			return
		}
		d.Exported = hasExportAncestor(node)
		fp.Defs = append(fp.Defs, d)
	})
}

func collectCalls(fp *FileParse, root *sitter.Node, lang *sitter.Language, src []byte) error {
	seen := map[string]bool{}
	return runQuery(callsQuery, root, lang, func(name string, node *sitter.Node) {
		var c Call
		c.Line = int(node.StartPoint().Row) + 1
		switch name {
		case "id":
			c.Name = node.Content(src)
		case "member":
			prop := node.ChildByFieldName("property")
			if prop == nil {
				return
			}
			c.Name = prop.Content(src)
			if obj := node.ChildByFieldName("object"); obj != nil {
				c.Recv = obj.Content(src)
			}
		}
		if c.Name == "" || c.Name == "require" {
			return
		}
		key := c.Name + ":" + c.Recv + ":" + itoa(c.Line)
		if seen[key] {
			return
		}
		seen[key] = true
		fp.Calls = append(fp.Calls, c)
	})
}

func collectImports(fp *FileParse, root *sitter.Node, lang *sitter.Language, src []byte) error {
	var lastFn string
	return runQuery(importsQuery, root, lang, func(name string, node *sitter.Node) {
		switch name {
		case "src":
			fp.Imports = append(fp.Imports, Import{
				Source: trimQuotes(node.Content(src)),
				Line:   int(node.StartPoint().Row) + 1,
			})
		case "fn":
			lastFn = node.Content(src)
		case "arg":
			if lastFn == "require" {
				fp.Imports = append(fp.Imports, Import{
					Source: trimQuotes(node.Content(src)),
					Line:   int(node.StartPoint().Row) + 1,
				})
			}
			lastFn = ""
		}
	})
}

const exportAssignQuery = `
(assignment_expression
  left: (member_expression object: (identifier) @lobj property: (property_identifier) @lprop)
  right: (identifier) @rhs)
(assignment_expression
  left: (member_expression object: (identifier) @lobj property: (property_identifier) @lprop)
  right: (object) @robj)
(assignment_expression
  left: (member_expression
    object: (member_expression object: (identifier) @nobj property: (property_identifier) @nprop)
    property: (property_identifier) @nleaf))
`

const exportEsQuery = `
(export_specifier name: (identifier) @name)
(export_statement (identifier) @default)
`

// markExported flags defs reachable through CommonJS (module.exports / exports.x)
// and ES (export {}, export default) so callers know the public surface. ES
// inline forms (export function f) are already caught by hasExportAncestor.
func markExported(fp *FileParse, root *sitter.Node, lang *sitter.Language, src []byte) {
	exported := map[string]bool{}
	_ = runMatches(exportAssignQuery, root, lang, func(caps map[string]*sitter.Node) {
		// module.exports.foo = fn  ->  mark "foo"
		if leaf := caps["nleaf"]; leaf != nil {
			if nodeText(caps["nobj"], src) == "module" && nodeText(caps["nprop"], src) == "exports" {
				exported[leaf.Content(src)] = true
			}
			return
		}
		lobj := nodeText(caps["lobj"], src)
		lprop := nodeText(caps["lprop"], src)
		moduleExports := lobj == "module" && lprop == "exports"
		exportsDot := lobj == "exports"
		if !moduleExports && !exportsDot {
			return
		}
		if rhs := caps["rhs"]; rhs != nil {
			exported[rhs.Content(src)] = true
			if moduleExports {
				fp.DefaultExport = rhs.Content(src) // module.exports = X
			}
		}
		if exportsDot {
			exported[lprop] = true
		}
		if robj := caps["robj"]; robj != nil && moduleExports {
			objectExportNames(robj, src, exported)
		}
	})
	_ = runQuery(exportEsQuery, root, lang, func(name string, node *sitter.Node) {
		exported[node.Content(src)] = true
		if name == "default" { // export default X
			fp.DefaultExport = node.Content(src)
		}
	})
	if len(exported) == 0 {
		return
	}
	for i := range fp.Defs {
		if exported[fp.Defs[i].Name] {
			fp.Defs[i].Exported = true
		}
	}
}

// objectExportNames marks names from `module.exports = { ... }`. For a pair
// `publicName: internalFn` the value identifier (the real def) is what we flag;
// the key is also marked in case a def carries that name.
func objectExportNames(obj *sitter.Node, src []byte, set map[string]bool) {
	for i := 0; i < int(obj.NamedChildCount()); i++ {
		ch := obj.NamedChild(i)
		switch ch.Type() {
		case "shorthand_property_identifier", "shorthand_property_identifier_pattern":
			set[ch.Content(src)] = true
		case "pair":
			if k := ch.ChildByFieldName("key"); k != nil {
				set[k.Content(src)] = true
			}
			if v := ch.ChildByFieldName("value"); v != nil && v.Type() == "identifier" {
				set[v.Content(src)] = true
			}
		}
	}
}

func nodeText(node *sitter.Node, src []byte) string {
	if node == nil {
		return ""
	}
	return node.Content(src)
}

func collectBindings(fp *FileParse, root *sitter.Node, lang *sitter.Language, src []byte) error {
	return runMatches(bindingsQuery, root, lang, func(caps map[string]*sitter.Node) {
		// ES import: one source plus a default / named / namespace local name.
		if s := caps["src"]; s != nil {
			source := trimQuotes(s.Content(src))
			if a := caps["alias"]; a != nil {
				// `import { orig as local }` -> Imported is the original name.
				fp.Bindings = append(fp.Bindings, Binding{Local: a.Content(src), Source: source, Imported: nodeText(caps["aliasname"], src)})
			} else if n := caps["named"]; n != nil {
				// `import { name }` -> local and imported are the same.
				name := n.Content(src)
				fp.Bindings = append(fp.Bindings, Binding{Local: name, Source: source, Imported: name})
			}
			if d := caps["default"]; d != nil {
				// `import x from` -> default export (Imported left "").
				fp.Bindings = append(fp.Bindings, Binding{Local: d.Content(src), Source: source})
			}
			if ns := caps["ns"]; ns != nil {
				// `import * as ns` -> namespace (Imported left "").
				fp.Bindings = append(fp.Bindings, Binding{Local: ns.Content(src), Source: source})
			}
			return
		}
		// CommonJS: const x = require('...') or const { a, b } = require('...').
		if fn := caps["rfn"]; fn == nil || fn.Content(src) != "require" {
			return
		}
		rsrc := caps["rsrc"]
		if rsrc == nil {
			return
		}
		source := trimQuotes(rsrc.Content(src))
		if l := caps["rlocal"]; l != nil {
			// `const x = require(...)` -> whole module / default (Imported "").
			fp.Bindings = append(fp.Bindings, Binding{Local: l.Content(src), Source: source})
		}
		if p := caps["rprop"]; p != nil {
			// `const { a } = require(...)` -> destructured named export.
			name := p.Content(src)
			fp.Bindings = append(fp.Bindings, Binding{Local: name, Source: source, Imported: name})
		}
		if k, v := caps["rpairkey"], caps["rpairval"]; k != nil && v != nil {
			// `const { orig: local } = require(...)` -> renamed named export.
			fp.Bindings = append(fp.Bindings, Binding{Local: v.Content(src), Source: source, Imported: k.Content(src)})
		}
	})
}

func fieldText(node *sitter.Node, field string, src []byte) string {
	c := node.ChildByFieldName(field)
	if c == nil {
		return ""
	}
	return c.Content(src)
}

func hasExportAncestor(node *sitter.Node) bool {
	for p := node.Parent(); p != nil; p = p.Parent() {
		t := p.Type()
		if t == "export_statement" {
			return true
		}
		if t == "program" {
			return false
		}
	}
	return false
}

func trimQuotes(s string) string {
	return strings.Trim(s, "'\"`")
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
