package dsl

// RawNode represents a node as parsed from an external source (e.g. YAML).
// It is intentionally format-agnostic: no serialization tags.
//
// A runnable node carries exactly one of Command or Argv — never both:
//
//   - Command (*string): the compact string form. Template substitution is applied
//     to the whole string first; the result is split into argv with strings.Fields
//     during expansion. This preserves template expressions that span tokens
//     (e.g. "docker compose -f {{ .file }} up -d").
//
//   - Argv ([]string): the pre-tokenized form, produced by either the array form
//     ("command: [...]") or the long form ("command: exe" + "args: [...]").
//     Template substitution is applied to each element independently.
type RawNode struct {
	Name     string
	Command  *string  // string form: template applied to whole string, then split
	Argv     []string // array/long form: already tokenized; template applied per-element
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
