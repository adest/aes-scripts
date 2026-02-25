package dsl

// RawNode represents a node as parsed from an external source (e.g. YAML).
// It is intentionally format-agnostic: no serialization tags.
type RawNode struct {
	Name     string
	Command  *string
	Cwd      *string
	Env      map[string]string
	Children []RawNode
	Uses     []string
}
