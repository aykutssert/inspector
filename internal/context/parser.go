package context

// FileParse holds the cross-language structural fingerprint of one source file.
// New language adapters produce this struct by implementing FileParser.
type FileParse struct {
	Path     string   `json:"path"`
	Imports  []Import `json:"imports,omitempty"`
	Defs     []Def    `json:"defs,omitempty"`
	Calls    []Call   `json:"calls,omitempty"`
	HasError bool     `json:"has_error,omitempty"`
}

// Import records one import statement.
type Import struct {
	Source string `json:"source"`
	Line   int    `json:"line"`
}

// Def records one definition (function, class, const, etc.).
type Def struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Line     int    `json:"line"`
	EndLine  int    `json:"end_line"`
	Exported bool   `json:"exported"`
}

// Call records one function/method call site.
type Call struct {
	Name string `json:"name"`
	Recv string `json:"recv,omitempty"`
	Line int    `json:"line"`
}

// FileParser is the interface that every language adapter implements.
// Given a project root and relative file path, it produces a FileParse.
type FileParser interface {
	Parse(root, path string) (*FileParse, error)
}

// MultiLangParser combines multiple FileParser instances by file path.
// It tries each parser in order and returns the first successful parse.
type MultiLangParser struct {
	parsers []FileParser
}

func NewMultiLangParser(parsers ...FileParser) *MultiLangParser {
	return &MultiLangParser{parsers: parsers}
}

func (m *MultiLangParser) Add(p FileParser) {
	m.parsers = append(m.parsers, p)
}

func (m *MultiLangParser) Parse(root, path string) (*FileParse, error) {
	for _, p := range m.parsers {
		fp, err := p.Parse(root, path)
		if err != nil {
			continue
		}
		if fp != nil {
			return fp, nil
		}
	}
	return nil, nil
}
