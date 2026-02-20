package dslyaml

import (
	"fmt"

	"go-tools/cmd/devshell/dsl"

	"gopkg.in/yaml.v3"
)

// Document is the YAML-level wrapper format that can embed both type definitions and nodes.
//
// YAML forms supported:
// - Mapping form (preferred): { types: {<name>: <RawNode>}, nodes: [<RawNode>...] }
// - Shorthand form (backward-compatible): a root YAML sequence is interpreted as nodes only.
//
// This package lives outside `dsl` on purpose: it handles I/O and YAML document conventions.
// The `dsl` package remains focused on the execution model, validation and expansion.
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

// NewRegistryFromDocuments creates a new empty DSL registry and registers all types from the provided documents.
func NewRegistryFromDocuments(docs ...Document) (*dsl.Registry, error) {
	reg := dsl.NewRegistry()
	for _, doc := range docs {
		for typeName, def := range doc.Types {
			if err := reg.Register(typeName, dsl.TypeDefinition{Expand: def}); err != nil {
				return nil, fmt.Errorf("phase=parse path=<doc>: register type %q: %w", typeName, err)
			}
		}
	}
	return reg, nil
}

// Build builds a runtime model from one YAML document.
// Types are loaded from `types`, then nodes from `nodes` are built with an initially empty registry.
func Build(in []byte) (*dsl.Container, error) {
	doc, err := Parse(in)
	if err != nil {
		return nil, err
	}
	return BuildFromDocuments(doc)
}

// BuildMany builds a runtime model from multiple YAML documents.
// Types are merged from all documents, and nodes are concatenated in order.
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

// BuildFromDocuments builds a runtime model from already parsed documents.
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

	eng := dsl.NewEngine(reg)
	return eng.Build(nodes)
}
