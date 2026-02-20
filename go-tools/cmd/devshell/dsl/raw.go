package dsl

// RawNode represents a node coming from YAML parsing.
// It is intentionally simple and neutral.
type RawNode struct {
	Name     string
	Command  *string
	Children []RawNode
	Uses     []string
}
