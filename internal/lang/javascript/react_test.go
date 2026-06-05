package javascript

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aykutssert/scout/internal/core"
)

// The React hint pack must only engage on real React/Next code, not on every
// JS/TS project — otherwise its React-shaped hints fire on plain Node/backend
// code (the noise this gate removes).
func TestReactPackDetectGatesOnReactSignal(t *testing.T) {
	react := React()

	// Plain Node project: a .ts file and an Express dependency, no React.
	node := t.TempDir()
	if err := os.WriteFile(filepath.Join(node, "package.json"), []byte(`{"dependencies":{"express":"^4.0.0"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if react.Detect(core.ProjectContext{Root: node, Files: []string{"src/server.ts"}}).Matched {
		t.Fatal("react pack must not match a plain Node/Express project")
	}

	// React project via a .tsx file.
	if !react.Detect(core.ProjectContext{Root: t.TempDir(), Files: []string{"src/App.tsx"}}).Matched {
		t.Fatal("react pack must match when a .tsx file is present")
	}

	// React project via dependency, even when only .ts files changed.
	dep := t.TempDir()
	if err := os.WriteFile(filepath.Join(dep, "package.json"), []byte(`{"dependencies":{"react":"^18.0.0"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !react.Detect(core.ProjectContext{Root: dep, Files: []string{"src/hooks.ts"}}).Matched {
		t.Fatal("react pack must match a react-dependency project on a .ts diff")
	}
}
