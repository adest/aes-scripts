package dsl

// ---------------------------------------------------------------------------
// Pipeline step types
// ---------------------------------------------------------------------------

// CaptureMode describes which output streams of a step are buffered in memory
// so that later steps can reference them.
//
// The zero value (CaptureNone) means no capture: all output goes to the
// terminal as usual and cannot be referenced by other steps.
type CaptureMode int

const (
	CaptureNone   CaptureMode = iota // default: output is not captured
	CaptureStdout                     // buffer stdout only
	CaptureStderr                     // buffer stderr only
	CaptureBoth                       // buffer both stdout and stderr
)

// Includes reports whether m captures the given single-stream mode.
// s must be CaptureStdout or CaptureStderr.
func (m CaptureMode) Includes(s CaptureMode) bool {
	if s == CaptureStdout {
		return m == CaptureStdout || m == CaptureBoth
	}
	return m == CaptureStderr || m == CaptureBoth
}

// OnFail describes the failure policy of a single pipeline step.
//
// The zero value means fail-fast: a non-zero exit code stops the pipeline.
type OnFail struct {
	// Action is one of "fail" (default), "continue", or "retry".
	// An empty string is treated as "fail".
	Action string

	// Attempts is the total number of executions for "retry" mode,
	// including the initial attempt (minimum: 2).
	Attempts int

	// Delay is the duration to wait between retry attempts (e.g. "2s", "500ms").
	// An empty string means no delay between attempts.
	Delay string
}

// RawStep is the unvalidated representation of a single step in a pipeline,
// as parsed from an external source (e.g. YAML).
//
// Command handling follows the same three-form convention as RawNode:
//   - Command (*string): string form — template applied first, then split into argv.
//   - Argv ([]string): array/long form — already tokenized, template per-element.
type RawStep struct {
	// ID is an optional unique identifier for this step within the pipeline.
	// Required when the step declares Capture (so other steps can reference it).
	// Must be a static value — no {{ }} template expressions.
	ID string

	Command *string           // string form: template applied, then split
	Argv    []string          // array/long form: already tokenized
	Cwd     *string           // optional working directory
	Env     map[string]string // optional extra environment variables

	// Capture declares which output streams to buffer in memory.
	// Requires ID to be set (otherwise the captured output is unreachable).
	Capture CaptureMode

	// Tee, when true, forwards captured streams to the terminal as well.
	// Only valid when Capture is set.
	Tee bool

	// Stdin is a step output reference in the format "steps.<id>.stdout"
	// or "steps.<id>.stderr". The referenced step must appear before this
	// step in the list and must have a compatible Capture mode.
	Stdin string

	// OnFail controls what happens when this step exits with a non-zero code.
	// The zero value means fail-fast.
	OnFail OnFail
}

// ---------------------------------------------------------------------------
// RawNode
// ---------------------------------------------------------------------------

// RawNode represents a node as parsed from an external source (e.g. YAML).
// It is intentionally format-agnostic: no serialization tags.
//
// A runnable node carries exactly one of Command or Argv — never both:
//
//   - Command (*string): the compact string form. Template substitution is applied
//     to the whole string first; the result is split into argv with strings.Fields
//     during expansion. This preserves template expressions that span tokens
//     (e.g. "docker compose -f {{ params.file }} up -d").
//
//   - Argv ([]string): the pre-tokenized form, produced by either the array form
//     ("command: [...]") or the long form ("command: exe" + "args: [...]").
//     Template substitution is applied to each element independently.
type RawNode struct {
	Name     string
	Command  *string  // string form: template applied to whole string, then split
	Argv     []string // array/long form: already tokenized; template applied per-element
	Cwd      *string
	Env      map[string]string
	Children []RawNode
	Uses     []string

	// Steps holds the ordered list of steps for a pipeline node.
	// A node with Steps set is a pipeline: it is executable, not a container.
	// Exactly one of Command/Argv, Children, Uses, or Steps must be set (XOR).
	Steps []RawStep

	// With holds parameters passed to the types referenced in Uses.
	// It is nil when the node has no `with` block.
	With *WithBlock

	// Inputs declares the runtime inputs for this node.
	// Only valid on runnable nodes (command) and pipeline nodes (steps).
	// Forbidden on container nodes (children) and abstract nodes (uses).
	// nil value → required input; non-nil string → optional with that default.
	// Referenced in commands via {{ inputs.name }}, resolved at execution time.
	Inputs ParamDefs
}

// WithBlock holds the parameters passed to a type via the `with` key.
// Exactly one of Shared or PerType is populated:
//
//   - Shared: a flat key/value map, shared across all types listed in Uses.
//     YAML form: `with: { key: value, ... }`
//
//   - PerType: a list of per-type param sets, used when different types in
//     Uses need different parameters.
//     YAML form: `with: [ { type: foo, key: value }, ... ]`
type WithBlock struct {
	Shared  map[string]string // mapping form
	PerType []TypedWith       // list form
}

// TypedWith associates a set of parameters with a specific type name.
// It is used in the list form of WithBlock.
type TypedWith struct {
	Type   string
	Params map[string]string
}
