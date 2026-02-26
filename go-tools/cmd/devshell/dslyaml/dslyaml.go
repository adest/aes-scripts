package dslyaml

import (
	"fmt"
	"unicode"

	"go-tools/cmd/devshell/dsl"

	"gopkg.in/yaml.v3"
)

// Document is the Go-level representation of a parsed DSL file.
//
// Two YAML forms are supported:
//   - Mapping form (preferred): a mapping with "types" and "nodes" keys.
//   - Shorthand form (backward-compatible): a bare sequence, interpreted as nodes only.
type Document struct {
	Types map[string]dsl.TypeDef
	Nodes []dsl.RawNode
}

// ---- Internal YAML parsing structs ----------------------------------------
//
// These types mirror the public dsl types but carry YAML struct tags and handle
// format-specific concerns (polymorphic `with`, scalar normalisation for
// `params`). They are converted to dsl types before being returned to callers.

// yamlDocument is the internal YAML parsing struct for a DSL file in mapping form.
type yamlDocument struct {
	Types map[string]yamlTypeDef `yaml:"types,omitempty"`
	Nodes []yamlRawNode          `yaml:"nodes,omitempty"`
}

// yamlTypeDef is the YAML representation of a type definition.
// The `params` block sits at the same level as the node body fields.
type yamlTypeDef struct {
	// Params maps parameter names to their default values.
	// A YAML null (~) is decoded as nil, meaning the parameter is required.
	// Any scalar (string, number) is decoded as the default value.
	Params map[string]interface{} `yaml:"params,omitempty"`

	// Node body fields — same as yamlRawNode.
	Name     string            `yaml:"name"`
	Command  *string           `yaml:"command,omitempty"`
	Cwd      *string           `yaml:"cwd,omitempty"`
	Env      map[string]string `yaml:"env,omitempty"`
	Children []yamlRawNode     `yaml:"children,omitempty"`
	Uses     []string          `yaml:"uses,omitempty"`
	// With uses yaml.Node (not *yaml.Node) because yaml.v3 does not populate
	// *yaml.Node fields correctly when decoding into a struct — the Kind ends
	// up as 0. A non-pointer yaml.Node is decoded correctly.
	// An absent `with` key is detected by checking With.Kind == 0.
	With yaml.Node `yaml:"with,omitempty"`
}

// yamlRawNode is the YAML representation of a node.
// It is identical to dsl.RawNode except that `with` is held as yaml.Node
// for polymorphic decoding (see the comment on yamlTypeDef.With).
type yamlRawNode struct {
	Name     string            `yaml:"name"`
	Command  *string           `yaml:"command,omitempty"`
	Cwd      *string           `yaml:"cwd,omitempty"`
	Env      map[string]string `yaml:"env,omitempty"`
	Children []yamlRawNode     `yaml:"children,omitempty"`
	Uses     []string          `yaml:"uses,omitempty"`
	With     yaml.Node         `yaml:"with,omitempty"`
}

// ---- Parse -----------------------------------------------------------------

// Parse parses a YAML document in either mapping or shorthand form.
func Parse(in []byte) (Document, error) {
	var docNode yaml.Node
	if err := yaml.Unmarshal(in, &docNode); err != nil {
		return Document{}, err
	}
	if len(docNode.Content) == 0 {
		return Document{}, fmt.Errorf("phase=parse path=<doc>: empty YAML")
	}
	root := docNode.Content[0]

	switch root.Kind {
	case yaml.SequenceNode:
		// Shorthand form: bare list → nodes only, no types.
		var yamlNodes []yamlRawNode
		if err := root.Decode(&yamlNodes); err != nil {
			return Document{}, err
		}
		nodes, err := convertNodes(yamlNodes)
		if err != nil {
			return Document{}, err
		}
		return Document{Nodes: nodes}, nil

	case yaml.MappingNode:
		var yd yamlDocument
		if err := root.Decode(&yd); err != nil {
			return Document{}, err
		}
		return convertDocument(yd)

	default:
		return Document{}, fmt.Errorf("phase=parse path=<doc>: unexpected YAML root kind: %d", root.Kind)
	}
}

// ---- Convert: yaml types → dsl types --------------------------------------

// convertDocument converts a yamlDocument to the public Document type.
func convertDocument(yd yamlDocument) (Document, error) {
	types, err := convertTypeDefs(yd.Types)
	if err != nil {
		return Document{}, err
	}
	nodes, err := convertNodes(yd.Nodes)
	if err != nil {
		return Document{}, err
	}
	return Document{Types: types, Nodes: nodes}, nil
}

