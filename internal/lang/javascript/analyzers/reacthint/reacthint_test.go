package reacthint

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/aykutssert/scout/internal/core"
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

func TestUseEffectMissingCleanup(t *testing.T) {
	src := `function Feed() {
  useEffect(() => {
    window.addEventListener("resize", refresh);
  }, []);
  return null;
}`
	if ids := parseSrc(t, ".tsx", src); !has(ids, "react-useeffect-missing-cleanup") {
		t.Fatalf("expected react-useeffect-missing-cleanup, got %v", ids)
	}
}

func TestUseEffectCleanupNotFlagged(t *testing.T) {
	src := `function Feed() {
  useEffect(() => {
    const timer = setInterval(refresh, 1000);
    return () => clearInterval(timer);
  }, []);
  return null;
}`
	if ids := parseSrc(t, ".tsx", src); has(ids, "react-useeffect-missing-cleanup") {
		t.Fatalf("effect cleanup must stay quiet, got %v", ids)
	}
}

func TestNestedResourceRegistrationNotFlagged(t *testing.T) {
	src := `function Feed() {
  useEffect(() => {
    const start = () => store.subscribe(refresh);
    start();
  }, []);
  return null;
}`
	if ids := parseSrc(t, ".tsx", src); has(ids, "react-useeffect-missing-cleanup") {
		t.Fatalf("nested function registration is not effect setup, got %v", ids)
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

func TestEffectPerformanceHintsSkipTests(t *testing.T) {
	src := `
		function Provider() {
			useEffect(() => {
				window.addEventListener("resize", resize);
			}, []);
			return <ThemeContext.Provider value={{ dark: true }} />;
		}
	`
	dir := t.TempDir()
	abs := filepath.Join(dir, "provider.test.tsx")
	if err := os.WriteFile(abs, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	findings, err := scanFile(abs, "src/__tests__/provider.test.tsx")
	if err != nil {
		t.Fatal(err)
	}
	for _, finding := range findings {
		if finding.RuleID == "react-useeffect-missing-cleanup" {
			t.Fatalf("production performance hints must skip tests, got %#v", findings)
		}
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

func TestDeepPropDrilling(t *testing.T) {
	src := `
		function Parent() {
			return <CompB user="alice" />;
		}
		
		function CompB({ user }) { // drilling node
			return <CompC user={user} />;
		}
		
		function CompC({ user }) { // drilling node
			return <CompD user={user} />;
		}
		
		function CompD({ user }) { // destination
			return <div>{user}</div>;
		}
	`
	ids := parseSrc(t, ".tsx", src)
	if !has(ids, "deep-prop-drilling") {
		t.Fatalf("expected deep-prop-drilling violation, got %v", ids)
	}
}

func TestDeepPropDrillingSafe(t *testing.T) {
	src := `
		function Parent() {
			return <CompB user="alice" />;
		}
		
		function CompB({ user }) { // safe: used locally!
			console.log(user);
			return <CompC user={user} />;
		}
		
		function CompC({ user }) { // destination
			return <div>{user}</div>;
		}
	`
	ids := parseSrc(t, ".tsx", src)
	if has(ids, "deep-prop-drilling") {
		t.Fatalf("did not expect deep-prop-drilling violation, got %v", ids)
	}
}

func TestUseEffectFetchSuggestQuery(t *testing.T) {
	src := `
		function Comp() {
			useEffect(() => {
				fetch('/api/data').then(res => res.json());
			}, []);

			useEffect(() => {
				axios.get('/api/info');
			}, []);

			return null;
		}
	`
	ids := parseSrc(t, ".tsx", src)
	if countID(ids, "react-useeffect-fetch-suggest-query") != 2 {
		t.Fatalf("expected 2 react-useeffect-fetch-suggest-query findings, got %v", ids)
	}
}

func TestUseEffectFetchSuggestQuerySafe(t *testing.T) {
	src := `
		function Comp() {
			// Safe: standard useEffect, no fetch or axios
			useEffect(() => {
				console.log("mounted");
			}, []);
			return null;
		}
	`
	ids := parseSrc(t, ".tsx", src)
	if has(ids, "react-useeffect-fetch-suggest-query") {
		t.Fatalf("did not expect react-useeffect-fetch-suggest-query finding, got %v", ids)
	}
}

func TestNoRenderInRender(t *testing.T) {
	src := `
		function Parent() {
			return (
				<div>
					{Child()}
					{ListItem({ item: "hello" })}
					{renderHeader()}
					{renderItem("test")}
					{this.renderFooter()}
				</div>
			);
		}
	`
	ids := parseSrc(t, ".tsx", src)
	if countID(ids, "no-render-in-render") != 5 {
		t.Fatalf("expected 5 no-render-in-render findings, got %v", ids)
	}
}

func TestNoRenderInRenderSafe(t *testing.T) {
	src := `
		function Parent() {
			return (
				<div>
					<Child />
					<ListItem item="hello" />
					{Boolean(true)}
					{Date()}
					{Math.random()}
					{renderHeader}
				</div>
			);
		}
	`
	ids := parseSrc(t, ".tsx", src)
	if has(ids, "no-render-in-render") {
		t.Fatalf("did not expect any no-render-in-render findings, got %v", ids)
	}
}

func TestStableEmptyFallback(t *testing.T) {
	src := `
		function Parent(props) {
			const items = props.items || [];
			const options = props.options ?? {};
			return null;
		}
	`
	ids := parseSrc(t, ".tsx", src)
	if countID(ids, "prefer-stable-empty-fallback") != 2 {
		t.Fatalf("expected 2 prefer-stable-empty-fallback findings, got %v", ids)
	}
}

func TestStableEmptyFallbackSafe(t *testing.T) {
	src := `
		const EMPTY_ARRAY = [];
		const EMPTY_OBJECT = {};
		const fallbackAtModule = someVal || []; // Safe: outside component

		function Parent({ items = [], options = {} }) { // Safe: default parameter
			const { data = {} } = props; // Safe: default destructuring
			const list = props.list || EMPTY_ARRAY; // Safe: stable variable
			const config = props.config || { compact: true }; // Safe: non-empty object
			const nums = props.nums || [1, 2]; // Safe: non-empty array
			const str = props.name || "default"; // Safe: string fallback
			return null;
		}
	`
	ids := parseSrc(t, ".tsx", src)
	if has(ids, "prefer-stable-empty-fallback") {
		t.Fatalf("did not expect any prefer-stable-empty-fallback findings, got %v", ids)
	}
}

// ─── no-initialize-state (#85) ─────────────────────────────────────────────

func TestInitializeState_Fires(t *testing.T) {
	src := `function C() {
  const [v, setV] = useState(null);
  useEffect(() => { setV(computeInitial()); }, []);
  return null;
}`
	if ids := parseSrc(t, ".tsx", src); !has(ids, "no-initialize-state") {
		t.Fatalf("expected no-initialize-state, got %v", ids)
	}
}

func TestInitializeState_ConciseArrow(t *testing.T) {
	src := `function C() {
  const [v, setV] = useState(null);
  useEffect(() => setV(42), []);
  return null;
}`
	if ids := parseSrc(t, ".tsx", src); !has(ids, "no-initialize-state") {
		t.Fatalf("expected no-initialize-state for concise arrow, got %v", ids)
	}
}

func TestInitializeState_WithFetch_NotFlagged(t *testing.T) {
	src := `function C() {
  const [data, setData] = useState(null);
  useEffect(() => {
    fetch("/api").then(r => r.json()).then(setData);
  }, []);
  return null;
}`
	if ids := parseSrc(t, ".tsx", src); has(ids, "no-initialize-state") {
		t.Fatalf("no-initialize-state should not fire when body has fetch, got %v", ids)
	}
}

func TestInitializeState_NonEmptyDeps_NotFlagged(t *testing.T) {
	src := `function C({ id }) {
  const [v, setV] = useState(null);
  useEffect(() => { setV(id * 2); }, [id]);
  return null;
}`
	if ids := parseSrc(t, ".tsx", src); has(ids, "no-initialize-state") {
		t.Fatalf("no-initialize-state should not fire with non-empty deps, got %v", ids)
	}
}

// ─── no-mutable-in-deps (#86) ──────────────────────────────────────────────

func TestMutableInDeps_RefCurrent(t *testing.T) {
	src := `function C({ inputRef }) {
  useEffect(() => {
    inputRef.current.focus();
  }, [inputRef.current]);
  return null;
}`
	if ids := parseSrc(t, ".tsx", src); !has(ids, "no-mutable-in-deps") {
		t.Fatalf("expected no-mutable-in-deps for ref.current, got %v", ids)
	}
}

func TestMutableInDeps_LocationPathname(t *testing.T) {
	src := `function C() {
  useEffect(() => {
    console.log("path changed");
  }, [location.pathname]);
  return null;
}`
	if ids := parseSrc(t, ".tsx", src); !has(ids, "no-mutable-in-deps") {
		t.Fatalf("expected no-mutable-in-deps for location.pathname, got %v", ids)
	}
}

func TestMutableInDeps_StateVar_NotFlagged(t *testing.T) {
	src := `function C() {
  const [count, setCount] = useState(0);
  useEffect(() => { doSomething(count); }, [count]);
  return null;
}`
	if ids := parseSrc(t, ".tsx", src); has(ids, "no-mutable-in-deps") {
		t.Fatalf("no-mutable-in-deps should not fire for reactive state, got %v", ids)
	}
}

// ─── prefer-use-sync-external-store (#92) ──────────────────────────────────

func TestPreferUseSyncExternalStore_Fires(t *testing.T) {
	src := `function C() {
  const [state, setState] = useState(store.getState());
  useEffect(() => {
    const unsub = store.subscribe(() => setState(store.getState()));
    return () => unsub();
  }, []);
  return null;
}`
	if ids := parseSrc(t, ".tsx", src); !has(ids, "prefer-use-sync-external-store") {
		t.Fatalf("expected prefer-use-sync-external-store, got %v", ids)
	}
}

func TestPreferUseSyncExternalStore_NoCleanup_NotFlagged(t *testing.T) {
	src := `function C() {
  useEffect(() => {
    store.subscribe(() => doSomething());
  }, []);
  return null;
}`
	if ids := parseSrc(t, ".tsx", src); has(ids, "prefer-use-sync-external-store") {
		t.Fatalf("prefer-use-sync-external-store should not fire without cleanup, got %v", ids)
	}
}

// ─── no-cascading-set-state (#78) ──────────────────────────────────────────

func TestCascadingSetState_Fires(t *testing.T) {
	src := `function C() {
  const [a, setA] = useState(null);
  const [b, setB] = useState(null);
  useEffect(() => {
    setA(1);
    setB(2);
  }, [dep]);
  return null;
}`
	if ids := parseSrc(t, ".tsx", src); !has(ids, "no-cascading-set-state") {
		t.Fatalf("expected no-cascading-set-state, got %v", ids)
	}
}

func TestCascadingSetState_SingleSetter_NotFlagged(t *testing.T) {
	src := `function C() {
  const [v, setV] = useState(0);
  useEffect(() => { setV(42); }, [dep]);
  return null;
}`
	if ids := parseSrc(t, ".tsx", src); has(ids, "no-cascading-set-state") {
		t.Fatalf("no-cascading-set-state should not fire for single setter, got %v", ids)
	}
}

// ─── no-self-updating-effect (#91) ─────────────────────────────────────────

func TestSelfUpdatingEffect_Fires(t *testing.T) {
	src := `function C() {
  const [count, setCount] = useState(0);
  useEffect(() => {
    setCount(count + 1);
  }, [count]);
  return null;
}`
	if ids := parseSrc(t, ".tsx", src); !has(ids, "no-self-updating-effect") {
		t.Fatalf("expected no-self-updating-effect, got %v", ids)
	}
}

func TestSelfUpdatingEffect_FunctionalUpdate_NotFlagged(t *testing.T) {
	// Using functional update removes the state from deps — correct pattern.
	src := `function C() {
  const [count, setCount] = useState(0);
  useEffect(() => {
    setCount(prev => prev + 1);
  }, []);
  return null;
}`
	if ids := parseSrc(t, ".tsx", src); has(ids, "no-self-updating-effect") {
		t.Fatalf("no-self-updating-effect should not fire for functional update, got %v", ids)
	}
}

func TestSelfUpdatingEffect_DifferentSetter_NotFlagged(t *testing.T) {
	src := `function C() {
  const [a, setA] = useState(0);
  const [b, setB] = useState(0);
  useEffect(() => {
    setB(a + 1); // updates b from a — a is in deps, but setA is not called
  }, [a]);
  return null;
}`
	if ids := parseSrc(t, ".tsx", src); has(ids, "no-self-updating-effect") {
		t.Fatalf("no-self-updating-effect should not fire when setter != dep var, got %v", ids)
	}
}

// ─── no-effect-event-in-deps (#82) ───────────────────────────────────────────

func TestEffectEventInDeps_Fires(t *testing.T) {
	src := `function C() {
  const handleSave = useEffectEvent(() => { save(value) });
  useEffect(() => {
    handleSave();
  }, [handleSave]);
  return null;
}`
	if ids := parseSrc(t, ".tsx", src); !has(ids, "no-effect-event-in-deps") {
		t.Fatalf("expected no-effect-event-in-deps, got %v", ids)
	}
}

func TestEffectEventInDeps_NotInDeps_NotFlagged(t *testing.T) {
	src := `function C() {
  const handleSave = useEffectEvent(() => { save(value) });
  useEffect(() => {
    handleSave();
  }, []);
  return null;
}`
	if ids := parseSrc(t, ".tsx", src); has(ids, "no-effect-event-in-deps") {
		t.Fatalf("no-effect-event-in-deps should not fire when fn not in deps, got %v", ids)
	}
}

// ─── no-pass-data-to-parent (#87) ────────────────────────────────────────────

func TestPassDataToParent_Fires(t *testing.T) {
	src := `function Child({ onDataChange }) {
  const [data, setData] = useState(null);
  useEffect(() => {
    onDataChange(data);
  }, [data, onDataChange]);
  return null;
}`
	if ids := parseSrc(t, ".tsx", src); !has(ids, "no-pass-data-to-parent") {
		t.Fatalf("expected no-pass-data-to-parent, got %v", ids)
	}
}

func TestPassDataToParent_NoStateArg_NotFlagged(t *testing.T) {
	src := `function Child({ onSubmit }) {
  const [data, setData] = useState(null);
  useEffect(() => {
    onSubmit("static-value");
  }, [onSubmit]);
  return null;
}`
	if ids := parseSrc(t, ".tsx", src); has(ids, "no-pass-data-to-parent") {
		t.Fatalf("no-pass-data-to-parent should not fire when arg is not state, got %v", ids)
	}
}

// ─── no-adjust-state-on-prop-change (#77) ────────────────────────────────────

func TestAdjustStateOnPropChange_Fires(t *testing.T) {
	src := `function C({ items }) {
  const [filtered, setFiltered] = useState([]);
  useEffect(() => {
    setFiltered(items.filter(x => x.active));
  }, [items]);
  return null;
}`
	if ids := parseSrc(t, ".tsx", src); !has(ids, "no-adjust-state-on-prop-change") {
		t.Fatalf("expected no-adjust-state-on-prop-change, got %v", ids)
	}
}

func TestAdjustStateOnPropChange_StateDep_NotFlagged(t *testing.T) {
	src := `function C() {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState([]);
  useEffect(() => {
    setResults(search(query));
  }, [query]);
  return null;
}`
	// dep is a state var — different smell, should NOT fire this rule
	if ids := parseSrc(t, ".tsx", src); has(ids, "no-adjust-state-on-prop-change") {
		t.Fatalf("no-adjust-state-on-prop-change should not fire when dep is state, got %v", ids)
	}
}

func TestAdjustStateOnPropChange_EmptyDeps_NotFlagged(t *testing.T) {
	src := `function C({ initial }) {
  const [val, setVal] = useState(0);
  useEffect(() => {
    setVal(initial);
  }, []);
  return null;
}`
	// Empty deps covered by no-initialize-state, not this rule
	if ids := parseSrc(t, ".tsx", src); has(ids, "no-adjust-state-on-prop-change") {
		t.Fatalf("no-adjust-state-on-prop-change should not fire on empty deps, got %v", ids)
	}
}

// ─── rendering-hydration-no-flicker (#49) ─────────────────────────────────────

func TestHydrationNoFlicker_Fires(t *testing.T) {
	src := `function App() {
  const [mounted, setMounted] = useState(false);
  useEffect(() => { setMounted(true) }, []);
  if (!mounted) return null;
  return <div />;
}`
	if ids := parseSrc(t, ".tsx", src); !has(ids, "rendering-hydration-no-flicker") {
		t.Fatalf("expected rendering-hydration-no-flicker, got %v", ids)
	}
}

func TestHydrationNoFlicker_NonBooleanInit_NotFlagged(t *testing.T) {
	src := `function App() {
  const [count, setCount] = useState(0);
  useEffect(() => { setCount(1) }, []);
  return null;
}`
	// setCount(1) — arg is "1" not "true", should not fire
	if ids := parseSrc(t, ".tsx", src); has(ids, "rendering-hydration-no-flicker") {
		t.Fatalf("rendering-hydration-no-flicker should not fire for non-true arg, got %v", ids)
	}
}

// ─── rerender-transitions-scroll (#53) ────────────────────────────────────────

func TestTransitionsScroll_Fires(t *testing.T) {
	src := `function C() {
  const [y, setY] = useState(0);
  return <div onScroll={e => setY(e.currentTarget.scrollTop)} />;
}`
	if ids := parseSrc(t, ".tsx", src); !has(ids, "rerender-transitions-scroll") {
		t.Fatalf("expected rerender-transitions-scroll, got %v", ids)
	}
}

func TestTransitionsScroll_NoSetter_NotFlagged(t *testing.T) {
	src := `function C() {
  return <div onScroll={e => console.log(e.target.scrollTop)} />;
}`
	if ids := parseSrc(t, ".tsx", src); has(ids, "rerender-transitions-scroll") {
		t.Fatalf("rerender-transitions-scroll should not fire without setState, got %v", ids)
	}
}

// ─── advanced-event-handler-refs (#76) ────────────────────────────────────────

func TestEventHandlerRefs_Fires(t *testing.T) {
	src := `function C({ handleKey }) {
  useEffect(() => {
    window.addEventListener('keydown', handleKey);
    return () => window.removeEventListener('keydown', handleKey);
  }, [handleKey]);
  return null;
}`
	if ids := parseSrc(t, ".tsx", src); !has(ids, "advanced-event-handler-refs") {
		t.Fatalf("expected advanced-event-handler-refs, got %v", ids)
	}
}

func TestEventHandlerRefs_NotInDeps_NotFlagged(t *testing.T) {
	src := `function C({ handleKey }) {
  const ref = useRef(handleKey);
  useEffect(() => {
    window.addEventListener('keydown', ref.current);
    return () => window.removeEventListener('keydown', ref.current);
  }, []);
  return null;
}`
	if ids := parseSrc(t, ".tsx", src); has(ids, "advanced-event-handler-refs") {
		t.Fatalf("advanced-event-handler-refs should not fire when handler not in deps, got %v", ids)
	}
}

func TestChainStateUpdates_Fires(t *testing.T) {
	src := `function C() {
  const [query, setQuery] = useState("");
  const [request, setRequest] = useState("");
  const [results, setResults] = useState([]);
  useEffect(() => {
    setRequest(query.trim());
  }, [query]);
  useEffect(() => {
    setResults(search(request));
  }, [request]);
  return null;
}`
	if ids := parseSrc(t, ".tsx", src); !has(ids, "no-chain-state-updates") {
		t.Fatalf("expected no-chain-state-updates, got %v", ids)
	}
}

func TestEffectChain_Fires(t *testing.T) {
	src := `function C() {
  const [a, setA] = useState(0);
  const [b, setB] = useState(0);
  const [c, setC] = useState(0);
  useEffect(() => { setB(a + 1); }, [a]);
  useEffect(() => { setC(b + 1); }, [b]);
  useEffect(() => { setA(c + 1); }, [c]);
  return null;
}`
	if ids := parseSrc(t, ".tsx", src); !has(ids, "no-effect-chain") {
		t.Fatalf("expected no-effect-chain, got %v", ids)
	}
}

func TestEffectEventHandler_Fires(t *testing.T) {
	src := `function Child({ onSave }) {
  const [submitted, setSubmitted] = useState(false);
  useEffect(() => {
    if (!submitted) return;
    onSave();
  }, [submitted, onSave]);
  return <button onClick={() => setSubmitted(true)}>Save</button>;
}`
	if ids := parseSrc(t, ".tsx", src); !has(ids, "no-effect-event-handler") {
		t.Fatalf("expected no-effect-event-handler, got %v", ids)
	}
}

func TestNoEventHandler_Fires(t *testing.T) {
	src := `function Child() {
  const [submitted, setSubmitted] = useState(false);
  useEffect(() => {
    if (!submitted) return;
    fetch('/api/save');
  }, [submitted]);
  return <button onClick={() => setSubmitted(true)}>Save</button>;
}`
	if ids := parseSrc(t, ".tsx", src); !has(ids, "no-event-handler") {
		t.Fatalf("expected no-event-handler, got %v", ids)
	}
}

func TestEventTriggerState_Fires(t *testing.T) {
	src := `function Child() {
  const [trigger, setTrigger] = useState(false);
  useEffect(() => {
    if (!trigger) return;
    sendBeacon();
  }, [trigger]);
  return <button onClick={() => setTrigger(true)}>Send</button>;
}`
	if ids := parseSrc(t, ".tsx", src); !has(ids, "no-event-trigger-state") {
		t.Fatalf("expected no-event-trigger-state, got %v", ids)
	}
}

func TestPassLiveStateToParent_Fires(t *testing.T) {
	src := `function Child({ onChange }) {
  const [value, setValue] = useState("");
  useEffect(() => {
    onChange(value);
  }, [value, onChange]);
  return <input onChange={e => setValue(e.target.value)} />;
}`
	if ids := parseSrc(t, ".tsx", src); !has(ids, "no-pass-live-state-to-parent") {
		t.Fatalf("expected no-pass-live-state-to-parent, got %v", ids)
	}
}

func TestPropCallbackInEffect_Fires(t *testing.T) {
	src := `function Child({ onReady }) {
  useEffect(() => {
    onReady();
  }, [onReady]);
  return null;
}`
	if ids := parseSrc(t, ".tsx", src); !has(ids, "no-prop-callback-in-effect") {
		t.Fatalf("expected no-prop-callback-in-effect, got %v", ids)
	}
}

func TestResetAllStateOnPropChange_Fires(t *testing.T) {
	src := `function Child({ userId }) {
  const [query, setQuery] = useState("");
  const [page, setPage] = useState(0);
  useEffect(() => {
    setQuery("");
    setPage(0);
  }, [userId]);
  return null;
}`
	if ids := parseSrc(t, ".tsx", src); !has(ids, "no-reset-all-state-on-prop-change") {
		t.Fatalf("expected no-reset-all-state-on-prop-change, got %v", ids)
	}
}

func TestHoistJSX_Fires(t *testing.T) {
	src := `function Card() {
  const icon = <svg><path d="M0 0h10v10z" /></svg>;
  return <div>{icon}</div>;
}`
	if ids := parseSrc(t, ".tsx", src); !has(ids, "rendering-hoist-jsx") {
		t.Fatalf("expected rendering-hoist-jsx, got %v", ids)
	}
}

func TestUseTransitionLoading_Fires(t *testing.T) {
	src := `function Form() {
  const [loading, setLoading] = useState(false);
  return <button onClick={async () => {
    setLoading(true);
    await save();
    setLoading(false);
  }}>Save</button>;
}`
	if ids := parseSrc(t, ".tsx", src); !has(ids, "rendering-usetransition-loading") {
		t.Fatalf("expected rendering-usetransition-loading, got %v", ids)
	}
}

func TestMemoBeforeEarlyReturn_Fires(t *testing.T) {
	src := `function List({ items }) {
  const visible = useMemo(() => items.slice(0, 5), [items]);
  if (!items.length) return null;
  return <ul>{visible.map(item => <li key={item}>{item}</li>)}</ul>;
}`
	if ids := parseSrc(t, ".tsx", src); !has(ids, "rerender-memo-before-early-return") {
		t.Fatalf("expected rerender-memo-before-early-return, got %v", ids)
	}
}
