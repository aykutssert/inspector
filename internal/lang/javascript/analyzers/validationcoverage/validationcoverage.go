package validationcoverage

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

	"github.com/aykutssert/scout/internal/architecture/validation"
	"github.com/aykutssert/scout/internal/core"
)

const (
	maxFileBytes = 1 << 20 // 1 MiB
	parseTimeout = 5 * time.Second
)

var jsExt = map[string]bool{
	".js": true, ".jsx": true, ".ts": true, ".tsx": true,
	".mjs": true, ".cjs": true, ".mts": true, ".cts": true,
}

var httpMethods = map[string]bool{
	"Get": true, "Post": true, "Put": true, "Delete": true, "Patch": true, "All": true,
	"get": true, "post": true, "put": true, "delete": true, "patch": true, "all": true,
}

type Analyzer struct{}

func New() *Analyzer { return &Analyzer{} }

var _ core.Analyzer = (*Analyzer)(nil)

func (a *Analyzer) Name() string { return "validation-coverage" }

func (a *Analyzer) Available() bool { return true }

func (a *Analyzer) Scan(ctx core.ProjectContext) ([]core.Finding, error) {
	var endpoints []validation.Endpoint

	for _, rel := range ctx.Files {
		if !jsExt[strings.ToLower(filepath.Ext(rel))] {
			continue
		}
		eps, err := scanFile(ctx.Root, rel)
		if err != nil {
			continue
		}
		endpoints = append(endpoints, eps...)
	}

	if len(endpoints) == 0 {
		return nil, nil
	}

	rules := []validation.Rule{
		{
			ID:        "nestjs.nestjs-missing-validation-pipe",
			Framework: "nestjs",
			Message:   "A NestJS controller method accepts a request body via @Body(), but no @UsePipes(ValidationPipe) decorator is declared on the method or the class. Ensure input validation is active (either via local @UsePipes or a global ValidationPipe in main.ts) to prevent unvalidated payloads.",
		},
	}

	violations := validation.Analyze(endpoints, rules)

	var findings []core.Finding
	for _, v := range violations {
		findings = append(findings, core.Finding{
			Analyzer:   a.Name(),
			RuleID:     v.RuleID,
			Severity:   core.SeverityWarning,
			Level:      core.SeverityWarning.String(),
			Category:   "maintainability",
			Confidence: core.ConfidenceHint,
			File:       v.File,
			Line:       v.Line,
			Message:    v.Message,
			Fix:        "Apply @UsePipes(ValidationPipe) on the class or method.",
		})
	}

	return findings, nil
}

func scanFile(root, rel string) ([]validation.Endpoint, error) {
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

	var endpoints []validation.Endpoint
	rootNode := tree.RootNode()

	walk(rootNode, func(n *sitter.Node) {
		if n.Type() != "class_declaration" {
			return
		}
		decorators := decoratorsOfNode(n, src)
		isController := false
		for decName := range decorators {
			if decName == "Controller" {
				isController = true
				break
			}
		}
		if !isController {
			return
		}

		_, classValidated := decorators["UsePipes"]

		classBody := n.ChildByFieldName("body")
		if classBody == nil {
			return
		}
		for i := 0; i < int(classBody.NamedChildCount()); i++ {
			method := classBody.NamedChild(i)
			if method.Type() != "method_definition" {
				continue
			}

			methodName := nodeText(method.ChildByFieldName("name"), src)
			methodDecorators := decoratorsOfNode(method, src)

			var httpMethod string
			for decName := range methodDecorators {
				if httpMethods[decName] {
					httpMethod = strings.ToUpper(decName)
					break
				}
			}

			if httpMethod == "" {
				continue
			}

			_, methodValidated := methodDecorators["UsePipes"]
			hasBody := methodHasBodyParam(method, src)

			endpoints = append(endpoints, validation.Endpoint{
				Framework: "nestjs",
				File:      rel,
				Line:      int(method.StartPoint().Row) + 1,
				Route:     fmt.Sprintf("%s %s", httpMethod, methodName),
				Handler:   methodName,
				HasBody:   hasBody,
				Validated: classValidated || methodValidated,
			})
		}
	})

	return endpoints, nil
}

func methodHasBodyParam(methodNode *sitter.Node, src []byte) bool {
	params := methodNode.ChildByFieldName("parameters")
	if params == nil {
		for i := 0; i < int(methodNode.NamedChildCount()); i++ {
			if methodNode.NamedChild(i).Type() == "formal_parameters" {
				params = methodNode.NamedChild(i)
				break
			}
		}
	}
	if params == nil {
		return false
	}

	hasBody := false
	walk(params, func(n *sitter.Node) {
		if n.Type() == "decorator" {
			name, _ := parseDecorator(n, src)
			if name == "Body" {
				hasBody = true
			}
		}
	})
	return hasBody
}

func decoratorsOfNode(n *sitter.Node, src []byte) map[string][]string {
	out := map[string][]string{}
	parent := n.Parent()
	if parent != nil {
		pending := map[string][]string{}
		for i := 0; i < int(parent.NamedChildCount()); i++ {
			ch := parent.NamedChild(i)
			if ch == n {
				for k, v := range pending {
					out[k] = v
				}
				break
			}
			if ch.Type() == "decorator" {
				name, args := parseDecorator(ch, src)
				if name != "" {
					pending[name] = args
				}
			} else {
				pending = map[string][]string{}
			}
		}
	}
	for i := 0; i < int(n.NamedChildCount()); i++ {
		ch := n.NamedChild(i)
		if ch.Type() == "decorator" {
			name, args := parseDecorator(ch, src)
			if name != "" {
				out[name] = args
			}
		}
	}
	return out
}

func parseDecorator(decNode *sitter.Node, src []byte) (string, []string) {
	var name string
	var args []string

	call := firstChildOfType(decNode, "call_expression")
	if call != nil {
		fn := call.ChildByFieldName("function")
		name = nodeText(fn, src)
		argsNode := call.ChildByFieldName("arguments")
		if argsNode != nil {
			for i := 0; i < int(argsNode.NamedChildCount()); i++ {
				arg := argsNode.NamedChild(i)
				if arg != nil {
					args = append(args, cleanQuotes(nodeText(arg, src)))
				}
			}
		}
	} else {
		id := firstChildOfType(decNode, "identifier")
		if id != nil {
			name = nodeText(id, src)
		}
	}

	return name, args
}

func firstChildOfType(n *sitter.Node, typ string) *sitter.Node {
	for i := 0; i < int(n.NamedChildCount()); i++ {
		ch := n.NamedChild(i)
		if ch.Type() == typ {
			return ch
		}
	}
	return nil
}

func walk(n *sitter.Node, fn func(*sitter.Node)) {
	fn(n)
	for i := 0; i < int(n.NamedChildCount()); i++ {
		walk(n.NamedChild(i), fn)
	}
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

func nodeText(n *sitter.Node, src []byte) string {
	if n == nil {
		return ""
	}
	return string(src[n.StartByte():n.EndByte()])
}

func cleanQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '`' && s[len(s)-1] == '`')) {
		return s[1 : len(s)-1]
	}
	return s
}