// convertTypeDefs converts the YAML map of type definitions to dsl.TypeDef values.
func convertTypeDefs(raw map[string]yamlTypeDef) (map[string]dsl.TypeDef, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	out := make(map[string]dsl.TypeDef, len(raw))
	for name, ytd := range raw {
		td, err := convertTypeDef(ytd)
		if err != nil {
			return nil, fmt.Errorf("type %q: %w", name, err)
		}
		out[name] = td
	}
	return out, nil
}

// convertTypeDef converts a single yamlTypeDef to dsl.TypeDef.
func convertTypeDef(ytd yamlTypeDef) (dsl.TypeDef, error) {
	params, err := convertParams(ytd.Params)
	if err != nil {
		return dsl.TypeDef{}, err
	}

	// Reuse node conversion by wrapping the type body fields.
	body, err := convertNode(yamlRawNode{
		Name:     ytd.Name,
		Command:  ytd.Command,
		Cwd:      ytd.Cwd,
		Env:      ytd.Env,
		Children: ytd.Children,
		Uses:     ytd.Uses,
		With:     ytd.With,
	})
	if err != nil {
		return dsl.TypeDef{}, err
	}

	return dsl.TypeDef{Params: params, Body: body}, nil
}

// convertParams converts a YAML params block to dsl.ParamDefs.
//
// Conversion rules:
//   - null value (~)  → nil pointer  (parameter is required)
//   - any scalar      → fmt.Sprintf("%v", v) as default string (parameter is optional)
//
// Numbers are valid defaults and are normalised to their string representation.
//
// Param names must be valid Go identifiers because they are referenced via
// {{ .paramName }} in Go templates. A hyphen like "my-param" would be parsed
// by the template engine as subtraction rather than as a field name.
// Use underscores instead: "my_param" → {{ .my_param }}.
func convertParams(raw map[string]interface{}) (dsl.ParamDefs, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	defs := make(dsl.ParamDefs, len(raw))
	for k, v := range raw {
		if !isGoIdentifier(k) {
			return nil, fmt.Errorf(
				"param %q: invalid name: param names must be valid Go identifiers "+
					"(letters, digits, and underscores only — no hyphens); "+
					"hint: rename to %q and use {{ .%s }} in templates",
				k, toSnakeCase(k), toSnakeCase(k),
			)
		}
		if v == nil {
			defs[k] = nil // required
		} else {
			s := fmt.Sprintf("%v", v)
			defs[k] = &s // optional with this default
		}
	}
	return defs, nil
}

// isGoIdentifier reports whether s is a valid Go identifier:
// a non-empty string of letters, digits, and underscores that does not
// start with a digit.
func isGoIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case unicode.IsLetter(r), r == '_':
			// always valid
		case unicode.IsDigit(r) && i > 0:
			// valid except as first character
		default:
			return false
		}
	}
	return true
}

// toSnakeCase replaces hyphens with underscores, used only for error hint messages.
func toSnakeCase(s string) string {
	out := make([]byte, len(s))
	for i := range s {
		if s[i] == '-' {
			out[i] = '_'
		} else {
			out[i] = s[i]
		}
	}
	return string(out)
}

// convertNodes converts a slice of yamlRawNode to []dsl.RawNode.
func convertNodes(raw []yamlRawNode) ([]dsl.RawNode, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	out := make([]dsl.RawNode, len(raw))
	for i, yn := range raw {
		n, err := convertNode(yn)
		if err != nil {
			return nil, err
		}
		out[i] = n
	}
	return out, nil
}

// convertNode converts a single yamlRawNode to dsl.RawNode.
func convertNode(yn yamlRawNode) (dsl.RawNode, error) {
	r := dsl.RawNode{
		Name:    yn.Name,
		Command: yn.Command,
		Cwd:     yn.Cwd,
		Env:     yn.Env,
		Uses:    yn.Uses,
	}

	// Preserve an explicit empty children slice (e.g. `children: []`) so that
	// downstream validation can distinguish "children key absent" (nil) from
	// "children key present but empty" (non-nil empty slice).
	if yn.Children != nil {
		r.Children = make([]dsl.RawNode, 0, len(yn.Children))
		for _, yc := range yn.Children {
			child, err := convertNode(yc)
			if err != nil {
				return dsl.RawNode{}, err
			}
			r.Children = append(r.Children, child)
		}
	}

	// yaml.Node.Kind == 0 means the field was absent in the YAML source.
	// We use a non-pointer yaml.Node (not *yaml.Node) because yaml.v3 does
	// not correctly populate *yaml.Node struct fields — Kind stays 0.
	if yn.With.Kind != 0 {
		wb, err := convertWithBlock(&yn.With)
		if err != nil {
			return dsl.RawNode{}, fmt.Errorf("with: %w", err)
		}
		r.With = &wb
	}

	return r, nil
}

