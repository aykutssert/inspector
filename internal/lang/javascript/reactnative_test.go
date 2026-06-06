package javascript

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aykutssert/scout/internal/core"
)

func TestReactNativePackDetection(t *testing.T) {
	native := t.TempDir()
	if err := os.WriteFile(filepath.Join(native, "package.json"),
		[]byte(`{"dependencies":{"react-native":"0.80.0"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !ReactNative().Detect(core.ProjectContext{Root: native, Files: []string{"App.tsx"}}).Matched {
		t.Fatal("React Native pack should match react-native dependency")
	}

	web := t.TempDir()
	if err := os.WriteFile(filepath.Join(web, "package.json"),
		[]byte(`{"dependencies":{"react":"^19.0.0"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if ReactNative().Detect(core.ProjectContext{Root: web, Files: []string{"App.tsx"}}).Matched {
		t.Fatal("React Native pack must not match web React")
	}
}
