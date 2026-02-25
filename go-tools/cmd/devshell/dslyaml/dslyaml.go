package dslyaml

import (
	"fmt"

	"go-tools/cmd/devshell/dsl"

	"gopkg.in/yaml.v3"
)

// Document is the YAML-level representation of a DSL file.
//
// Two forms are supported:
//   - Mapping form (preferred): a mapping with "types" and "nodes" keys.
//   - Shorthand form (backward-compatible): a bare sequence, interpreted as nodes only.
type Document struct {
	Types map[string]dsl.RawNode `yaml:"types,omitempty"`
	Nodes []dsl.RawNode          `yaml:"nodes,omitempty"`
}

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
		var nodes []dsl.RawNode
		if err := root.Decode(&nodes); err != nil {
			return Document{}, err
		}
		return Document{Nodes: nodes}, nil
	case yaml.MappingNode:
		var d Document
		if err := root.Decode(&d); err != nil {
			return Document{}, err
		}
		return d, nil
	default:
		return Document{}, fmt.Errorf("phase=parse path=<doc>: unexpected YAML root kind: %d", root.Kind)
	}
}

// NewRegistryFromDocuments builds a Registry from the type definitions in all provided documents.
// Returns an error if the same type name appears in more than one document.
func NewRegistryFromDocuments(docs ...Document) (*dsl.Registry, error) {
	reg := dsl.NewRegistry()
	for _, doc := range docs {
		for typeName, raw := range doc.Types {
			if err := reg.Register(typeName, raw); err != nil {
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

// BuildMany parses multiple YAML documents, merges their types, concatenates their
// nodes in order, and returns the runtime execution tree.
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

// BuildFromDocuments builds the runtime execution tree from already-parsed documents.
// Types are merged across all documents; nodes are concatenated in order.
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
