package dsl

import "fmt"

// TypeDef holds a named type definition: its parameter declarations and its
// node body (the template that gets expanded when the type is used).
type TypeDef struct {
	// Params declares the parameters this type accepts.
	// See ParamDefs for the required/optional convention.
	// Params may be nil if the type accepts no parameters.
	Params ParamDefs

	// Inputs declares the runtime inputs this type requires.
	// These are propagated onto the concrete node produced by expansion.
	// Referenced in the type body via {{ inputs.name }}, resolved at execution time.
	// Inputs may be nil if the type declares no runtime inputs.
	Inputs ParamDefs

	// Body is the RawNode that defines the type's structure.
	// All string fields in Body (name, command, cwd, env values, nested with
	// values, and child names) support template substitution:
	//   {{ params.paramName }} — resolved at expansion time (Phase 2)
	//   {{ inputs.inputName }} — preserved as literal, resolved at execution time
	//   {{ steps.id.stream }}  — preserved as literal, resolved at execution time
	Body RawNode
}

// ParamDefs maps parameter names to their default value.
//
//   - nil value  → the parameter is required; the caller MUST supply it.
//   - non-nil    → the parameter is optional with that string as its default.
//
// Numbers declared as defaults are stored as their string representation.
type ParamDefs map[string]*string

// resolveParams validates caller-supplied parameters against the type's
// declarations, applies defaults for omitted optional parameters, and returns
// the final flat map of string values ready for template substitution.
//
// typeName is used only to look up the matching entry in a PerType WithBlock.
func resolveParams(defs ParamDefs, with *WithBlock, typeName string) (map[string]string, error) {
	return applyParamDefs(defs, extractCallerParams(with, typeName))
}

// applyParamDefs is the core of param resolution: given the declared ParamDefs
// and the flat caller-supplied map, it checks for unknown/missing params and
// fills in defaults, returning the final resolved map.
//
// Separated from resolveParams so that expand.go can call it with a
// pre-extracted caller map (needed for nth-occurrence multi-use matching).
func applyParamDefs(defs ParamDefs, callerParams map[string]string) (map[string]string, error) {
	// Reject unknown parameters (strict validation: any param not declared
	// in the type definition is an error).
	for k := range callerParams {
		if _, declared := defs[k]; !declared {
			return nil, fmt.Errorf("%w: %s", ErrUnknownParam, k)
		}
	}

	result := make(map[string]string, len(defs))

	for name, defaultVal := range defs {
		if v, provided := callerParams[name]; provided {
			// Caller explicitly supplied this parameter.
			result[name] = v
		} else if defaultVal != nil {
			// Parameter was omitted; use the declared default.
			result[name] = *defaultVal
		} else {
			// Required parameter not provided.
			return nil, fmt.Errorf("%w: %s", ErrMissingParam, name)
		}
	}

	return result, nil
}

// extractCallerParams returns the flat param map that should be applied to the
// given typeName, based on the content of the WithBlock:
//
//   - nil WithBlock    → returns nil (no params supplied)
//   - Shared form      → returns the shared map for all types
//   - PerType form     → returns the map for the first entry matching typeName
func extractCallerParams(with *WithBlock, typeName string) map[string]string {
	return extractNthCallerParams(with, typeName, 0)
}

// extractNthCallerParams is like extractCallerParams but returns the n-th
// (0-indexed) PerType entry for typeName instead of the first.
//
// This is needed when the same type appears multiple times in a `uses` list:
// each occurrence must consume the corresponding PerType entry in order rather
// than always matching the first one.
func extractNthCallerParams(with *WithBlock, typeName string, n int) map[string]string {
	if with == nil {
		return nil
	}
	if with.Shared != nil {
		return with.Shared
	}
	count := 0
	for _, tw := range with.PerType {
		if tw.Type == typeName {
			if count == n {
				return tw.Params
			}
			count++
		}
	}
	return nil
}
