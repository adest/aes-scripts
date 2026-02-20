package dsl

type Node interface {
	isNode()
	Name() string
}

type Container struct {
	NodeName string
	Children []Node
}

type Runnable struct {
	NodeName string
	Command  string
}

func (c *Container) isNode() {}
func (r *Runnable) isNode()  {}

func (c *Container) Name() string { return c.NodeName }
func (r *Runnable) Name() string  { return r.NodeName }
