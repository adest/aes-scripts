package dsl

// Registry holds named type definitions available for node expansion.
// Types are registered once before building, then looked up by name during expansion.
type Registry struct {
	types map[string]TypeDef
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		types: make(map[string]TypeDef),
	}
}

// Register adds a type definition to the registry.
// Returns ErrTypeAlreadyExists if a type with that name is already registered.
func (r *Registry) Register(name string, def TypeDef) error {
	if _, exists := r.types[name]; exists {
		return ErrTypeAlreadyExists
	}
	r.types[name] = def
	return nil
}

// Get returns the type definition for the given type name.
func (r *Registry) Get(name string) (TypeDef, bool) {
	def, ok := r.types[name]
	return def, ok
}
