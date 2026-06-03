package context

type DefLoc struct {
	Name     string `json:"name"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	EndLine  int    `json:"end_line"`
	Kind     string `json:"kind"`
	Exported bool   `json:"exported"`
}

type CallLoc struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Recv string `json:"recv,omitempty"`
	// Resolved is true when the call was tied to a definition through a
	// language-specific high-confidence edge. False means the provider used a
	// weaker heuristic and agents should treat it as supporting context.
	Resolved bool `json:"resolved"`
}

type DefDetail struct {
	DefLoc
	Callees []string `json:"callees,omitempty"`
	// Callers are high-confidence edges. UnresolvedCallers are looser provider
	// heuristics kept separate so an agent does not mistake a guess for a
	// definite dependency.
	Callers           []CallLoc `json:"callers,omitempty"`
	UnresolvedCallers []CallLoc `json:"unresolved_callers,omitempty"`
	Source            string    `json:"source,omitempty"`
}

type Context struct {
	Target      string      `json:"target"`
	Kind        string      `json:"kind"`
	Definitions []DefDetail `json:"definitions,omitempty"`
	Defs        []DefLoc    `json:"defs,omitempty"`
	Imports     []string    `json:"imports,omitempty"`
	ImportedBy  []string    `json:"imported_by,omitempty"`
	Diagnostics []string    `json:"diagnostics,omitempty"`
}

type Provider interface {
	Name() string
	GetContext(root string, files []string, target string) (Context, error)
}
