package nesthint

import sitter "github.com/smacker/go-tree-sitter"

func moduleFromDecorator(rel string, fi *fileInfo, c *classInfo, classNode *sitter.Node, src []byte) *moduleInfo {
	m := &moduleInfo{
		Ref:         c.Ref,
		File:        rel,
		Line:        c.Line,
		IsGlobal:    c.Decorators["Global"],
		Imports:     map[string]bool{},
		Controllers: map[string]bool{},
		Providers:   map[string]bool{},
		Exports:     map[string]bool{},
	}
	if dec := findClassDecorator(classNode, "Module", src); dec != nil {
		if obj := firstChildOfType(dec, "object"); obj != nil {
			readModuleArray(rel, fi, obj, "imports", m.Imports, &m.UnknownImports, src)
			readModuleArray(rel, fi, obj, "controllers", m.Controllers, nil, src)
			readModuleArray(rel, fi, obj, "providers", m.Providers, &m.UnknownProvider, src)
			readModuleArray(rel, fi, obj, "exports", m.Exports, &m.UnknownExports, src)
		}
	}
	return m
}

func readModuleArray(rel string, fi *fileInfo, obj *sitter.Node, prop string, out map[string]bool, unknown *bool, src []byte) {
	pair := objectPair(obj, prop, src)
	if pair == nil {
		return
	}
	arr := firstChildOfType(pair, "array")
	if arr == nil {
		if unknown != nil {
			*unknown = true
		}
		return
	}
	for i := 0; i < int(arr.NamedChildCount()); i++ {
		el := arr.NamedChild(i)
		switch el.Type() {
		case "identifier":
			if ref := resolveLocal(rel, fi, text(el, src)); ref.key() != "" {
				out[ref.key()] = true
			}
		case "call_expression":
			if prop != "imports" {
				if unknown != nil {
					*unknown = true
				}
				continue
			}
			if ref := forwardRefModule(rel, fi, el, src); ref.key() != "" {
				out[ref.key()] = true
				continue
			}
			if unknown != nil {
				*unknown = true
			}
		case "object":
			if prop != "providers" {
				if unknown != nil {
					*unknown = true
				}
				continue
			}
			if provider := providerToken(rel, fi, el, src); provider.key() != "" {
				out[provider.key()] = true
			}
		default:
			if unknown != nil {
				*unknown = true
			}
		}
	}
}

func forwardRefModule(rel string, fi *fileInfo, call *sitter.Node, src []byte) symbolRef {
	fn := call.ChildByFieldName("function")
	if fn == nil || text(fn, src) != "forwardRef" {
		return symbolRef{}
	}
	args := firstChildOfType(call, "arguments")
	if args == nil {
		return symbolRef{}
	}
	// The canonical form is forwardRef(() => Module). Nest projects also use
	// forwardRef(function ref() { return Module; }). Read the callback's return
	// expression and accept only a bare module identifier. A member expression,
	// ternary, call, or non-bare block body is ambiguous, so bail rather than
	// guess.
	var name string
	if arrow := firstDirectChildOfType(args, "arrow_function"); arrow != nil {
		name = bareIdentifierName(arrow.ChildByFieldName("body"), src)
	}
	if name == "" {
		if fnExpr := firstDirectChildOfType(args, "function_expression"); fnExpr != nil {
			name = functionReturnIdentifierName(fnExpr, src)
		}
	}
	if name == "" {
		return symbolRef{}
	}
	return resolveLocal(rel, fi, name)
}

func objectPair(obj *sitter.Node, prop string, src []byte) *sitter.Node {
	for i := 0; i < int(obj.NamedChildCount()); i++ {
		pair := obj.NamedChild(i)
		if pair.Type() != "pair" {
			continue
		}
		key := pair.NamedChild(0)
		if key != nil && text(key, src) == prop {
			return pair
		}
	}
	return nil
}

func providerToken(rel string, fi *fileInfo, obj *sitter.Node, src []byte) symbolRef {
	pair := objectPair(obj, "provide", src)
	if pair == nil {
		return symbolRef{}
	}
	for i := 1; i < int(pair.NamedChildCount()); i++ {
		ch := pair.NamedChild(i)
		if ch.Type() == "identifier" {
			return resolveLocal(rel, fi, text(ch, src))
		}
	}
	return symbolRef{}
}
