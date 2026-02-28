package dsl

import (
	"fmt"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Phase 1 — raw tree validation
// ---------------------------------------------------------------------------

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
// constraints, `with` validity, sibling uniqueness, and recursive child validation.
func validateRawNode(n RawNode, siblings map[string]struct{}, path string) error {
	if n.Name == "" {
		return fmt.Errorf("phase=raw path=%s: node is missing a name", path)
	}
	if _, exists := siblings[n.Name]; exists {
		return fmt.Errorf("phase=raw path=%s: duplicate sibling name: %s", path, n.Name)
	}
	siblings[n.Name] = struct{}{}

	hasCommand := n.Command != nil || n.Argv != nil
	hasChildren := n.Children != nil
	hasUses := n.Uses != nil
	hasSteps := n.Steps != nil

	// XOR rule: exactly one of command, children, uses, steps.
	count := 0
	for _, b := range []bool{hasCommand, hasChildren, hasUses, hasSteps} {
		if b {
			count++
		}
	}
	switch count {
	case 0:
		return fmt.Errorf("phase=raw path=%s: node must define exactly one of: command, children, uses, or steps", path)
	case 1:
		// valid, fall through to content checks
	default:
		return fmt.Errorf("phase=raw path=%s: node cannot combine command, children, uses, and steps", path)
	}

	// `inputs` is only valid on runnable and pipeline nodes.
	if n.Inputs != nil && (hasChildren || hasUses) {
		return fmt.Errorf("phase=raw path=%s: 'inputs' can only be declared on runnable or pipeline nodes", path)
	}

	// `with` is only valid on abstract nodes (nodes that have `uses`).
	if n.With != nil && !hasUses {
		return fmt.Errorf("phase=raw path=%s: 'with' can only be used on abstract nodes (requires 'uses')", path)
	}

	// In list form, each entry must declare a non-empty `type`.
	if n.With != nil {
		for _, tw := range n.With.PerType {
			if tw.Type == "" {
				return fmt.Errorf("phase=raw path=%s: 'with' list item is missing 'type'", path)
			}
		}
	}

	switch {
	case hasCommand:
		empty := false
		if n.Command != nil {
			empty = strings.TrimSpace(*n.Command) == ""
		} else {
			empty = len(n.Argv) == 0 || strings.TrimSpace(n.Argv[0]) == ""
		}
		if empty {
			return fmt.Errorf("phase=raw path=%s: runnable command must not be empty", path)
		}
		// Validate that all {{ inputs.name }} refs are declared in the inputs block.
		if n.Command != nil {
			if err := validateInputRefsInString(*n.Command, n.Inputs, path, "command"); err != nil {
				return err
			}
		}
		for i, token := range n.Argv {
			if err := validateInputRefsInString(token, n.Inputs, path, fmt.Sprintf("argv[%d]", i)); err != nil {
				return err
			}
		}
		if n.Cwd != nil {
			if err := validateInputRefsInString(*n.Cwd, n.Inputs, path, "cwd"); err != nil {
				return err
			}
		}
		for k, v := range n.Env {
			if err := validateInputRefsInString(v, n.Inputs, path, "env["+k+"]"); err != nil {
				return err
			}
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

	case hasSteps:
		if err := validateRawSteps(n.Steps, path, n.Inputs); err != nil {
			return err
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// Pipeline step validation (reused in Phase 1 and during expansion)
// ---------------------------------------------------------------------------

// priorStepInfo records what we know about a step that has already been
// validated, so later steps can check their references against it.
type priorStepInfo struct {
	capture CaptureMode
}

// validateRawSteps validates the steps of a pipeline node.
//
// It checks structural rules (command, id uniqueness, capture, tee, on-fail)
// and reference rules (stdin and {{ steps.X.Y }} args must point to prior steps
// with compatible capture modes).
//
// This function is called from Phase 1 for top-level pipeline nodes, and from
// expandPipeline for type-body pipeline nodes after template substitution.
func validateRawSteps(steps []RawStep, path string, inputDefs ParamDefs) error {
	if len(steps) == 0 {
		return fmt.Errorf("phase=raw path=%s: pipeline must have at least one step", path)
	}

	// priorByID collects the IDs of steps seen so far, together with their
	// capture mode. We build this incrementally so that forward-reference
	// checks are automatically correct: a step can only reference IDs that
	// were added before it was processed.
	priorByID := make(map[string]priorStepInfo)

	for i, step := range steps {
		stepPath := fmt.Sprintf("%s[%d]", path, i)
		if step.ID != "" {
			stepPath = fmt.Sprintf("%s[%s]", path, step.ID)
		}

		// --- command ---
		empty := false
		if step.Command != nil {
			empty = strings.TrimSpace(*step.Command) == ""
		} else if step.Argv != nil {
			empty = len(step.Argv) == 0 || strings.TrimSpace(step.Argv[0]) == ""
		} else {
			empty = true // neither Command nor Argv set
		}
		if empty {
			return fmt.Errorf("phase=raw path=%s: step command must not be empty", stepPath)
		}

		// --- id ---
		if step.ID != "" {
			if strings.Contains(step.ID, "{{") {
				return fmt.Errorf("phase=raw path=%s: step id must be a static identifier (no {{ }} expressions)", stepPath)
			}
			if _, dup := priorByID[step.ID]; dup {
				return fmt.Errorf("phase=raw path=%s: %w: %s", stepPath, ErrStepRefUnknown, step.ID)
			}
			// Check for duplicate IDs by re-scanning (priorByID would have caught it above).
		}

		// --- capture ---
		if step.Capture != CaptureNone {
			if step.ID == "" {
				return fmt.Errorf("phase=raw path=%s: capture requires id (captured output would be unreachable without an id)", stepPath)
			}
		}

		// --- tee ---
		if step.Tee && step.Capture == CaptureNone {
			return fmt.Errorf("phase=raw path=%s: tee requires capture", stepPath)
		}

		// --- stdin ---
		if step.Stdin != "" {
			refID, stream, err := parseStdinRef(step.Stdin)
			if err != nil {
				return fmt.Errorf("phase=raw path=%s: stdin: %w", stepPath, err)
			}
			if err := checkStepRef(refID, stream, priorByID, stepPath, "stdin"); err != nil {
				return err
			}
		}

		// --- step refs in argv ---
		argv := step.Argv
		if step.Command != nil {
			argv = []string{*step.Command}
		}
		for _, token := range argv {
			if err := validateStepRefsInString(token, priorByID, stepPath, "command/args"); err != nil {
				return err
			}
		}

		// --- step refs in env values ---
		for k, v := range step.Env {
			if err := validateStepRefsInString(v, priorByID, stepPath, "env["+k+"]"); err != nil {
				return err
			}
		}

		// --- step refs in cwd ---
		if step.Cwd != nil {
			if err := validateStepRefsInString(*step.Cwd, priorByID, stepPath, "cwd"); err != nil {
				return err
			}
		}

		// --- input refs in command/argv, env, cwd ---
		for _, token := range argv {
			if err := validateInputRefsInString(token, inputDefs, stepPath, "command/args"); err != nil {
				return err
			}
		}
		for k, v := range step.Env {
			if err := validateInputRefsInString(v, inputDefs, stepPath, "env["+k+"]"); err != nil {
				return err
			}
		}
		if step.Cwd != nil {
			if err := validateInputRefsInString(*step.Cwd, inputDefs, stepPath, "cwd"); err != nil {
				return err
			}
		}

		// --- on-fail ---
		if err := validateOnFail(step.OnFail, stepPath); err != nil {
			return err
		}

		// Register this step so subsequent steps can reference it.
		if step.ID != "" {
			priorByID[step.ID] = priorStepInfo{capture: step.Capture}
		}
	}

	return nil
}

// validateOnFail checks that the OnFail struct is consistent with the spec.
func validateOnFail(of OnFail, path string) error {
	switch of.Action {
	case "", "fail", "continue":
		// valid; no extra fields expected
		if of.Attempts != 0 {
			return fmt.Errorf("phase=raw path=%s: on-fail: attempts is only valid with action: retry", path)
		}
		if of.Delay != "" {
			return fmt.Errorf("phase=raw path=%s: on-fail: delay is only valid with action: retry", path)
		}
	case "retry":
		if of.Attempts < 2 {
			return fmt.Errorf("phase=raw path=%s: on-fail: attempts must be >= 2 for retry (got %d)", path, of.Attempts)
		}
		if of.Delay != "" {
			if _, err := time.ParseDuration(of.Delay); err != nil {
				return fmt.Errorf("phase=raw path=%s: on-fail: delay %q is not a valid duration (e.g. '2s', '500ms'): %w", path, of.Delay, err)
			}
		}
	default:
		return fmt.Errorf("phase=raw path=%s: on-fail: action must be 'fail', 'continue', or 'retry' (got %q)", path, of.Action)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Step reference helpers
// ---------------------------------------------------------------------------

// parseStdinRef parses a stdin step reference of the form "steps.<id>.<stream>".
// Returns the step ID and stream (CaptureStdout or CaptureStderr), or an error.
func parseStdinRef(s string) (stepID string, stream CaptureMode, err error) {
	parts := strings.SplitN(s, ".", 3)
	if len(parts) != 3 || parts[0] != "steps" {
		return "", CaptureNone, fmt.Errorf("%w: %q (expected steps.<id>.stdout or steps.<id>.stderr)", ErrStepRefBadFormat, s)
	}
	stepID = parts[1]
	if stepID == "" {
		return "", CaptureNone, fmt.Errorf("%w: step id is empty in %q", ErrStepRefBadFormat, s)
	}
	switch parts[2] {
	case "stdout":
		return stepID, CaptureStdout, nil
	case "stderr":
		return stepID, CaptureStderr, nil
	default:
		return "", CaptureNone, fmt.Errorf("%w: stream must be 'stdout' or 'stderr' in %q (got %q)", ErrStepRefBadFormat, s, parts[2])
	}
}

// validateStepRefsInString finds all {{ steps.<id>.<stream> }} patterns in s
// and validates each one against the prior steps map.
func validateStepRefsInString(s string, priorByID map[string]priorStepInfo, path, field string) error {
	matches := templateRe.FindAllStringSubmatch(s, -1)
	for _, m := range matches {
		// m[1] = namespace, m[2] = first segment, m[3] = second segment
		if m[1] != "steps" {
			continue
		}
		id := m[2]
		streamStr := m[3]
		var stream CaptureMode
		switch streamStr {
		case "stdout":
			stream = CaptureStdout
		case "stderr":
			stream = CaptureStderr
		default:
			return fmt.Errorf("phase=raw path=%s: %s: %w: stream must be stdout or stderr (got %q)", path, field, ErrStepRefBadFormat, streamStr)
		}
		if err := checkStepRef(id, stream, priorByID, path, field); err != nil {
			return err
		}
	}
	return nil
}

// validateInputRefsInString finds all {{ inputs.<name> }} patterns in s and
// checks that each name is declared in inputDefs. If inputDefs is nil (no
// inputs block declared), any {{ inputs.name }} reference is an error.
func validateInputRefsInString(s string, inputDefs ParamDefs, path, field string) error {
	matches := templateRe.FindAllStringSubmatch(s, -1)
	for _, m := range matches {
		if m[1] != "inputs" {
			continue
		}
		name := m[2]
		if _, declared := inputDefs[name]; !declared {
			return fmt.Errorf("phase=raw path=%s: %s: %w: %s", path, field, ErrUnknownInput, name)
		}
	}
	return nil
}

// checkStepRef validates that a step reference (id + stream) points to a
// prior step that captures the expected stream.
func checkStepRef(id string, stream CaptureMode, priorByID map[string]priorStepInfo, path, field string) error {
	prior, exists := priorByID[id]
	if !exists {
		// Could be either unknown or forward — we report it as unknown.
		// (If it exists later in the list, the caller would have added it after the
		// current step, so it correctly shows up as missing here.)
		return fmt.Errorf("phase=raw path=%s: %s: %w: %q (did you mean to reference a step that comes later?)", path, field, ErrStepRefUnknown, id)
	}
	if !prior.capture.Includes(stream) {
		return fmt.Errorf("phase=raw path=%s: %s: %w: step %q does not capture %s", path, field, ErrStepRefUncaptured, id, streamName(stream))
	}
	return nil
}

// streamName returns the human-readable stream name for error messages.
func streamName(m CaptureMode) string {
	switch m {
	case CaptureStdout:
		return "stdout"
	case CaptureStderr:
		return "stderr"
	default:
		return "unknown"
	}
}

// ---------------------------------------------------------------------------
// Phase 3 — runtime tree validation
// ---------------------------------------------------------------------------

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
		if len(node.Argv) == 0 || strings.TrimSpace(node.Argv[0]) == "" {
			return fmt.Errorf("phase=runtime path=%s: runnable command must not be empty", path)
		}
	case *Pipeline:
		if len(node.Steps) == 0 {
			return fmt.Errorf("phase=runtime path=%s: pipeline must have at least one step", path)
		}
		for i, step := range node.Steps {
			if len(step.Argv) == 0 || strings.TrimSpace(step.Argv[0]) == "" {
				return fmt.Errorf("phase=runtime path=%s: step[%d] command must not be empty", path, i)
			}
		}
	default:
		return fmt.Errorf("phase=runtime path=%s: unknown node type: %T", path, n)
	}
	return nil
}
