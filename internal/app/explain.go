package app

import (
	"github.com/aykutssert/scout/internal/explain"
)

// ExplainOptions configures an explain lookup.
type ExplainOptions struct {
	// CatalogPath overrides the embedded default catalog.
	// Leave empty to use the catalog embedded in the binary.
	CatalogPath string
	// RuleID is the rule to look up (e.g. "general.jwt-verify-without-algorithms").
	RuleID string
}

// ExplainResult is the explain command's output.
type ExplainResult struct {
	RuleID string `json:"rule_id"`
	Why    string `json:"why"`
	Bad    string `json:"bad,omitempty"`
	Good   string `json:"good,omitempty"`
	Fix    string `json:"fix,omitempty"`
	// Found is false when the rule has no catalog entry yet.
	Found bool `json:"found"`
}

// Explain looks up a rule in the catalog and returns its explanation.
func Explain(opts ExplainOptions) (ExplainResult, error) {
	var (
		c   *explain.Catalog
		err error
	)
	if opts.CatalogPath != "" {
		c, err = explain.Load(opts.CatalogPath)
	} else {
		c, err = explain.Default()
	}
	if err != nil {
		return ExplainResult{}, err
	}

	entry, ok := c.Lookup(opts.RuleID)
	if !ok {
		return ExplainResult{RuleID: opts.RuleID, Found: false}, nil
	}
	return ExplainResult{
		RuleID: entry.ID,
		Why:    entry.Why,
		Bad:    entry.Bad,
		Good:   entry.Good,
		Fix:    entry.Fix,
		Found:  true,
	}, nil
}
