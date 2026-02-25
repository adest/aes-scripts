package dsl

import "fmt"

// expandRoot wraps the top-level node list in an implicit root Container.
func expandRoot(nodes []RawNode, reg *Registry) (*Container, error) {
	root := &Container{NodeName: "root"}
	for _, n := range nodes {
		path := n.Name
		if path == "" {
			path = "<root>"
		}
		node, err := expandNode(n, reg, nil, path)
		if err != nil {
			return nil, err
		}
		root.Children = append(root.Children, node)
	}
	return root, nil
}

// expandNode recursively expands a single raw node into its runtime form.
//
// stack tracks the chain of type names currently being expanded for cycle detection.
// path is the dot-separated node address used in error messages.
func expandNode(r RawNode, reg *Registry, stack []string, path string) (Node, error) {
	if r.Name == "" {
		return nil, fmt.Errorf("phase=expand path=%s: %w", path, ErrInvalidNode)
	}

	// Detect cycles before descending into any uses.
	for _, t := range r.Uses {
		for _, s := range stack {
			if s == t {
				return nil, fmt.Errorf("phase=expand path=%s: %w: %s", path, ErrCycleDetected, t)
			}
		}
	}

	// Runnable leaf.
	if r.Command != nil {
		var cwd string
		if r.Cwd != nil {
			cwd = *r.Cwd
		}
		return &Runnable{
			NodeName: r.Name,
			Command:  *r.Command,
			Cwd:      cwd,
			Env:      r.Env,
		}, nil
	}

	// Single-use abstract node: the node takes the shape of the expanded type directly.
	// The abstract node's own name overrides the type root name (§4.2).
	if len(r.Uses) == 1 && len(r.Children) == 0 {
		t := r.Uses[0]
		raw, ok := reg.Get(t)
		if !ok {
			return nil, fmt.Errorf("phase=expand path=%s: %w: %s", path, ErrUnknownType, t)
		}
		expanded, err := expandNode(raw, reg, withType(stack, t), path)
		if err != nil {
			return nil, err
		}
		// Adopt the abstract node's name onto the expanded node.
		setNodeName(expanded, r.Name)
		return expanded, nil
	}

	// Container: collects children from uses (in order) then explicit children.
	container := &Container{NodeName: r.Name}

	for _, t := range r.Uses {
		raw, ok := reg.Get(t)
		if !ok {
			return nil, fmt.Errorf("phase=expand path=%s: %w: %s", path, ErrUnknownType, t)
		}
		childPath := joinPath(path, raw.Name)
		if raw.Name == "" {
			childPath = joinPath(path, "<type>")
		}
		child, err := expandNode(raw, reg, withType(stack, t), childPath)
		if err != nil {
			return nil, err
		}
		container.Children = append(container.Children, child)
	}

	for _, c := range r.Children {
		childPath := joinPath(path, c.Name)
		if c.Name == "" {
			childPath = joinPath(path, "<missing>")
		}
		child, err := expandNode(c, reg, stack, childPath)
		if err != nil {
			return nil, err
		}
		container.Children = append(container.Children, child)
	}

	// Reject duplicate sibling names produced by expansion.
	seen := map[string]struct{}{}
	for _, child := range container.Children {
		if _, exists := seen[child.Name()]; exists {
			return nil, fmt.Errorf("phase=expand path=%s: %w: %s", path, ErrDuplicateChild, child.Name())
		}
		seen[child.Name()] = struct{}{}
	}

	return container, nil
}

// withType returns a new stack with t appended, without mutating the original slice.
func withType(stack []string, t string) []string {
	return append(append([]string(nil), stack...), t)
}

// setNodeName sets the name on any Node (Runnable or Container).
func setNodeName(n Node, name string) {
	switch node := n.(type) {
	case *Runnable:
		node.NodeName = name
	case *Container:
		node.NodeName = name
	}
}

// joinPath builds a dot-separated path string for use in error messages.
func joinPath(parent, child string) string {
	if parent == "" {
		return child
	}
	if child == "" {
		return parent
	}
	return parent + "." + child
}
