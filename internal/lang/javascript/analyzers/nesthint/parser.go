package nesthint

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/typescript/tsx"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

const (
	maxFileBytes = 1 << 20
	parseTimeout = 5 * time.Second
)

func parseFile(root, rel string, resolver *importResolver) (*fileInfo, error) {
	abs := filepath.Join(root, rel)
	if info, err := os.Stat(abs); err == nil && info.Size() > maxFileBytes {
		return nil, fmt.Errorf("file too large")
	}
	src, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	if len(src) > maxFileBytes {
		return nil, fmt.Errorf("file too large")
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

	fi := &fileInfo{
		Path:     rel,
		Resolver: resolver,
		Imports:  map[string]importBinding{},
		Classes:  map[string]*classInfo{},
	}
	rootNode := tree.RootNode()
	walk(rootNode, func(n *sitter.Node) {
		switch n.Type() {
		case "import_statement":
			collectImport(fi, n, src)
		case "lexical_declaration", "variable_declaration":
			collectMutableVar(fi, n, src)
		}
	})
	walk(rootNode, func(n *sitter.Node) {
		if n.Type() != "class_declaration" {
			return
		}
		name := className(n, src)
		if name == "" {
			return
		}
		c := &classInfo{
			Ref:        symbolRef{File: rel, Name: name},
			Line:       line(n),
			Decorators: classDecorators(n, src),
		}
		c.Constructor = constructorDeps(rel, fi, n, src)
		fi.Classes[name] = c
		if c.Decorators["Module"] {
			fi.Modules = append(fi.Modules, moduleFromDecorator(rel, fi, c, n, src))
		}
		if c.Decorators["Injectable"] {
			c.InjectableScope = extractInjectableScope(n, src)
		}
	})
	return fi, nil
}

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

func className(n *sitter.Node, src []byte) string {
	for i := 0; i < int(n.NamedChildCount()); i++ {
		ch := n.NamedChild(i)
		if ch.Type() == "type_identifier" || ch.Type() == "identifier" {
			return text(ch, src)
		}
	}
	return ""
}

func classDecorators(classNode *sitter.Node, src []byte) map[string]bool {
	out := map[string]bool{}
	parent := classNode.Parent()
	if parent != nil {
		for i := 0; i < int(parent.NamedChildCount()); i++ {
			ch := parent.NamedChild(i)
			if ch == classNode {
				break
			}
			if ch.Type() == "decorator" {
				if name := decoratorName(ch, src); name != "" {
					out[name] = true
				}
			}
		}
	}
	for i := 0; i < int(classNode.NamedChildCount()); i++ {
		ch := classNode.NamedChild(i)
		if ch.Type() == "decorator" {
			if name := decoratorName(ch, src); name != "" {
				out[name] = true
			}
		}
	}
	return out
}

func decoratorName(n *sitter.Node, src []byte) string {
	var name string
	walk(n, func(c *sitter.Node) {
		if name != "" {
			return
		}
		if c.Type() == "identifier" {
			name = text(c, src)
		}
	})
	return name
}

func findClassDecorator(classNode *sitter.Node, want string, src []byte) *sitter.Node {
	parent := classNode.Parent()
	if parent != nil {
		for i := 0; i < int(parent.NamedChildCount()); i++ {
			ch := parent.NamedChild(i)
			if ch == classNode {
				break
			}
			if ch.Type() == "decorator" && decoratorName(ch, src) == want {
				return ch
			}
		}
	}
	for i := 0; i < int(classNode.NamedChildCount()); i++ {
		ch := classNode.NamedChild(i)
		if ch.Type() == "decorator" && decoratorName(ch, src) == want {
			return ch
		}
	}
	return nil
}

func constructorDeps(rel string, fi *fileInfo, classNode *sitter.Node, src []byte) []injectionDep {
	var deps []injectionDep
	walk(classNode, func(n *sitter.Node) {
		if n.Type() != "method_definition" || methodName(n, src) != "constructor" {
			return
		}
		params := firstChildOfType(n, "formal_parameters")
		if params == nil {
			return
		}
		for i := 0; i < int(params.NamedChildCount()); i++ {
			param := params.NamedChild(i)
			if param.Type() != "required_parameter" && param.Type() != "optional_parameter" {
				continue
			}
			if hasDecorator(param, "Inject", src) {
				continue
			}
			typeName := parameterTypeName(param, src)
			if typeName == "" || isPrimitiveType(typeName) {
				continue
			}
			ref := resolveLocal(rel, fi, typeName)
			if ref.key() == "" {
				continue
			}
			deps = append(deps, injectionDep{Name: typeName, Ref: ref, Line: line(param)})
		}
	})
	return deps
}

func methodName(n *sitter.Node, src []byte) string {
	for i := 0; i < int(n.NamedChildCount()); i++ {
		ch := n.NamedChild(i)
		if ch.Type() == "property_identifier" || ch.Type() == "identifier" {
			return text(ch, src)
		}
	}
	return ""
}

func hasDecorator(n *sitter.Node, want string, src []byte) bool {
	found := false
	walk(n, func(c *sitter.Node) {
		if c.Type() == "decorator" && decoratorName(c, src) == want {
			found = true
		}
	})
	return found
}

var simpleTypeRe = regexp.MustCompile(`^([A-Za-z_$][A-Za-z0-9_$]*)`)

func parameterTypeName(param *sitter.Node, src []byte) string {
	var raw string
	walk(param, func(n *sitter.Node) {
		if raw != "" || n.Type() != "type_annotation" {
			return
		}
		raw = strings.TrimSpace(strings.TrimPrefix(text(n, src), ":"))
	})
	if raw == "" {
		return ""
	}
	m := simpleTypeRe.FindStringSubmatch(raw)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

func isPrimitiveType(name string) bool {
	switch name {
	case "string", "number", "boolean", "bigint", "symbol", "object", "unknown", "any", "void", "never", "Date", "Promise", "Array", "Record":
		return true
	default:
		return false
	}
}
