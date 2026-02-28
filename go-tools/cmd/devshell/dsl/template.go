package dsl

import (
	"fmt"
	"regexp"
	"strings"
)

// templateRe matches the three recognized template namespaces:
//
//   - {{ params.name }}  — expansion-time param substitution (Phase 2)
//   - {{ inputs.name }}  — runtime input reference (preserved as literal in Phase 2)
//   - {{ steps.id.stream }} — pipeline step output reference (preserved as literal)
//
// Capture groups:
//
//	[1] namespace: "params", "inputs", or "steps"
//	[2] first name segment (param/input name, or step id)
//	[3] second name segment (stream name, only for "steps")
var templateRe = regexp.MustCompile(
	`\{\{\s*(params|inputs|steps)\.([A-Za-z0-9_][A-Za-z0-9_-]*)(?:\.([A-Za-z0-9_][A-Za-z0-9_-]*))?\s*\}\}`,
)

// anyTemplateRe matches any {{ ... }} expression, used after substitution to
// detect unrecognized template markers that were not processed.
var anyTemplateRe = regexp.MustCompile(`\{\{[^}]*\}\}`)

// applyTemplates returns a deep copy of r with all {{ params.name }} expressions
// substituted using the provided params map.
//
// {{ inputs.name }} and {{ steps.id.stream }} expressions are preserved as
// literals — they are execution-time constructs resolved by the executor.
//
// Fields subject to substitution: Name, Command, Cwd, Env values, nested With
// values, and all Children and Steps recursively.
func applyTemplates(r RawNode, params map[string]string) (RawNode, error) {
	var err error
	out := r

	// Copy Inputs as-is — input declarations are not templates.
	out.Inputs = r.Inputs

	out.Name, err = substituteString(r.Name, params)
	if err != nil {
		return RawNode{}, fmt.Errorf("name: %w", err)
	}

	if r.Command != nil {
		s, err := substituteString(*r.Command, params)
		if err != nil {
			return RawNode{}, fmt.Errorf("command: %w", err)
		}
		out.Command = &s
	}

	if r.Argv != nil {
		out.Argv = make([]string, len(r.Argv))
		for i, token := range r.Argv {
			s, err := substituteString(token, params)
			if err != nil {
				return RawNode{}, fmt.Errorf("command[%d]: %w", i, err)
			}
			out.Argv[i] = s
		}
	}

	if r.Cwd != nil {
		s, err := substituteString(*r.Cwd, params)
		if err != nil {
			return RawNode{}, fmt.Errorf("cwd: %w", err)
		}
		out.Cwd = &s
	}

	if len(r.Env) > 0 {
		out.Env = make(map[string]string, len(r.Env))
		for k, v := range r.Env {
			out.Env[k], err = substituteString(v, params)
			if err != nil {
				return RawNode{}, fmt.Errorf("env[%s]: %w", k, err)
			}
		}
	}

	if len(r.Children) > 0 {
		out.Children = make([]RawNode, len(r.Children))
		for i, c := range r.Children {
			out.Children[i], err = applyTemplates(c, params)
			if err != nil {
				return RawNode{}, err
			}
		}
	}

	if r.With != nil {
		wb, err := applyTemplatesToWith(*r.With, params)
		if err != nil {
			return RawNode{}, fmt.Errorf("with: %w", err)
		}
		out.With = &wb
	}

	if len(r.Steps) > 0 {
		out.Steps = make([]RawStep, len(r.Steps))
		for i, step := range r.Steps {
			s, err := applyTemplatesToStep(step, params)
			if err != nil {
				return RawNode{}, fmt.Errorf("steps[%d]: %w", i, err)
			}
			out.Steps[i] = s
		}
	}

	return out, nil
}

