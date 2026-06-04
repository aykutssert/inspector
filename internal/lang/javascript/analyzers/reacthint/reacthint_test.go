package reacthint

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/aykutssert/inspector/internal/core"
)

// parseSrc writes src to a temp file with ext and runs the detectors.
func parseSrc(t *testing.T, ext, src string) []string {
	t.Helper()
	fs := parseFindings(t, ext, src)
	var ids []string
	for _, f := range fs {
		ids = append(ids, f.RuleID)
	}
	return ids
}

func parseFindings(t *testing.T, ext, src string) []core.Finding {
	t.Helper()
	dir := t.TempDir()
	abs := filepath.Join(dir, "f"+ext)
	if err := os.WriteFile(abs, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	fs, err := scanFile(abs, "f"+ext)
	if err != nil {
		t.Fatalf("scanFile: %v", err)
	}
	return fs
}

func has(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

func countID(ids []string, id string) int {
	n := 0
	for _, x := range ids {
		if x == id {
			n++
		}
	}
	return n
}

func TestDerivedStateFromProp(t *testing.T) {
	src := `function C(props) {
  const [v, setV] = useState(props.value);
  return null;
}`
	if ids := parseSrc(t, ".tsx", src); !has(ids, "no-derived-state") {
		t.Fatalf("expected no-derived-state, got %v", ids)
	}
}

func TestDerivedStateFromCall(t *testing.T) {
	src := `function C(props) {
  const [v, setV] = useState(compute(props));
  return null;
}`
	if ids := parseSrc(t, ".tsx", src); !has(ids, "no-derived-state") {
		t.Fatalf("expected no-derived-state for computed init, got %v", ids)
	}
}

func TestStateFromLiteralNotFlagged(t *testing.T) {
	src := `function C() {
  const [v, setV] = useState(0);
  const [s, setS] = useState("");
  return null;
}`
	if ids := parseSrc(t, ".tsx", src); has(ids, "no-derived-state") {
		t.Fatalf("literal initializer should not be flagged, got %v", ids)
	}
}

func TestLazyInitNotFlagged(t *testing.T) {
	src := `function C(props) {
  const [v, setV] = useState(() => compute(props));
  return null;
}`
	if ids := parseSrc(t, ".tsx", src); has(ids, "no-derived-state") {
		t.Fatalf("lazy initializer should not be flagged, got %v", ids)
	}
}

func TestSetStateInEffectNoDeps(t *testing.T) {
	src := `function C() {
  const [v, setV] = useState(0);
  useEffect(() => {
    setV(1);
  });
  return null;
}`
	if ids := parseSrc(t, ".tsx", src); !has(ids, "setstate-in-effect-without-deps") {
		t.Fatalf("expected setstate-in-effect-without-deps, got %v", ids)
	}
}

func TestEffectWithDepsNotFlagged(t *testing.T) {
	src := `function C() {
  const [v, setV] = useState(0);
  useEffect(() => {
    setV(1);
  }, []);
  return null;
}`
	if ids := parseSrc(t, ".tsx", src); has(ids, "setstate-in-effect-without-deps") {
		t.Fatalf("effect with dep array should not be flagged, got %v", ids)
	}
}

func TestPreferUseReducer(t *testing.T) {
	src := `function Form() {
  const [a, setA] = useState(0);
  const [b, setB] = useState(0);
  const [c, setC] = useState(0);
  const [d, setD] = useState(0);
  return null;
}`
	if ids := parseSrc(t, ".tsx", src); !has(ids, "prefer-use-reducer") {
		t.Fatalf("expected prefer-use-reducer, got %v", ids)
	}
}

func TestFewUseStateNotFlagged(t *testing.T) {
	src := `function Form() {
  const [a, setA] = useState(0);
  const [b, setB] = useState(0);
  return null;
}`
	if ids := parseSrc(t, ".tsx", src); has(ids, "prefer-use-reducer") {
		t.Fatalf("two useState should not suggest useReducer, got %v", ids)
	}
}

func TestGodComponent(t *testing.T) {
	src := `function Dashboard({
  account,
  user,
  plan,
  filters,
  metrics,
  alerts,
  theme,
  permissions,
  locale,
}) {
  const [tab, setTab] = useState("overview");
  const [query, setQuery] = useState("");
  const [sort, setSort] = useState("name");
  const [selected, setSelected] = useState(null);
  const [expanded, setExpanded] = useState(false);
` + strings.Repeat("  const value = compute(metrics);\n", 96) + `  return <section>{tab}{query}{sort}{selected}{expanded}</section>;
}`
	if ids := parseSrc(t, ".tsx", src); !has(ids, "god-component") {
		t.Fatalf("expected god-component, got %v", ids)
	}
}

func TestGodComponentCountsTypedProps(t *testing.T) {
	src := `type Props = {
  a: string; b: string; c: string; d: string;
  e: string; f: string; g: string; h: string;
  i: string; j: string; k: string; l: string;
};
const Dashboard = ({ a, b, c, d, e, f, g, h, i, j, k, l }: Props) => {
  return <section>{a}{b}{c}{d}{e}{f}{g}{h}{i}{j}{k}{l}</section>;
};`
	if ids := parseSrc(t, ".tsx", src); !has(ids, "god-component") {
		t.Fatalf("expected typed prop-heavy component to be flagged, got %v", ids)
	}
}

func TestSmallComponentNotGodComponent(t *testing.T) {
	src := `function Card({ title, subtitle }) {
  const [expanded, setExpanded] = useState(false);
  return <article>{title}{subtitle}{expanded}</article>;
}`
	if ids := parseSrc(t, ".tsx", src); has(ids, "god-component") {
		t.Fatalf("small component should not be flagged, got %v", ids)
	}
}

func TestEmDashInJSX(t *testing.T) {
	src := "function C() { return <p>Fast — and secure</p>; }"
	if ids := parseSrc(t, ".tsx", src); !has(ids, "em-dash-in-jsx-text") {
		t.Fatalf("expected em-dash-in-jsx-text, got %v", ids)
	}
}

func TestPlainHyphenNotFlagged(t *testing.T) {
	src := "function C() { return <p>well-tested code</p>; }"
	if ids := parseSrc(t, ".tsx", src); has(ids, "em-dash-in-jsx-text") {
		t.Fatalf("regular hyphen should not be flagged, got %v", ids)
	}
}

func TestRenderTimeAllocationInJSX(t *testing.T) {
	src := `function Card({ title }) {
  return <Panel options={{ compact: true }} today={new Date()} matcher={/admin/}>{title}</Panel>;
}`
	if ids := parseSrc(t, ".tsx", src); !has(ids, "render-time-allocation") {
		t.Fatalf("expected render-time-allocation, got %v", ids)
	}
}

// The {{ __html }} wrapper on dangerouslySetInnerHTML is required by the React
// API, not an avoidable render allocation. The real risk is XSS (reported by the
// security layer), so the allocation hint must stay silent here and not bury it.
func TestDangerouslySetInnerHTMLNotRenderAllocation(t *testing.T) {
	src := `function View({ html }) {
  return <div dangerouslySetInnerHTML={{ __html: html }} />;
}`
	if ids := parseSrc(t, ".tsx", src); has(ids, "render-time-allocation") {
		t.Fatalf("dangerouslySetInnerHTML wrapper must not be flagged as render-time-allocation, got %v", ids)
	}
}

func TestStableJSXValuesNotFlagged(t *testing.T) {
	src := `const options = { compact: true };
const matcher = /admin/;
function Card({ title, today }) {
  return <Panel options={options} today={today} matcher={matcher}>{title}</Panel>;
}`
	if ids := parseSrc(t, ".tsx", src); has(ids, "render-time-allocation") {
		t.Fatalf("stable values should not be flagged, got %v", ids)
	}
}

func TestMemoizedChildUnstableInlineProps(t *testing.T) {
	src := `import React from "react";

const Child = React.memo(function Child() {
  return null;
});

function Parent() {
  return <Child onSave={() => save()} options={{ dense: true }} items={[1, 2]} cb={handler.bind(null)} value={new Date()} />;
}`
	ids := parseSrc(t, ".tsx", src)
	if !has(ids, "memoized-child-unstable-prop") {
		t.Fatalf("expected memoized-child-unstable-prop, got %v", ids)
	}
	if has(ids, "render-time-allocation") {
		t.Fatalf("memoized child unstable props should not also emit render-time-allocation, got %v", ids)
	}
}

func TestMemoizedChildLocalUnstableIdentifierProps(t *testing.T) {
	src := `import { memo } from "react";

const Child = memo(function Child() {
  return null;
});

function Parent() {
  const options = { dense: true };
  const onSave = () => save();
  return <Child options={options} onSave={onSave} />;
}`
	if ids := parseSrc(t, ".tsx", src); !has(ids, "memoized-child-unstable-prop") {
		t.Fatalf("expected local unstable prop to be flagged, got %v", ids)
	}
}

func TestMemoizedChildStableHookPropsNotFlagged(t *testing.T) {
	src := `import { memo, useCallback, useMemo, useRef } from "react";

const Child = memo(function Child() {
  return null;
});

function Parent() {
  const options = useMemo(() => ({ dense: true }), []);
  const onSave = useCallback(() => save(), []);
  const rootRef = useRef(null);
  return <Child options={options} onSave={onSave} rootRef={rootRef} count={1} label="ok" />;
}`
	if ids := parseSrc(t, ".tsx", src); has(ids, "memoized-child-unstable-prop") {
		t.Fatalf("stable memo/useCallback/useRef props should not be flagged, got %v", ids)
	}
}

func TestNonMemoizedChildStillGetsGenericRenderAllocation(t *testing.T) {
	src := `function Child() {
  return null;
}

function Parent() {
  return <Child options={{ dense: true }} />;
}`
	ids := parseSrc(t, ".tsx", src)
	if has(ids, "memoized-child-unstable-prop") {
		t.Fatalf("non-memoized child should not get memo-specific finding, got %v", ids)
	}
	if !has(ids, "render-time-allocation") {
		t.Fatalf("non-memoized inline object should still get generic render allocation, got %v", ids)
	}
}

func TestImportedMemoizedChildUnstableProp(t *testing.T) {
	root := writeReactProject(t, map[string]string{
		"Child.tsx": `import { memo } from "react";

export const Child = memo(function Child() {
  return null;
});
`,
		"Parent.tsx": `import { Child } from "./Child";

export function Parent() {
  return <Child onSave={() => save()} />;
}
`,
	})
	findings := scanReactProject(t, root)
	var ids []string
	for _, f := range findings {
		ids = append(ids, f.RuleID)
	}
	if countID(ids, "memoized-child-unstable-prop") != 1 {
		t.Fatalf("expected one imported memoized child finding, got ids=%v findings=%#v", ids, findings)
	}
}

func TestBarrelMemoizedChildUnstableProp(t *testing.T) {
	root := writeReactProject(t, map[string]string{
		"components/Child.tsx": `import { memo } from "react";

export const Child = memo(function Child() {
  return null;
});
`,
		"components/index.ts": `export { Child } from "./Child";`,
		"Parent.tsx": `import { Child } from "./components";

export function Parent() {
  return <Child options={{ dense: true }} />;
}
`,
	})
	findings := scanReactProject(t, root)
	var ids []string
	for _, f := range findings {
		ids = append(ids, f.RuleID)
	}
	if countID(ids, "memoized-child-unstable-prop") != 1 {
		t.Fatalf("expected one barrel memoized child finding, got ids=%v findings=%#v", ids, findings)
	}
}

func TestDefaultExportMemoizedChildUnstableProp(t *testing.T) {
	root := writeReactProject(t, map[string]string{
		"Child.tsx": `import { memo } from "react";

export default memo(function Child() {
  return null;
});
`,
		"Parent.tsx": `import Child from "./Child";

export function Parent() {
  return <Child onSave={() => save()} />;
}
`,
	})
	findings := scanReactProject(t, root)
	var ids []string
	for _, f := range findings {
		ids = append(ids, f.RuleID)
	}
	if countID(ids, "memoized-child-unstable-prop") != 1 {
		t.Fatalf("expected one default memoized child finding, got ids=%v findings=%#v", ids, findings)
	}
}

func TestPathAliasMemoizedChildUnstableProp(t *testing.T) {
	root := writeReactProject(t, map[string]string{
		"tsconfig.json": `{
  // React projects commonly import components through aliases.
  "compilerOptions": {
    "baseUrl": ".",
    "paths": {
      "@ui/*": ["src/components/*"],
    },
  },
}`,
		"src/components/Child.tsx": `import { memo } from "react";

export const Child = memo(function Child() {
  return null;
});
`,
		"src/Parent.tsx": `import { Child } from "@ui/Child";

export function Parent() {
  return <Child options={{ dense: true }} />;
}
`,
	})
	findings := scanReactProject(t, root)
	var ids []string
	for _, f := range findings {
		ids = append(ids, f.RuleID)
	}
	if countID(ids, "memoized-child-unstable-prop") != 1 {
		t.Fatalf("expected one alias memoized child finding, got ids=%v findings=%#v", ids, findings)
	}
}

// Plain .ts (no JSX grammar) must still run the non-JSX detectors without error.
func TestPlainTSParses(t *testing.T) {
	src := `export function useThing(props: { value: number }) {
  const [v, setV] = useState(props.value);
  return v;
}`
	if ids := parseSrc(t, ".ts", src); !has(ids, "no-derived-state") {
		t.Fatalf("expected hint in .ts, got %v", ids)
	}
}

func writeReactProject(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func scanReactProject(t *testing.T, root string) []core.Finding {
	t.Helper()
	var files []string
	if err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	got, err := New().Scan(core.ProjectContext{Root: root, Files: files, Languages: []string{"javascript"}})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestCallbackPropRenderedNotInvoked(t *testing.T) {
	src := `function Button({ onClick }) {
  return <span>{onClick}</span>;
}`
	if ids := parseSrc(t, ".tsx", src); !has(ids, "callback-prop-not-invoked") {
		t.Fatalf("expected callback-prop-not-invoked, got %v", ids)
	}
}

func TestCallbackPropRenamedPairRendered(t *testing.T) {
	src := `function Row({ onSave: save }) {
  return <div>{save}</div>;
}`
	if ids := parseSrc(t, ".tsx", src); !has(ids, "callback-prop-not-invoked") {
		t.Fatalf("expected flag for renamed callback prop, got %v", ids)
	}
}

func TestCallbackPropInterpolated(t *testing.T) {
	src := "function L({ onChange }) {\n" +
		"  const label = `value=${onChange}`;\n" +
		"  return <p>{label}</p>;\n" +
		"}"
	if ids := parseSrc(t, ".tsx", src); !has(ids, "callback-prop-not-invoked") {
		t.Fatalf("expected flag for interpolated callback prop, got %v", ids)
	}
}

func TestCallbackPropForwardedNotFlagged(t *testing.T) {
	src := `function Button({ onClick }) {
  return <button onClick={onClick}>ok</button>;
}`
	if ids := parseSrc(t, ".tsx", src); has(ids, "callback-prop-not-invoked") {
		t.Fatalf("forwarded handler should not be flagged, got %v", ids)
	}
}

func TestCallbackPropInvokedNotFlagged(t *testing.T) {
	src := `function Form({ onSubmit }) {
  const submit = () => onSubmit();
  return <form onSubmit={submit}>x</form>;
}`
	if ids := parseSrc(t, ".tsx", src); has(ids, "callback-prop-not-invoked") {
		t.Fatalf("invoked callback should not be flagged, got %v", ids)
	}
}

func TestCallbackPropAliasedNotFlagged(t *testing.T) {
	src := `function W({ onLoad }) {
  const cb = onLoad;
  cb();
  return <div>x</div>;
}`
	if ids := parseSrc(t, ".tsx", src); has(ids, "callback-prop-not-invoked") {
		t.Fatalf("aliased callback should not be flagged, got %v", ids)
	}
}

func TestCallbackPropPassedAsArgNotFlagged(t *testing.T) {
	src := `function W({ onMount }) {
  useEffect(onMount, []);
  return <div>x</div>;
}`
	if ids := parseSrc(t, ".tsx", src); has(ids, "callback-prop-not-invoked") {
		t.Fatalf("callback passed as argument should not be flagged, got %v", ids)
	}
}

func TestCallbackPropGuardNotFlagged(t *testing.T) {
	src := `function W({ onClose }) {
  return <div>{onClose && <button onClick={onClose}>x</button>}</div>;
}`
	if ids := parseSrc(t, ".tsx", src); has(ids, "callback-prop-not-invoked") {
		t.Fatalf("guarded callback should not be flagged, got %v", ids)
	}
}

func TestCallbackPropRenderedButAlsoCalledNotFlagged(t *testing.T) {
	src := `function W({ onPing }) {
  onPing();
  return <span>{onPing}</span>;
}`
	if ids := parseSrc(t, ".tsx", src); has(ids, "callback-prop-not-invoked") {
		t.Fatalf("a callback invoked anywhere honors the contract, got %v", ids)
	}
}

func TestNonCallbackPropRenderedNotFlagged(t *testing.T) {
	src := `function W({ title }) {
  return <h1>{title}</h1>;
}`
	if ids := parseSrc(t, ".tsx", src); has(ids, "callback-prop-not-invoked") {
		t.Fatalf("non-callback prop should not be flagged, got %v", ids)
	}
}

func TestComponentSplitting(t *testing.T) {
	src := `export function CompA() {
` + strings.Repeat("  const x = 1;\n", 160) + `  return <div>{x}</div>;
}

export function CompB() {
` + strings.Repeat("  const y = 2;\n", 160) + `  return <div>{y}</div>;
}`

	ids := parseSrc(t, ".tsx", src)
	if !has(ids, "component-splitting") {
		t.Fatalf("expected component-splitting hint, got %v", ids)
	}
}

func TestComponentSplittingNotFlaggedIfOneLarge(t *testing.T) {
	src := `export function CompA() {
` + strings.Repeat("  const x = 1;\n", 160) + `  return <div>{x}</div>;
}

export function CompB() {
  return <div>small</div>;
}`

	ids := parseSrc(t, ".tsx", src)
	if has(ids, "component-splitting") {
		t.Fatalf("should not suggest component-splitting if only one large, got %v", ids)
	}
}
