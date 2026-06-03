package packs

import "testing"

func TestDefaultRegistryProvidesScanSurface(t *testing.T) {
	r := Default()
	if got := len(r.ScanAdapters("rules")); got < 2 {
		t.Fatalf("default registry should include JS and Svelte scan adapters, got %d", got)
	}
	if got := len(r.ContextAdapters()); got == 0 {
		t.Fatal("default registry should include at least one context adapter")
	}
	if got := len(r.Analyzers(nil)); got < 3 {
		t.Fatalf("default registry should include global and pack analyzers, got %d", got)
	}
}
