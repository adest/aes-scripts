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
)
