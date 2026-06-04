package policycoverage

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

	"github.com/aykutssert/inspector/internal/architecture/policy"
	"github.com/aykutssert/inspector/internal/core"
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

func (a *Analyzer) Name() string { return "policy-coverage" }

func (a *Analyzer) Available() bool { return true }

func (a *Analyzer) Scan(ctx core.ProjectContext) ([]core.Finding, error) {
	var endpoints []policy.Endpoint

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

	// Default rules for NestJS and Express
	rules := []Rule{
		{
			ID:            "nestjs.missing-auth-guard",
			Framework:     "nestjs",
			RequiredAnyOf: []string{"JwtAuthGuard", "AuthGuard"},
			Exclusions:    []string{"Public"},
			Message:       "NestJS controller route is missing authentication guards or explicit @Public() exclusion.",
		},
		{
			ID:            "express.missing-auth-middleware",
			Framework:     "express",
			RequiredAnyOf: []string{"auth", "requireAuth", "authenticate", "passport.authenticate"},
			Message:       "Express route is missing required authentication middleware.",
		},
	}

	violations := policy.Analyze(endpoints, translateRules(rules))

	var findings []core.Finding
	for _, v := range violations {
		findings = append(findings, core.Finding{
			Analyzer:   a.Name(),
			RuleID:     v.RuleID,
			Severity:   core.SeverityWarning,
			Level:      core.SeverityWarning.String(),
			Category:   "security",
			// hint: route policy coverage is inferred from guard/middleware names,
			// which legitimately vary (public routes, custom auth names). The agent
			// confirms the route truly lacks auth. Kept at hint until the
			// false-positive profile is measured, like the taint rules.
			Confidence: core.ConfidenceHint,
			File:       v.File,
			Line:       v.Line,
			Message:    v.Message,
			Fix:        "Apply a required security guard, middleware, or designate the route as public with an explicit exclusion.",
		})
	}

	return findings, nil
}

// Rule wrapper to local package structure
type Rule struct {
	ID            string
	Framework     string
	RequiredAnyOf []string
	Exclusions    []string
	Message       string
}

func translateRules(rules []Rule) []policy.Rule {
	var out []policy.Rule
	for _, r := range rules {
		out = append(out, policy.Rule{
			ID:            r.ID,
			Framework:     r.Framework,
			RequiredAnyOf: r.RequiredAnyOf,
			Exclusions:    r.Exclusions,
			Message:       r.Message,
		})
	}
	return out
}

