package codegraph

import (
	"context"
	"os"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
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
	Line int    `json:"line"`
}

type FileParse struct {
	Path    string   `json:"path"`
	Imports []Import `json:"imports,omitempty"`
	Defs    []Def    `json:"defs,omitempty"`
	Calls   []Call   `json:"calls,omitempty"`
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
(call_expression function: (member_expression property: (property_identifier) @prop))
`

const importsQuery = `
(import_statement source: (string) @src)
(call_expression
  function: (identifier) @fn
  arguments: (arguments (string) @arg))
`

func ParseJS(path string) (*FileParse, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lang := javascript.GetLanguage()
	parser := sitter.NewParser()
	parser.SetLanguage(lang)
	tree, err := parser.ParseCtx(context.Background(), nil, src)
	if err != nil {
		return nil, err
	}
	root := tree.RootNode()

	fp := &FileParse{Path: path}
	collectDefs(fp, root, lang, src)
	collectCalls(fp, root, lang, src)
	collectImports(fp, root, lang, src)
	return fp, nil
}

func runQuery(q string, root *sitter.Node, lang *sitter.Language, fn func(name string, node *sitter.Node)) {
	query, err := sitter.NewQuery([]byte(q), lang)
	if err != nil {
		return
	}
	qc := sitter.NewQueryCursor()
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
}

func collectDefs(fp *FileParse, root *sitter.Node, lang *sitter.Language, src []byte) {
	runQuery(defsQuery, root, lang, func(name string, node *sitter.Node) {
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

func collectCalls(fp *FileParse, root *sitter.Node, lang *sitter.Language, src []byte) {
	seen := map[string]bool{}
	runQuery(callsQuery, root, lang, func(name string, node *sitter.Node) {
		text := node.Content(src)
		if text == "" || text == "require" {
			return
		}
		line := int(node.StartPoint().Row) + 1
		key := text + ":" + itoa(line)
		if seen[key] {
			return
		}
		seen[key] = true
		fp.Calls = append(fp.Calls, Call{Name: text, Line: line})
	})
}

func collectImports(fp *FileParse, root *sitter.Node, lang *sitter.Language, src []byte) {
	var lastFn string
	runQuery(importsQuery, root, lang, func(name string, node *sitter.Node) {
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
