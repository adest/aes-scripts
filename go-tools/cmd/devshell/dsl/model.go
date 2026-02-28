package dsl

// Node is the sealed runtime interface for all nodes in the execution tree.
// Only Container, Runnable, and Pipeline implement it.
// The unexported isNode() method prevents external implementations.
type Node interface {
	isNode()
	Name() string
}

// Container groups child nodes. It is not directly executable.
type Container struct {
	NodeName string
	Children []Node
}

// Runnable defines an executable command.
// Argv holds the argument vector: Argv[0] is the executable, Argv[1:] are the arguments.
// Cwd is optional; empty means use the process working directory.
// Env is optional extra environment variables merged into the process environment.
// Inputs declares the runtime inputs for this node: nil value = required,
// non-nil = optional with that string as the default.
// Elements of Argv, Cwd, and Env values may contain {{ inputs.name }} references
// that are resolved at execution time after inputs are collected.
type Runnable struct {
	NodeName string
	Argv     []string
	Cwd      string
	Env      map[string]string
	Inputs   map[string]*string
}

// Pipeline is an executable node containing an ordered sequence of steps.
//
// Steps are executed synchronously, in declaration order.
// By default, the pipeline is fail-fast: the first failing step stops execution.
// Each step can override this with its own OnFail policy.
// Inputs declares the runtime inputs for this pipeline, collected before execution begins.
type Pipeline struct {
	NodeName string
	Steps    []PipelineStep
	Inputs   map[string]*string
}

// PipelineStep is a single step within a Pipeline: an executable command
// with optional stdin piping, output capture, and failure handling.
//
// Argv holds the final argument vector. Elements may contain step output
// references in the form {{ steps.<id>.stdout }} — these are resolved
// at execution time after the referenced step completes.
type PipelineStep struct {
	ID      string
	Argv    []string
	Cwd     string
	Env     map[string]string
	Capture CaptureMode // which streams to buffer in memory
	Tee     bool        // also forward captured streams to the terminal
	Stdin   *StepRef    // nil = no stdin pipe
	OnFail  OnFail
}

// StepRef identifies the output stream of a named step.
//
// It is used in two ways:
//  1. As the Stdin of a later step: the captured output is fed as process stdin.
//  2. Embedded in argv/env/cwd strings as {{ steps.<id>.stdout }}: resolved
//     at execution time after the referenced step completes.
type StepRef struct {
	StepID string      // id of the step to read from
	Stream CaptureMode // CaptureStdout or CaptureStderr
}

// ---------------------------------------------------------------------------
// isNode / Name implementations
// ---------------------------------------------------------------------------

func (c *Container) isNode() {}
func (r *Runnable) isNode()  {}
func (p *Pipeline) isNode()  {}

func (c *Container) Name() string { return c.NodeName }
func (r *Runnable) Name() string  { return r.NodeName }
func (p *Pipeline) Name() string  { return p.NodeName }

// ---------------------------------------------------------------------------
// Helper accessors
// ---------------------------------------------------------------------------

// Find returns the direct child with the given name, or false if not found.
func (c *Container) Find(name string) (Node, bool) {
	for _, child := range c.Children {
		if child.Name() == name {
			return child, true
		}
	}
	return nil, false
}

// AsRunnable returns the node as a *Runnable.
// The second return value is false if the node is not a Runnable.
func AsRunnable(n Node) (*Runnable, bool) {
	r, ok := n.(*Runnable)
	return r, ok
}

// AsContainer returns the node as a *Container.
// The second return value is false if the node is not a Container.
func AsContainer(n Node) (*Container, bool) {
	c, ok := n.(*Container)
	return c, ok
}

// AsPipeline returns the node as a *Pipeline.
// The second return value is false if the node is not a Pipeline.
func AsPipeline(n Node) (*Pipeline, bool) {
	p, ok := n.(*Pipeline)
	return p, ok
}
