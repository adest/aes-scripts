package dsl

// Node is the sealed runtime interface for all nodes in the execution tree.
// Only Container and Runnable implement it.
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
type Runnable struct {
	NodeName string
	Argv     []string
	Cwd      string
	Env      map[string]string
}

func (c *Container) isNode() {}
func (r *Runnable) isNode()  {}

func (c *Container) Name() string { return c.NodeName }
func (r *Runnable) Name() string  { return r.NodeName }

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
