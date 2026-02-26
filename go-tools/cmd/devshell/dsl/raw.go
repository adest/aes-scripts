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

	// With holds parameters passed to the types referenced in Uses.
	// It is nil when the node has no `with` block.
	With *WithBlock
}

// WithBlock holds the parameters passed to a type via the `with` key.
// Exactly one of Shared or PerType is populated:
//
//   - Shared: a flat key/value map, shared across all types listed in Uses.
//     YAML form: `with: { key: value, ... }`
//
//   - PerType: a list of per-type param sets, used when different types in
//     Uses need different parameters.
//     YAML form: `with: [ { type: foo, key: value }, ... ]`
type WithBlock struct {
	Shared  map[string]string // mapping form
	PerType []TypedWith       // list form
}

// TypedWith associates a set of parameters with a specific type name.
// It is used in the list form of WithBlock.
type TypedWith struct {
	Type   string
	Params map[string]string
}
