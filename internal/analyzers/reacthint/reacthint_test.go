package reacthint

import (
	"os"
	"path/filepath"
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
