package dsl

type TypeDefinition struct {
	Expand RawNode
}

type Registry struct {
	types map[string]TypeDefinition
}

func NewRegistry() *Registry {
	return &Registry{
		types: make(map[string]TypeDefinition),
	}
}

func (r *Registry) Register(name string, def TypeDefinition) error {
	if _, exists := r.types[name]; exists {
		return ErrTypeAlreadyExists
	}
	r.types[name] = def
	return nil
}

func (r *Registry) Get(name string) (TypeDefinition, bool) {
	def, ok := r.types[name]
	return def, ok
}
