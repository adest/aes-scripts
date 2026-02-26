package dsl

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"text/template"
)

// stepRefRe matches a step output reference: {{ steps.<id>.<stream> }}
//
// Step output references are execution-time constructs and must NOT be
// processed by the Go template engine during Phase 2 (type param expansion).
// They are escaped before template execution and restored afterward.
//
// Valid examples: {{ steps.bw.stdout }}, {{steps.my-step.stderr}}
var stepRefRe = regexp.MustCompile(`\{\{\s*steps\.([A-Za-z0-9_][A-Za-z0-9_-]*)\.(\w+)\s*\}\}`)

// escapeStepRefs replaces every step output reference in s with a safe
// placeholder so that the Go template engine ignores it.
//
// Returns the modified string and the original references in order,
// so that restoreStepRefs can put them back after template execution.
//
// Placeholders use ASCII NUL bytes as boundaries (e.g. "\x000\x00")
// which cannot appear in valid YAML strings.
func escapeStepRefs(s string) (escaped string, refs []string) {
	escaped = stepRefRe.ReplaceAllStringFunc(s, func(match string) string {
		idx := len(refs)
		refs = append(refs, match)
		return fmt.Sprintf("\x00%d\x00", idx) // NUL-bounded index as placeholder
	})
	return
}

// restoreStepRefs replaces placeholders written by escapeStepRefs with the
// original step output references.
func restoreStepRefs(s string, refs []string) string {
	for i, ref := range refs {
		s = strings.ReplaceAll(s, fmt.Sprintf("\x00%d\x00", i), ref)
	}
	return s
}

// applyTemplates returns a deep copy of r with all string fields substituted
// using Go template syntax and the provided params map.
//
// Fields subject to substitution: Name, Command, Cwd, Env values, nested With
// values, and all Children recursively.
//
// Template syntax: {{ .paramName }}
// The template data is map[string]string, so .paramName resolves params["paramName"].
//
// Substitution happens before child types are expanded, so template values
// (including nested `with` values) flow correctly into nested type expansions.
func applyTemplates(r RawNode, params map[string]string) (RawNode, error) {
	var err error
	// Start with a shallow copy; we'll replace each field that needs substitution.
	out := r

	out.Name, err = substituteString(r.Name, params)
	if err != nil {
		return RawNode{}, fmt.Errorf("name: %w", err)
	}

	if r.Command != nil {
		// String form: apply template to the whole string.
		// Splitting into argv happens later in expand.go, after substitution.
		s, err := substituteString(*r.Command, params)
		if err != nil {
			return RawNode{}, fmt.Errorf("command: %w", err)
		}
		out.Command = &s
	}

	if r.Argv != nil {
		// Array/long form: apply template to each token independently.
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

	// Pipeline steps: apply type-param substitution to each step's string fields.
	// Step output references ({{ steps.X.Y }}) inside those fields are preserved
	// as literals by substituteString — they are execution-time, not expansion-time.
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

// applyTemplatesToStep applies Go template substitution to the string fields
// of a single pipeline step.
//
// The ID and Stdin fields are intentionally left untouched:
//   - ID must be a static identifier (no template expressions allowed).
//   - Stdin is already a step reference string, not a template string.
func applyTemplatesToStep(step RawStep, params map[string]string) (RawStep, error) {
	out := step // shallow copy; we replace fields that need substitution

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

// applyTemplatesToWith substitutes template expressions in all string values
// of a WithBlock. This handles the case where a type body passes templated
// values to a nested type via `with`.
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

// substituteString applies Go template substitution to s using params as the
// template data. Returns s unchanged if it contains no template markers (fast path).
//
// Step output references ({{ steps.<id>.<stream> }}) are preserved as literals:
// they are escaped before template execution and restored afterward, so that the
// Go template engine never sees them. They are resolved at execution time instead.
//
// Using missingkey=error so that a template referencing a param that was not
// resolved surfaces as an explicit error rather than silently producing "<no value>".
func substituteString(s string, params map[string]string) (string, error) {
	// Fast path: skip parsing entirely when there are no template markers.
	if !strings.Contains(s, "{{") {
		return s, nil
	}

	// Escape step output references so the Go template engine does not try
	// to resolve them (they are execution-time, not expansion-time).
	safe, refs := escapeStepRefs(s)

	t, err := template.New("").Option("missingkey=error").Parse(safe)
	if err != nil {
		return "", fmt.Errorf("template parse error in %q: %w", s, err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, params); err != nil {
		return "", fmt.Errorf("template execute error in %q: %w", s, err)
	}

	// Restore the original step output references after template execution.
	return restoreStepRefs(buf.String(), refs), nil
}
