package context

// RepoMap is a language-agnostic structural map of a repository.
//
// Designed to be the first thing an LLM reads about a codebase: a single
// document that conveys what matters, how files relate, and where to start —
// without reading a single source file.
//
// Sources of truth only: all fields are derived from the parsed import graph,
// AST-extracted exports, and project manifests (package.json, go.mod, etc.).
// No folder-naming heuristics or guesses.
type RepoMap struct {
	Language     string    `json:"language"`
	Framework    string    `json:"framework,omitempty"`
	FrameworkVer string    `json:"framework_ver,omitempty"`
	EntryPoints  []string  `json:"entry_points,omitempty"`
	HotFiles     []FileNode `json:"hot_files,omitempty"` // top files by import count
	Dirs         []DirNode  `json:"dirs"`
}

// DirNode represents one directory in the repo, with its files sorted by
// importance (imported_by count descending).
type DirNode struct {
	Path      string     `json:"path"`
	FileCount int        `json:"file_count"`
	Files     []FileNode `json:"files"`
}

// FileNode is a single file's structural fingerprint.
//
// ImportedBy is the number of other files in the repo that import this file —
// the primary signal of importance. A file with ImportedBy=0 and exports is
// likely an entry point; one with ImportedBy=20 is a critical shared module.
type FileNode struct {
	Path       string   `json:"path"`
	ImportedBy int      `json:"imported_by"`
	Exports    []Export `json:"exports,omitempty"`
	Deps       []string `json:"deps,omitempty"` // external packages (npm, pip, etc.)
}

// Export is one publicly accessible symbol from a file.
type Export struct {
	Name string `json:"name"`
	Kind string `json:"kind"`          // function | class | method | type | const
	Sig  string `json:"sig,omitempty"` // first-line signature (stripped of body opener)
}

// MapProvider builds a RepoMap from a project root.
// Language adapters implement this interface; the app layer calls it.
type MapProvider interface {
	GetMap(root string, files []string) (RepoMap, error)
}
