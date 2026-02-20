package dsl

type Engine struct {
	registry *Registry
}

func NewEngine(reg *Registry) *Engine {
	return &Engine{registry: reg}
}

func (e *Engine) Build(raw []RawNode) (*Container, error) {

	if err := ValidateRawTree(raw); err != nil {
		return nil, err
	}

	root, err := ExpandRoot(raw, e.registry)
	if err != nil {
		return nil, err
	}

	if err := validateRuntimeTree(root); err != nil {
		return nil, err
	}

	return root, nil
}