func scanFile(root, rel string) ([]policy.Endpoint, error) {
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

	var endpoints []policy.Endpoint
	rootNode := tree.RootNode()

	// 1. Extract NestJS Controllers
	walk(rootNode, func(n *sitter.Node) {
		if n.Type() != "class_declaration" {
			return
		}
		decorators := decoratorsOfNode(n, src)
		var controllerPath string
		isController := false
		for decName, decArgs := range decorators {
			if decName == "Controller" {
				isController = true
				if len(decArgs) > 0 {
					controllerPath = decArgs[0]
				}
				break
			}
		}
		if !isController {
			return
		}

		className := nodeText(n.ChildByFieldName("name"), src)

		// Collect class-level policies
		var classPolicies []string
		for decName, decArgs := range decorators {
			if decName == "UseGuards" {
				classPolicies = append(classPolicies, decArgs...)
			}
		}

		// Traverse class methods
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
			var routePath string
			for decName, decArgs := range methodDecorators {
				if httpMethods[decName] {
					httpMethod = strings.ToUpper(decName)
					if len(decArgs) > 0 {
						routePath = decArgs[0]
					}
					break
				}
			}

			if httpMethod == "" {
				continue // Not a route handler
			}

			fullRoute := fmt.Sprintf("%s %s", httpMethod, combinePaths(controllerPath, routePath))

			// Collect method-level policies & exclusions
			var methodPolicies []string
			var exclusions []string
			for decName, decArgs := range methodDecorators {
				if decName == "UseGuards" {
					methodPolicies = append(methodPolicies, decArgs...)
				} else if decName == "Public" {
					exclusions = append(exclusions, "Public")
				}
			}

			endpoints = append(endpoints, policy.Endpoint{
				Framework:  "nestjs",
				File:       rel,
				Line:       int(method.StartPoint().Row) + 1,
				Class:      className,
				Handler:    methodName,
				Route:      fullRoute,
				Policies:   append(append([]string(nil), classPolicies...), methodPolicies...),
				Exclusions: exclusions,
			})
		}
	})

	// 2. Extract Express Routes
	walk(rootNode, func(n *sitter.Node) {
		if n.Type() != "call_expression" {
			return
		}
		callee := n.ChildByFieldName("function")
		if callee == nil || callee.Type() != "member_expression" {
			return
		}

		property := callee.ChildByFieldName("property")
		if property == nil {
			return
		}
		propName := nodeText(property, src)
		if !httpMethods[propName] {
			return
		}

		argsNode := n.ChildByFieldName("arguments")
		if argsNode == nil || argsNode.NamedChildCount() < 2 {
			return
		}

		// First argument must be the path literal
		firstArg := argsNode.NamedChild(0)
		if firstArg == nil || (firstArg.Type() != "string" && firstArg.Type() != "string_fragment") {
			return
		}

		routePath := cleanQuotes(nodeText(firstArg, src))
		method := strings.ToUpper(propName)

		// Parse remaining arguments as middleware / handlers
		var policies []string
		for i := 1; i < int(argsNode.NamedChildCount())-1; i++ {
			arg := argsNode.NamedChild(i)
			if arg == nil {
				continue
			}
			policies = append(policies, extractExpressPolicy(arg, src)...)
		}

		endpoints = append(endpoints, policy.Endpoint{
			Framework: "express",
			File:      rel,
			Line:      int(n.StartPoint().Row) + 1,
			Route:     fmt.Sprintf("%s %s", method, routePath),
			Handler:   "expressRoute",
			Policies:  policies,
		})
	})

	return endpoints, nil
}

func extractExpressPolicy(node *sitter.Node, src []byte) []string {
	var out []string
	switch node.Type() {
	case "identifier":
		out = append(out, nodeText(node, src))
	case "member_expression":
		out = append(out, nodeText(node, src))
	case "call_expression":
		// e.g. passport.authenticate('jwt')
		fn := node.ChildByFieldName("function")
		if fn != nil {
			out = append(out, nodeText(fn, src))
		}
		// Also add string arguments (like 'jwt') as policies
		args := node.ChildByFieldName("arguments")
		if args != nil {
			for i := 0; i < int(args.NamedChildCount()); i++ {
				arg := args.NamedChild(i)
				if arg != nil && (arg.Type() == "string" || arg.Type() == "string_fragment") {
					out = append(out, cleanQuotes(nodeText(arg, src)))
				}
			}
		}
	}
	return out
}

func decoratorsOfNode(n *sitter.Node, src []byte) map[string][]string {
	out := map[string][]string{}
	// Decorators can precede class declarations in parent
	parent := n.Parent()
	if parent != nil {
		for i := 0; i < int(parent.NamedChildCount()); i++ {
			ch := parent.NamedChild(i)
			if ch == n {
				break
			}
			if ch.Type() == "decorator" {
				name, args := parseDecorator(ch, src)
				if name != "" {
					out[name] = args
				}
			}
		}
	}
	// Or exist inside the node children
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
	// Decorators are typically (decorator (call_expression ...)) or (decorator (identifier))
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

func combinePaths(base, sub string) string {
	base = cleanQuotes(base)
	sub = cleanQuotes(sub)
	if !strings.HasPrefix(base, "/") {
		base = "/" + base
	}
	base = strings.TrimSuffix(base, "/")
	if sub != "" && !strings.HasPrefix(sub, "/") {
		sub = "/" + sub
	}
	return base + sub
}
