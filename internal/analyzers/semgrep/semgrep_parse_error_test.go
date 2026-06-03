package semgrep

import "testing"

func TestIsTypeScriptFile(t *testing.T) {
	ts := []string{"a.ts", "b.tsx", "c.mts", "d.cts", "src/Store.TS", "x.TSX"}
	for _, f := range ts {
		if !isTypeScriptFile(f) {
			t.Fatalf("%s should be a TypeScript file", f)
		}
	}
	notTS := []string{"a.js", "b.jsx", "c.svelte", "d.py", "e.json", "f.mjs", "noext"}
	for _, f := range notTS {
		if isTypeScriptFile(f) {
			t.Fatalf("%s must not be treated as TypeScript", f)
		}
	}
}