// convertWithBlock converts a polymorphic YAML `with` node to dsl.WithBlock.
//
// Two YAML forms are accepted:
//
//	Mapping:  with: { key: value, ... }          → WithBlock.Shared
//	Sequence: with: [ { type: foo, ... }, ... ]  → WithBlock.PerType
//
// Scalar values (strings and numbers) are normalised to strings in both forms.
func convertWithBlock(node *yaml.Node) (dsl.WithBlock, error) {
	switch node.Kind {
	case yaml.MappingNode:
		m, err := decodeMappingAsStrings(node)
		if err != nil {
			return dsl.WithBlock{}, err
		}
		return dsl.WithBlock{Shared: m}, nil

	case yaml.SequenceNode:
		var perType []dsl.TypedWith
		for _, item := range node.Content {
			tw, err := decodeTypedWith(item)
			if err != nil {
				return dsl.WithBlock{}, err
			}
			perType = append(perType, tw)
		}
		return dsl.WithBlock{PerType: perType}, nil

	default:
		return dsl.WithBlock{}, fmt.Errorf("expected mapping or sequence, got YAML kind %d", node.Kind)
	}
}

// decodeMappingAsStrings reads a YAML mapping node into a map[string]string.
// yaml.v3 stores every scalar's text representation in node.Value, so numbers
// like 8080 arrive here already as "8080" — no additional conversion needed.
func decodeMappingAsStrings(node *yaml.Node) (map[string]string, error) {
	// MappingNode.Content is a flat list of alternating key / value nodes.
	if len(node.Content)%2 != 0 {
		return nil, fmt.Errorf("malformed YAML mapping: odd number of content nodes")
	}
	out := make(map[string]string, len(node.Content)/2)
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i].Value
		val := node.Content[i+1].Value
		out[key] = val
	}
	return out, nil
}

// decodeTypedWith converts a single item of the `with` list form into a
// dsl.TypedWith. The `type` key is extracted; all other keys become Params.
func decodeTypedWith(node *yaml.Node) (dsl.TypedWith, error) {
	if node.Kind != yaml.MappingNode {
		return dsl.TypedWith{}, fmt.Errorf("'with' list item must be a mapping, got YAML kind %d", node.Kind)
	}
	tw := dsl.TypedWith{Params: make(map[string]string)}
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i].Value
		val := node.Content[i+1].Value
		if key == "type" {
			tw.Type = val
		} else {
			tw.Params[key] = val
		}
	}
	if tw.Type == "" {
		return dsl.TypedWith{}, fmt.Errorf("'with' list item is missing 'type'")
	}
	return tw, nil
}

// ---- Public build functions ------------------------------------------------

// NewRegistryFromDocuments builds a Registry from the type definitions in all
// provided documents. Returns an error if the same type name appears in more
// than one document.
func NewRegistryFromDocuments(docs ...Document) (*dsl.Registry, error) {
	reg := dsl.NewRegistry()
	for _, doc := range docs {
		for typeName, def := range doc.Types {
			if err := reg.Register(typeName, def); err != nil {
				return nil, fmt.Errorf("phase=parse path=<doc>: register type %q: %w", typeName, err)
			}
		}
	}
	return reg, nil
}

// Build parses a single YAML document and returns the runtime execution tree.
func Build(in []byte) (*dsl.Container, error) {
	doc, err := Parse(in)
	if err != nil {
		return nil, err
	}
	return BuildFromDocuments(doc)
}

// BuildMany parses multiple YAML documents, merges their types, concatenates
// their nodes in order, and returns the runtime execution tree.
func BuildMany(inputs ...[]byte) (*dsl.Container, error) {
	docs := make([]Document, 0, len(inputs))
	for _, in := range inputs {
		doc, err := Parse(in)
		if err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	return BuildFromDocuments(docs...)
}

// BuildFromDocuments builds the runtime execution tree from already-parsed
// documents. Types are merged across all documents; nodes are concatenated
// in order.
func BuildFromDocuments(docs ...Document) (*dsl.Container, error) {
	var nodes []dsl.RawNode
	for _, doc := range docs {
		nodes = append(nodes, doc.Nodes...)
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("phase=parse path=<doc>: missing or empty 'nodes'")
	}

	reg, err := NewRegistryFromDocuments(docs...)
	if err != nil {
		return nil, err
	}

	return dsl.NewEngine(reg).Build(nodes)
}