// applyTemplatesToStep applies {{ params.name }} substitution to the string
// fields of a single pipeline step.
//
// ID and Stdin are intentionally left untouched:
//   - ID must be a static identifier (no template expressions allowed).
//   - Stdin is already a step reference string, not a template string.
func applyTemplatesToStep(step RawStep, params map[string]string) (RawStep, error) {
	out := step

	if step.Command != nil {
		s, err := substituteString(*step.Command, params)
		if err != nil {
			return RawStep{}, fmt.Errorf("command: %w", err)
		}
		out.Command = &s
	}

	if step.Argv != nil {
		out.Argv = make([]string, len(step.Argv))
		for i, token := range step.Argv {
			s, err := substituteString(token, params)
			if err != nil {
				return RawStep{}, fmt.Errorf("argv[%d]: %w", i, err)
			}
			out.Argv[i] = s
		}
	}

	if step.Cwd != nil {
		s, err := substituteString(*step.Cwd, params)
		if err != nil {
			return RawStep{}, fmt.Errorf("cwd: %w", err)
		}
		out.Cwd = &s
	}

	if len(step.Env) > 0 {
		out.Env = make(map[string]string, len(step.Env))
		for k, v := range step.Env {
			s, err := substituteString(v, params)
			if err != nil {
				return RawStep{}, fmt.Errorf("env[%s]: %w", k, err)
			}
			out.Env[k] = s
		}
	}

	return out, nil
}

// applyTemplatesToWith substitutes {{ params.name }} in all string values of a
// WithBlock, handling the case where a type body passes templated values to a
// nested type via `with`.
func applyTemplatesToWith(wb WithBlock, params map[string]string) (WithBlock, error) {
	if wb.Shared != nil {
		out := make(map[string]string, len(wb.Shared))
		for k, v := range wb.Shared {
			s, err := substituteString(v, params)
			if err != nil {
				return WithBlock{}, fmt.Errorf("[%s]: %w", k, err)
			}
			out[k] = s
		}
		return WithBlock{Shared: out}, nil
	}

	out := make([]TypedWith, len(wb.PerType))
	for i, tw := range wb.PerType {
		outParams := make(map[string]string, len(tw.Params))
		for k, v := range tw.Params {
			s, err := substituteString(v, params)
			if err != nil {
				return WithBlock{}, fmt.Errorf("[type=%s][%s]: %w", tw.Type, k, err)
			}
			outParams[k] = s
		}
		out[i] = TypedWith{Type: tw.Type, Params: outParams}
	}
	return WithBlock{PerType: out}, nil
}

// substituteString resolves {{ params.name }} expressions in s using the
// provided params map. Returns s unchanged if it contains no {{ markers (fast path).
//
// {{ inputs.name }} and {{ steps.id.stream }} expressions are preserved as
// literals — they are execution-time constructs and must not be resolved here.
//
// Any {{ ... }} expression that does not match a recognized namespace
// (params, inputs, steps) is treated as an error, providing a clear migration
// message for the old {{ .param }} syntax.
func substituteString(s string, params map[string]string) (string, error) {
	if !strings.Contains(s, "{{") {
		return s, nil
	}

	locs := templateRe.FindAllStringSubmatchIndex(s, -1)

	var buf strings.Builder
	last := 0
	for _, loc := range locs {
		// loc[0]:loc[1] = full match
		// loc[2]:loc[3] = namespace
		// loc[4]:loc[5] = first name segment
		// loc[6]:loc[7] = second name segment (may be -1 if absent)
		buf.WriteString(s[last:loc[0]])
		namespace := s[loc[2]:loc[3]]

		switch namespace {
		case "params":
			name := s[loc[4]:loc[5]]
			v, ok := params[name]
			if !ok {
				return "", fmt.Errorf("unknown param %q in %q", name, s)
			}
			buf.WriteString(v)
		case "inputs", "steps":
			// Preserve as literal for execution-time resolution.
			buf.WriteString(s[loc[0]:loc[1]])
		}
		last = loc[1]
	}
	buf.WriteString(s[last:])
	result := buf.String()

	// After substitution, detect any {{ ... }} that remain but were NOT matched
	// by templateRe (e.g. old {{ .param }} syntax). Preserved inputs/steps
	// literals are fine — only truly unrecognized patterns should error.
	if strings.Contains(result, "{{") {
		for _, m := range anyTemplateRe.FindAllString(result, -1) {
			if !templateRe.MatchString(m) {
				return "", fmt.Errorf(
					"unrecognized template expression %q in %q — "+
						"valid forms: {{ params.name }}, {{ inputs.name }}, {{ steps.id.stream }}",
					m, s,
				)
			}
		}
	}

	return result, nil
}
