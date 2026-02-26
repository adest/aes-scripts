package dsl

import (
	"fmt"
	"strings"
)

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
// The expansion logic handles four cases:
//  1. Runnable: a node with a `command` — converted directly.
//  2. Pipeline: a node with `steps` — converted directly (no type expansion).
//  3. Single-use abstract: a node with exactly one entry in `uses` and no
//     explicit children. The type is expanded and the abstract node's name
//     overrides the type root name (§5.2).
//  4. Container: explicit `children`, or multiple entries in `uses`, or both.
//     Uses are expanded first (in order), then explicit children are appended.
//
// stack tracks type names currently being expanded for cycle detection.
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

	// Validate PerType with entries now, before branching on single vs
	// multi-use, so the check applies to both cases.
	if r.With != nil {
		for _, tw := range r.With.PerType {
			if !containsString(r.Uses, tw.Type) {
				return nil, fmt.Errorf(
					"phase=expand path=%s: 'with' list references type %q which is not in uses",
					path, tw.Type,
				)
			}
		}
	}

	// --- Pipeline leaf ---
	// A node with steps is an executable pipeline. It does not recurse into
	// type expansion — steps are always concrete commands. Step references
	// ({{ steps.X.Y }}) embedded in argv/env/cwd are left as-is for the
	// executor to resolve at run time.
	if r.Steps != nil {
		return expandPipeline(r, path)
	}

	// --- Runnable leaf ---
	if r.Command != nil || r.Argv != nil {
		var argv []string
		if r.Command != nil {
			// String form: split after template substitution.
			argv = strings.Fields(*r.Command)
		} else {
			argv = r.Argv
		}
		var cwd string
		if r.Cwd != nil {
			cwd = *r.Cwd
		}
		return &Runnable{
			NodeName: r.Name,
			Argv:     argv,
			Cwd:      cwd,
			Env:      r.Env,
		}, nil
	}

	// --- Single-use abstract node ---
	// When Uses has exactly one type and no explicit children, the node adopts
	// the full shape of the expanded type. The abstract node's own name takes
	// priority over the type root name (§5.2).
	if len(r.Uses) == 1 && len(r.Children) == 0 {
		t := r.Uses[0]
		typeDef, ok := reg.Get(t)
		if !ok {
			return nil, fmt.Errorf("phase=expand path=%s: %w: %s", path, ErrUnknownType, t)
		}

		// Resolve params: validate caller params, apply defaults.
		params, err := resolveParams(typeDef.Params, r.With, t)
		if err != nil {
			return nil, fmt.Errorf("phase=expand path=%s: %w", path, err)
		}

		// Substitute {{ .paramName }} in all string fields of the type body.
		substituted, err := applyTemplates(typeDef.Body, params)
		if err != nil {
			return nil, fmt.Errorf("phase=expand path=%s: template: %w", path, err)
		}

		// §5.2: The abstract node's name always takes priority.
		// Setting it here (before recursing) also handles type bodies that
		// declare no name — expandNode requires a non-empty name.
		substituted.Name = r.Name

		return expandNode(substituted, reg, withType(stack, t), path)
	}

	// --- Container: multi-use and/or explicit children ---

	container := &Container{NodeName: r.Name}

	// When the same type appears more than once in Uses (e.g. two independent
	// service instances), consecutive PerType entries for that type must be
	// consumed in order rather than always picking the first match.
	// typeOccurrence tracks how many times each type has been processed so far.
	typeOccurrence := map[string]int{}

	// Expand each type in Uses order, applying params and templates.
	for _, t := range r.Uses {
		typeDef, ok := reg.Get(t)
		if !ok {
			return nil, fmt.Errorf("phase=expand path=%s: %w: %s", path, ErrUnknownType, t)
		}

		// For PerType with, consume entries in declaration order so that the
		// same type can appear in Uses multiple times with distinct params.
		// extractNthCallerParams finds the n-th PerType entry for this type.
		occurrence := typeOccurrence[t]
		typeOccurrence[t]++
		callerParams := extractNthCallerParams(r.With, t, occurrence)
		params, err := applyParamDefs(typeDef.Params, callerParams)
		if err != nil {
			return nil, fmt.Errorf("phase=expand path=%s: %w", path, err)
		}

		substituted, err := applyTemplates(typeDef.Body, params)
		if err != nil {
			return nil, fmt.Errorf("phase=expand path=%s: template: %w", path, err)
		}

		// If the type body declares no name (and no template produced one),
		// fall back to the type name itself so expandNode doesn't reject it.
		if substituted.Name == "" {
			substituted.Name = t
		}

		// Use the substituted name (after template resolution) for the child path.
		childPath := joinPath(path, substituted.Name)

		child, err := expandNode(substituted, reg, withType(stack, t), childPath)
		if err != nil {
			return nil, err
		}
		container.Children = append(container.Children, child)
	}

	// Append explicit children after uses-derived children.
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
	// Names are checked on their resolved (post-template) values.
	seen := map[string]struct{}{}
	for _, child := range container.Children {
		if _, exists := seen[child.Name()]; exists {
			return nil, fmt.Errorf("phase=expand path=%s: %w: %s", path, ErrDuplicateChild, child.Name())
		}
		seen[child.Name()] = struct{}{}
	}

	return container, nil
}

// expandPipeline converts a raw pipeline node into the runtime *Pipeline type.
//
// Each RawStep is converted to a PipelineStep:
//   - Command/Argv → Argv (string form is split via strings.Fields)
//   - Stdin string  → *StepRef (parsed from "steps.<id>.<stream>")
//   - Other fields  → copied directly
//
// Step-reference validation is re-run here so that pipeline nodes originating
// from type expansion (which bypassed Phase 1) are also validated.
func expandPipeline(r RawNode, path string) (*Pipeline, error) {
	// Re-validate steps: catches issues in type-body pipelines that were not
	// seen during Phase 1 (which only validates top-level nodes).
	if err := validateRawSteps(r.Steps, path); err != nil {
		return nil, fmt.Errorf("phase=expand %w", err)
	}

	p := &Pipeline{NodeName: r.Name}

	for i, raw := range r.Steps {
		// Build the final argv from whichever command form was used.
		var argv []string
		if raw.Command != nil {
			// String form: split into tokens (template substitution was already
			// applied by applyTemplatesToStep; step refs are kept as literals).
			argv = strings.Fields(*raw.Command)
		} else {
			argv = raw.Argv
		}

		// Resolve the cwd pointer to a plain string (empty = inherit).
		var cwd string
		if raw.Cwd != nil {
			cwd = *raw.Cwd
		}

		// Parse the optional stdin reference ("steps.<id>.stdout").
		var stdinRef *StepRef
		if raw.Stdin != "" {
			stepID, stream, err := parseStdinRef(raw.Stdin)
			if err != nil {
				return nil, fmt.Errorf("phase=expand path=%s[%d]: stdin: %w", path, i, err)
			}
			stdinRef = &StepRef{StepID: stepID, Stream: stream}
		}

		p.Steps = append(p.Steps, PipelineStep{
			ID:      raw.ID,
			Argv:    argv,
			Cwd:     cwd,
			Env:     raw.Env,
			Capture: raw.Capture,
			Tee:     raw.Tee,
			Stdin:   stdinRef,
			OnFail:  raw.OnFail,
		})
	}

	return p, nil
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

// containsString reports whether slice contains s.
func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
