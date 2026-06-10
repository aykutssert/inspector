// Package explain loads the rule catalog and answers explain queries.
// The default catalog is embedded in the binary (catalog.yaml in this package).
// An override path can be supplied at runtime for development or custom entries.
package explain

import (
	_ "embed"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

//go:embed catalog.yaml
var defaultCatalog []byte

// Entry is one rule's explanation.
type Entry struct {
	ID   string `yaml:"id"`
	Why  string `yaml:"why"`
	Bad  string `yaml:"bad"`
	Good string `yaml:"good"`
	Fix  string `yaml:"fix"`
}

// Catalog holds all loaded entries indexed by rule ID.
type Catalog struct {
	entries map[string]Entry
}

// Default loads the catalog embedded in the binary.
func Default() (*Catalog, error) {
	return parse(defaultCatalog, "<embedded>")
}

// Load reads the catalog from the given YAML file path, overriding the default.
func Load(path string) (*Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("explain: read catalog %s: %w", path, err)
	}
	return parse(data, path)
}

func parse(data []byte, src string) (*Catalog, error) {
	var entries []Entry
	if err := yaml.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("explain: parse catalog %s: %w", src, err)
	}
	c := &Catalog{entries: make(map[string]Entry, len(entries))}
	for _, e := range entries {
		c.entries[e.ID] = e
	}
	return c, nil
}

// Lookup returns the entry for ruleID and true, or a zero Entry and false if
// the rule has no catalog entry yet.
func (c *Catalog) Lookup(ruleID string) (Entry, bool) {
	e, ok := c.entries[ruleID]
	return e, ok
}
