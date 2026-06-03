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

func TestStateFromProp(t *testing.T) {
	src := `function C(props) {
  const [v, setV] = useState(props.value);
  return null;
}`
	ids := parseSrc(t, ".tsx", src)
	if !has(ids, "state-initialized-from-prop") {
		t.Fatalf("expected state-initialized-from-prop, got %v", ids)
	}
}

func TestStateFromLiteralNotFlagged(t *testing.T) {
	src := `function C() {
  const [v, setV] = useState(0);
  const [s, setS] = useState("");
  return null;
}`
	ids := parseSrc(t, ".tsx", src)
	if has(ids, "state-initialized-from-prop") {
		t.Fatalf("literal initializer should not be flagged, got %v", ids)
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
	ids := parseSrc(t, ".tsx", src)
	if !has(ids, "setstate-in-effect-without-deps") {
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
	ids := parseSrc(t, ".tsx", src)
	if has(ids, "setstate-in-effect-without-deps") {
		t.Fatalf("effect with dep array should not be flagged, got %v", ids)
	}
}

func TestEffectNoDepsWithoutSetterNotFlagged(t *testing.T) {
	src := `function C() {
  useEffect(() => {
    console.log("tick");
  });
  return null;
}`
	ids := parseSrc(t, ".tsx", src)
	if has(ids, "setstate-in-effect-without-deps") {
		t.Fatalf("effect without a setter should not be flagged, got %v", ids)
	}
}

// Plain .ts (no JSX grammar) must still parse hooks without error.
func TestPlainTSParses(t *testing.T) {
	src := `export function useThing(props: { value: number }) {
  const [v, setV] = useState(props.value);
  return v;
}`
	ids := parseSrc(t, ".ts", src)
	if !has(ids, "state-initialized-from-prop") {
		t.Fatalf("expected hint in .ts, got %v", ids)
	}
}
