package dsl

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

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
		s, err := substituteString(*r.Command, params)
		if err != nil {
			return RawNode{}, fmt.Errorf("command: %w", err)
		}
		out.Command = &s
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
// Using missingkey=error so that a template referencing a param that was not
// resolved surfaces as an explicit error rather than silently producing "<no value>".
func substituteString(s string, params map[string]string) (string, error) {
	// Fast path: skip parsing entirely when there are no template markers.
	if !strings.Contains(s, "{{") {
		return s, nil
	}

	t, err := template.New("").Option("missingkey=error").Parse(s)
	if err != nil {
		return "", fmt.Errorf("template parse error in %q: %w", s, err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, params); err != nil {
		return "", fmt.Errorf("template execute error in %q: %w", s, err)
	}

	return buf.String(), nil
}
