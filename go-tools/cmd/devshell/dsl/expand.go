package dsl

import (
	"fmt"
)

func ExpandRoot(nodes []RawNode, reg *Registry) (*Container, error) {
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

func expandNode(r RawNode, reg *Registry, stack []string, path string) (Node, error) {

	if r.Name == "" {
		return nil, fmt.Errorf("phase=expand path=%s: %w", path, ErrInvalidNode)
	}

	// Detect cycle
	for _, t := range r.Uses {
		for _, s := range stack {
			if s == t {
				return nil, fmt.Errorf("phase=expand path=%s: %w: %s", path, ErrCycleDetected, t)
			}
		}
	}

	// Runnable leaf
	if r.Command != nil {
		if len(r.Children) > 0 || len(r.Uses) > 0 {
			return nil, fmt.Errorf("phase=expand path=%s: %w", path, ErrInvalidNode)
		}
		return &Runnable{
			NodeName: r.Name,
			Command:  *r.Command,
		}, nil
	}

	// Abstract node: if there is exactly one use, the node can become directly the expanded node.
	if len(r.Uses) == 1 && len(r.Children) == 0 {
		t := r.Uses[0]
		def, ok := reg.Get(t)
		if !ok {
			return nil, fmt.Errorf("phase=expand path=%s: %w: %s", path, ErrUnknownType, t)
		}

		newStack := append(stack, t)
		expanded, err := expandNode(def.Expand, reg, newStack, path)
		if err != nil {
			return nil, err
		}
		// Adopt the expanded node, but keep the abstract node name.
		switch n := expanded.(type) {
		case *Runnable:
			n.NodeName = r.Name
		case *Container:
			n.NodeName = r.Name
		}
		return expanded, nil
	}

	container := &Container{
		NodeName: r.Name,
	}

	// Expand types
	for _, t := range r.Uses {

		def, ok := reg.Get(t)
		if !ok {
			return nil, fmt.Errorf("phase=expand path=%s: %w: %s", path, ErrUnknownType, t)
		}

		newStack := append(stack, t)
		childPath := joinPath(path, def.Expand.Name)
		if def.Expand.Name == "" {
			childPath = joinPath(path, "<type>")
		}
		child, err := expandNode(def.Expand, reg, newStack, childPath)
		if err != nil {
			return nil, err
		}

		container.Children = append(container.Children, child)
	}

	// Expand explicit children
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

	// Collision detection
	seen := map[string]struct{}{}
	for _, child := range container.Children {
		if _, exists := seen[child.Name()]; exists {
			return nil, fmt.Errorf("phase=expand path=%s: %w: %s", path, ErrDuplicateChild, child.Name())
		}
		seen[child.Name()] = struct{}{}
	}

	return container, nil
}
