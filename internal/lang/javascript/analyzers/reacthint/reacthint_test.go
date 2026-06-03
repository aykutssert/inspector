package reacthint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// parseSrc writes src to a temp file with ext and runs the detectors.
func parseSrc(t *testing.T, ext, src string) []string {
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
	var ids []string
	for _, f := range fs {
		ids = append(ids, f.RuleID)
	}
	return ids
}

func has(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
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

func TestRepeatedMagicLiteral(t *testing.T) {
	src := `const statusA = "pending-review";
const statusB = "pending-review";
const statusC = "pending-review";
const statusD = "pending-review";
const retryA = 30;
const retryB = 30;
const retryC = 30;`
	if ids := parseSrc(t, ".ts", src); !has(ids, "repeated-magic-literal") {
		t.Fatalf("expected repeated-magic-literal, got %v", ids)
	}
}

func TestCommonLiteralsNotFlagged(t *testing.T) {
	src := `import React from "react";
import { useMemo } from "react";
const a = "";
const b = "";
const c = "";
const d = "";
const x = 1;
const y = 1;
const z = 1;
const label = "ok";
const label2 = "ok";
const label3 = "ok";
const label4 = "ok";`
	if ids := parseSrc(t, ".tsx", src); has(ids, "repeated-magic-literal") {
		t.Fatalf("common literals should not be flagged, got %v", ids)
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
