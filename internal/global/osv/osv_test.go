package osv

import (
	"reflect"
	"testing"
)

func TestOSVScanArgsUseV2SourceCommand(t *testing.T) {
	got := osvScanArgs("/repo")
	want := []string{"scan", "source", "--format", "json", "-r", "/repo"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("osv args mismatch\nwant: %#v\n got: %#v", want, got)
	}
}

func TestManifestChangedUsesDependencyManifestBasename(t *testing.T) {
	if !manifestChanged([]string{"apps/web/src/app.ts", "apps/web/package-lock.json"}) {
		t.Fatal("package-lock.json should trigger dependency scan even under a workspace path")
	}
	if manifestChanged([]string{"apps/web/src/app.ts", "packages/ui/Button.tsx"}) {
		t.Fatal("source-only changes should not trigger dependency scan")
	}
}
