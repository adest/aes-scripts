package dsl

import (
	"fmt"
	"strings"
)

// validateRawTree validates the top-level node list before expansion (phase 1).
func validateRawTree(nodes []RawNode) error {
	names := map[string]struct{}{}
	for _, n := range nodes {
		path := n.Name
		if path == "" {
			path = "<root>"
		}
		if err := validateRawNode(n, names, path); err != nil {
			return err
		}
	}
	return nil
}

// validateRawNode validates a single raw node: name presence, XOR rule, content
// constraints, sibling uniqueness, and recursive child validation.
func validateRawNode(n RawNode, siblings map[string]struct{}, path string) error {
	if n.Name == "" {
		return fmt.Errorf("phase=raw path=%s: node is missing a name", path)
	}
	if _, exists := siblings[n.Name]; exists {
		return fmt.Errorf("phase=raw path=%s: duplicate sibling name: %s", path, n.Name)
	}
	siblings[n.Name] = struct{}{}

	hasCommand := n.Command != nil
	hasChildren := n.Children != nil
	hasUses := n.Uses != nil

	count := 0
	if hasCommand {
		count++
	}
	if hasChildren {
		count++
	}
	if hasUses {
		count++
	}
	switch count {
	case 0:
		return fmt.Errorf("phase=raw path=%s: node must define exactly one of: command, children or uses", path)
	case 1:
		// valid, fall through to content checks
	default:
		return fmt.Errorf("phase=raw path=%s: node cannot combine command, children and uses", path)
	}

	switch {
	case hasCommand:
		if strings.TrimSpace(*n.Command) == "" {
			return fmt.Errorf("phase=raw path=%s: runnable command must not be empty", path)
		}
	case hasChildren:
		if len(n.Children) == 0 {
			return fmt.Errorf("phase=raw path=%s: container must have at least one child", path)
		}
		childNames := map[string]struct{}{}
		for _, c := range n.Children {
			childPath := joinPath(path, c.Name)
			if c.Name == "" {
				childPath = joinPath(path, "<missing>")
			}
			if err := validateRawNode(c, childNames, childPath); err != nil {
				return err
			}
		}
	case hasUses:
		if len(n.Uses) == 0 {
			return fmt.Errorf("phase=raw path=%s: abstract node uses must contain at least one entry", path)
		}
	}

	return nil
}

// validateRuntimeTree validates the expanded Node tree (phase 3).
func validateRuntimeTree(root Node) error {
	if root == nil {
		return fmt.Errorf("phase=runtime path=<root>: runtime tree is nil")
	}
	// The root is the implicit container. Report child paths without the "root." prefix.
	if c, ok := root.(*Container); ok && c.NodeName == "root" {
		if len(c.Children) == 0 {
			return fmt.Errorf("phase=runtime path=<root>: container must have at least one child")
		}
		childNames := map[string]struct{}{}
		for _, child := range c.Children {
			childPath := child.Name()
			if childPath == "" {
				childPath = "<missing>"
			}
			if err := validateRuntimeNode(child, childNames, childPath); err != nil {
				return err
			}
		}
		return nil
	}
	path := root.Name()
	if path == "" {
		path = "<root>"
	}
	return validateRuntimeNode(root, map[string]struct{}{}, path)
}

// validateRuntimeNode checks that a node satisfies runtime constraints after expansion.
func validateRuntimeNode(n Node, siblings map[string]struct{}, path string) error {
	name := n.Name()
	if name == "" {
		return fmt.Errorf("phase=runtime path=%s: node is missing a name", path)
	}
	if _, exists := siblings[name]; exists {
		return fmt.Errorf("phase=runtime path=%s: duplicate sibling name: %s", path, name)
	}
	siblings[name] = struct{}{}

	switch node := n.(type) {
	case *Container:
		if len(node.Children) == 0 {
			return fmt.Errorf("phase=runtime path=%s: container must have at least one child", path)
		}
		childNames := map[string]struct{}{}
		for _, c := range node.Children {
			childPath := joinPath(path, c.Name())
			if c.Name() == "" {
				childPath = joinPath(path, "<missing>")
			}
			if err := validateRuntimeNode(c, childNames, childPath); err != nil {
				return err
			}
		}
	case *Runnable:
		if strings.TrimSpace(node.Command) == "" {
			return fmt.Errorf("phase=runtime path=%s: runnable command must not be empty", path)
		}
	default:
		return fmt.Errorf("phase=runtime path=%s: unknown node type: %T", path, n)
	}
	return nil
}
