package taintflow

import (
	"fmt"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/aykutssert/scout/internal/core"
)

// nosqlMethods are mongoose/MongoDB query methods that accept a filter
// object as their first argument. A tainted filter lets an attacker inject
// query operators ($where, $gt, $ne, ...).
var nosqlMethods = map[string]bool{
	"find": true, "findOne": true, "findOneAndUpdate": true,
	"findOneAndDelete": true, "findOneAndReplace": true,
	"deleteOne": true, "deleteMany": true,
	"updateOne": true, "updateMany": true, "replaceOne": true,
	"aggregate": true, "countDocuments": true,
}

// massAssignMethods accept a whole object as the document to persist,
// without field-level filtering.
var massAssignMethods = map[string]bool{
	"create": true, "insertMany": true,
}

// commandFuncs spawn a shell command, whether called as a bare identifier
// (destructured import) or as a method on child_process/cp.
var commandFuncs = map[string]bool{
	"exec": true, "execSync": true, "execFile": true, "execFileSync": true,
	"spawn": true, "spawnSync": true,
}

const maxSinkTextLen = 80

func sinkText(node *sitter.Node, src []byte) string {
	t := node.Content(src)
	if len(t) > maxSinkTextLen {
		t = t[:maxSinkTextLen-1] + "…"
	}
	return t
}

// callee returns the method/function name of a call_expression and whether
// it is a member call (obj.method(...)) vs a bare call (method(...)).
func callee(call *sitter.Node, src []byte) (name string, isMember bool) {
	fn := call.ChildByFieldName("function")
	if fn == nil {
		return "", false
	}
	switch fn.Type() {
	case "member_expression":
		prop := fn.ChildByFieldName("property")
		if prop == nil {
			return "", false
		}
		return prop.Content(src), true
	case "identifier":
		return fn.Content(src), false
	}
	return "", false
}

// callArgs returns the named argument nodes of a call_expression.
func callArgs(call *sitter.Node) []*sitter.Node {
	args := call.ChildByFieldName("arguments")
	if args == nil {
		return nil
	}
	out := make([]*sitter.Node, 0, args.NamedChildCount())
	for i := 0; i < int(args.NamedChildCount()); i++ {
		out = append(out, args.NamedChild(i))
	}
	return out
}

// checkSink matches a call_expression against the known sink table and, if
// one of its relevant arguments is tainted, builds a Finding.
func checkSink(call *sitter.Node, src []byte, file string, tainted map[string]taintInfo) []core.Finding {
	method, isMember := callee(call, src)
	if method == "" {
		return nil
	}
	args := callArgs(call)
	if len(args) == 0 {
		return nil
	}

	switch {
	case isMember && nosqlMethods[method]:
		if name, ok := taintedRoot(args[0], src, tainted); ok {
			return []core.Finding{buildFinding(call, src, file, "taint-nosql-query",
				fmt.Sprintf("`%s` is assigned from `%s` (line %d) and passed as the filter to `%s` here. An attacker-controlled query operator (e.g. `$where`, `$gt`, `$ne`, `$regex`) in that object could alter or bypass the query. (CWE-943)",
					name, tainted[name].source, tainted[name].line, sinkText(call, src)),
				"Build the filter from named, validated fields (e.g. `{ id: req.params.id }`) instead of passing the raw object through.",
				core.SeverityWarning)}
		}
	case isMember && massAssignMethods[method]:
		if name, ok := taintedRoot(args[0], src, tainted); ok {
			return []core.Finding{buildFinding(call, src, file, "taint-mass-assignment",
				fmt.Sprintf("`%s` is assigned from `%s` (line %d) and passed directly to `%s` here. An attacker can set unintended fields (e.g. `role`, `isAdmin`, `id`) via mass assignment. (CWE-915)",
					name, tainted[name].source, tainted[name].line, sinkText(call, src)),
				"Pick only the allowed fields explicitly (e.g. `{ name, email }`) before creating/inserting.",
				core.SeverityWarning)}
		}
	case commandFuncs[method]:
		if name, ok := taintedRoot(args[0], src, tainted); ok {
			return []core.Finding{buildFinding(call, src, file, "taint-command-injection",
				fmt.Sprintf("`%s` is assigned from `%s` (line %d) and passed to `%s` here. If it reaches the shell unescaped, an attacker can inject arbitrary commands. (CWE-78)",
					name, tainted[name].source, tainted[name].line, sinkText(call, src)),
				"Avoid building shell commands from user input; use execFile/spawn with an argument array, or validate against an allow-list.",
				core.SeverityWarning)}
		}
	case !isMember && method == "eval":
		if name, ok := taintedRoot(args[0], src, tainted); ok {
			return []core.Finding{buildFinding(call, src, file, "taint-code-injection",
				fmt.Sprintf("`%s` is assigned from `%s` (line %d) and passed to `eval()` here. Arbitrary attacker-controlled code can execute. (CWE-94)",
					name, tainted[name].source, tainted[name].line),
				"Remove eval; use JSON.parse for data or a safe expression evaluator.",
				core.SeverityWarning)}
		}
	case !isMember && method == "fetch":
		if name, ok := taintedRoot(args[0], src, tainted); ok {
			return []core.Finding{buildFinding(call, src, file, "taint-ssrf",
				fmt.Sprintf("`%s` is assigned from `%s` (line %d) and used as the URL for `fetch()` here. An attacker can make the server request arbitrary internal or external addresses (SSRF). (CWE-918)",
					name, tainted[name].source, tainted[name].line),
				"Validate the URL against an allow-list of trusted hosts before fetching.",
				core.SeverityWarning)}
		}
	}
	return nil
}

func buildFinding(call *sitter.Node, src []byte, file, ruleID, message, fix string, sev core.Severity) core.Finding {
	return core.Finding{
		Analyzer:   "taint-flow",
		RuleID:     ruleID,
		Severity:   sev,
		Level:      sev.String(),
		Category:   "security",
		Confidence: core.ConfidenceHint,
		File:       file,
		Line:       int(call.StartPoint().Row) + 1,
		Message:    message,
		Fix:        fix,
	}
}
