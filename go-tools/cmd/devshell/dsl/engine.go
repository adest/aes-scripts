package dsl

// Engine runs the three-phase build pipeline:
//  1. Raw validation  — structural checks before any expansion
//  2. Expansion       — resolves abstract nodes via the type registry
//  3. Runtime validation — ensures the expanded tree satisfies all runtime constraints
type Engine struct {
	registry *Registry
}

// NewEngine returns an Engine backed by the provided Registry.
func NewEngine(reg *Registry) *Engine {
	return &Engine{registry: reg}
}

// Build validates, expands, and re-validates the raw node list.
// On success it returns an implicit root Container wrapping all top-level nodes.
func (e *Engine) Build(raw []RawNode) (*Container, error) {
	if err := validateRawTree(raw); err != nil {
		return nil, err
	}

	root, err := expandRoot(raw, e.registry)
	if err != nil {
		return nil, err
	}

	if err := validateRuntimeTree(root); err != nil {
		return nil, err
	}

	return root, nil
}
