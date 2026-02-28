package dsl

import "errors"

var (
	ErrTypeAlreadyExists = errors.New("type already exists")
	ErrUnknownType       = errors.New("unknown type")
	ErrCycleDetected     = errors.New("cycle detected")
	ErrDuplicateChild    = errors.New("duplicate child name")
	ErrInvalidNode       = errors.New("invalid node definition")
	ErrMissingParam      = errors.New("missing required param")
	ErrUnknownParam      = errors.New("unknown param")

	// Runtime input errors.
	ErrMissingInput = errors.New("missing required input")
	ErrUnknownInput = errors.New("unknown input") // referenced but not declared in inputs block

	// Pipeline step errors.
	ErrStepRefBadFormat  = errors.New("invalid step reference format")       // e.g. wrong syntax for steps.<id>.<stream>
	ErrStepRefUnknown    = errors.New("step reference to unknown id")        // no step with that id
	ErrStepRefForward    = errors.New("step reference to a later step")      // forward references are forbidden
	ErrStepRefUncaptured = errors.New("step reference to uncaptured stream") // referenced step does not capture that stream
)
