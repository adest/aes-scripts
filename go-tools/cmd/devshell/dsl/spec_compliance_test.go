package dsl

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func strPtr(s string) *string {
	return &s
}

func mustContain(t *testing.T, got string, subs ...string) {
	t.Helper()
	for _, sub := range subs {
		if !strings.Contains(got, sub) {
			t.Fatalf("expected %q to contain %q", got, sub)
		}
	}
}

func snapshotTree(n Node) string {
	var b strings.Builder
	var walk func(Node, string)
	walk = func(node Node, path string) {
		switch x := node.(type) {
		case *Runnable:
			fmt.Fprintf(&b, "R %s cmd=%q\n", path, x.Command)
		case *Container:
			fmt.Fprintf(&b, "C %s\n", path)
			for _, c := range x.Children {
				childPath := joinPath(path, c.Name())
				walk(c, childPath)
			}
		default:
			fmt.Fprintf(&b, "? %s type=%T\n", path, node)
		}
	}
	walk(n, n.Name())
	return b.String()
}

func buildAndGetSingleChild(t *testing.T, reg *Registry, raw RawNode) Node {
	t.Helper()
	eng := NewEngine(reg)
	root, err := eng.Build([]RawNode{raw})
	if err != nil {
		t.Fatalf("unexpected build error: %v", err)
	}
	if root == nil || len(root.Children) != 1 {
		t.Fatalf("expected 1 child under implicit root")
	}
	return root.Children[0]
}

func TestRawNode_XORRule(t *testing.T) {
	t.Run("XOR: command+uses invalid", func(t *testing.T) {
		node := RawNode{Name: "invalid", Command: strPtr("echo test"), Uses: []string{"docker"}}
		err := ValidateRawTree([]RawNode{node})
		if err == nil {
			t.Fatal("expected error for invalid XOR rule")
		}
		mustContain(t, err.Error(), "phase=raw", "path=invalid", "cannot combine")
	})

	t.Run("name required", func(t *testing.T) {
		err := ValidateRawTree([]RawNode{{Command: strPtr("echo")}})
		if err == nil {
			t.Fatal("expected error")
		}
		mustContain(t, err.Error(), "phase=raw", "path=<root>")
	})

	t.Run("runnable command must not be empty", func(t *testing.T) {
		err := ValidateRawTree([]RawNode{{Name: "r", Command: strPtr("   ")}})
		if err == nil {
			t.Fatal("expected error")
		}
		mustContain(t, err.Error(), "phase=raw", "path=r", "command must not be empty")
	})

	t.Run("container must have at least one child", func(t *testing.T) {
		err := ValidateRawTree([]RawNode{{Name: "c", Children: []RawNode{}}})
		if err == nil {
			t.Fatal("expected error")
		}
		mustContain(t, err.Error(), "phase=raw", "path=c", "at least one child")
	})

	t.Run("abstract uses must have at least one entry", func(t *testing.T) {
		err := ValidateRawTree([]RawNode{{Name: "a", Uses: []string{}}})
		if err == nil {
			t.Fatal("expected error")
		}
		mustContain(t, err.Error(), "phase=raw", "path=a", "at least one entry")
	})
}

func TestRawNode_ValidRunnable(t *testing.T) {
	node := RawNode{
		Name:    "build",
		Command: strPtr("go build"),
	}

	err := ValidateRawTree([]RawNode{node})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRawNode_ValidContainer(t *testing.T) {
	node := RawNode{
		Name: "backend",
		Children: []RawNode{
			{Name: "build", Command: strPtr("go build")},
			{Name: "test", Command: strPtr("go test")},
		},
	}

	err := ValidateRawTree([]RawNode{node})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRawNode_AbstractNodeUses(t *testing.T) {
	node := RawNode{
		Name: "stack",
		Uses: []string{"docker-compose"},
	}

	err := ValidateRawTree([]RawNode{node})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRawNode_DuplicateSiblingNames(t *testing.T) {
	node := RawNode{
		Name: "root",
		Children: []RawNode{
			{Name: "build", Command: strPtr("go build")},
			{Name: "build", Command: strPtr("go test")},
		},
	}

	err := ValidateRawTree([]RawNode{node})
	if err == nil {
		t.Fatal("expected error for duplicate sibling names")
	}
}

func TestExpansion_MustProduceValidNode(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register("fake", TypeDefinition{
		Expand: RawNode{
			Name: "generated",
			Children: []RawNode{
				{Name: "run", Command: strPtr("")}, // invalid at runtime (empty command)
			},
		},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	engine := NewEngine(registry)
	raw := []RawNode{
		{Name: "test", Uses: []string{"fake"}},
	}

	_, err := engine.Build(raw)
	if err == nil {
		t.Fatal("expected runtime validation error after expansion")
	}
	mustContain(t, err.Error(), "phase=runtime", "path=test")
}

func TestRuntime_EmptyContainer(t *testing.T) {
	root := &Container{
		NodeName: "root",
		Children: []Node{},
	}

	err := validateRuntimeTree(root)
	if err == nil {
		t.Fatalf("expected error for empty container")
	}
	mustContain(t, err.Error(), "phase=runtime", "at least one child")
}

func TestRuntime_RunnableWithoutCommand(t *testing.T) {
	r := &Runnable{NodeName: "run", Command: ""}
	err := validateRuntimeTree(r)
	if err == nil {
		t.Fatal("expected error for runnable without command")
	}
}

func TestRuntime_ValidTree(t *testing.T) {
	root := &Container{
		NodeName: "root",
		Children: []Node{
			&Runnable{NodeName: "build", Command: "go build"},
			&Container{
				NodeName: "nested",
				Children: []Node{
					&Runnable{NodeName: "test", Command: "go test"},
				},
			},
		},
	}

	err := validateRuntimeTree(root)
	if err != nil {
		t.Fatalf("unexpected error for valid tree: %v", err)
	}
}

func TestSpec_TopLevelRootIsImplicitContainer(t *testing.T) {
	reg := NewRegistry()
	eng := NewEngine(reg)
	cmd := "echo hi"
	root, err := eng.Build([]RawNode{{Name: "x", Command: strPtr(cmd)}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root == nil || root.NodeName != "root" {
		t.Fatalf("expected implicit root container")
	}
	if len(root.Children) != 1 || root.Children[0].Name() != "x" {
		t.Fatalf("expected root to wrap top-level list")
	}
}

func TestSpec_ExpansionRules(t *testing.T) {
	t.Run("unknown type includes phase+path and wraps ErrUnknownType", func(t *testing.T) {
		reg := NewRegistry()
		eng := NewEngine(reg)
		_, err := eng.Build([]RawNode{{Name: "stack", Uses: []string{"missing"}}})
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, ErrUnknownType) {
			t.Fatalf("expected ErrUnknownType, got %v", err)
		}
		mustContain(t, err.Error(), "phase=expand", "path=stack")
	})

	t.Run("cycle detected wraps ErrCycleDetected", func(t *testing.T) {
		reg := NewRegistry()
		if err := reg.Register("A", TypeDefinition{Expand: RawNode{Name: "genA", Uses: []string{"A"}}}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		eng := NewEngine(reg)
		_, err := eng.Build([]RawNode{{Name: "stack", Uses: []string{"A"}}})
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, ErrCycleDetected) {
			t.Fatalf("expected ErrCycleDetected, got %v", err)
		}
		mustContain(t, err.Error(), "phase=expand", "path=stack")
	})

	t.Run("single use can become runnable (name preserved)", func(t *testing.T) {
		reg := NewRegistry()
		cmd := "echo ok"
		if err := reg.Register("T", TypeDefinition{Expand: RawNode{Name: "generated", Command: strPtr(cmd)}}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		n := buildAndGetSingleChild(t, reg, RawNode{Name: "stack", Uses: []string{"T"}})
		r, ok := n.(*Runnable)
		if !ok {
			t.Fatalf("expected Runnable, got %T", n)
		}
		if r.Name() != "stack" {
			t.Fatalf("expected runnable name 'stack', got %q", r.Name())
		}
		if r.Command != cmd {
			t.Fatalf("expected command %q, got %q", cmd, r.Command)
		}
	})

	t.Run("abstract node name overrides type root name (container)", func(t *testing.T) {
		reg := NewRegistry()
		if err := reg.Register("T", TypeDefinition{Expand: RawNode{
			Name: "compose",
			Children: []RawNode{
				{Name: "up", Command: strPtr("echo up")},
				{Name: "down", Command: strPtr("echo down")},
			},
		}}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		n := buildAndGetSingleChild(t, reg, RawNode{Name: "stack", Uses: []string{"T"}})
		c, ok := n.(*Container)
		if !ok {
			t.Fatalf("expected Container, got %T", n)
		}
		if c.Name() != "stack" {
			t.Fatalf("expected expanded node name 'stack', got %q", c.Name())
		}
		if len(c.Children) != 2 {
			t.Fatalf("expected 2 children, got %d", len(c.Children))
		}
	})

	t.Run("multiple uses expanded in order and appended sequentially", func(t *testing.T) {
		reg := NewRegistry()
		if err := reg.Register("A", TypeDefinition{Expand: RawNode{Name: "a", Command: strPtr("echo a")}}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := reg.Register("B", TypeDefinition{Expand: RawNode{Name: "b", Command: strPtr("echo b")}}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		n := buildAndGetSingleChild(t, reg, RawNode{Name: "stack", Uses: []string{"A", "B"}})
		c, ok := n.(*Container)
		if !ok {
			t.Fatalf("expected Container, got %T", n)
		}
		if len(c.Children) != 2 {
			t.Fatalf("expected 2 children, got %d", len(c.Children))
		}
		if c.Children[0].Name() != "a" || c.Children[1].Name() != "b" {
			t.Fatalf("expected order a then b, got %q then %q", c.Children[0].Name(), c.Children[1].Name())
		}
	})

	t.Run("duplicate sibling names after expansion rejected", func(t *testing.T) {
		reg := NewRegistry()
		if err := reg.Register("A", TypeDefinition{Expand: RawNode{Name: "dup", Command: strPtr("echo 1")}}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := reg.Register("B", TypeDefinition{Expand: RawNode{Name: "dup", Command: strPtr("echo 2")}}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		eng := NewEngine(reg)
		_, err := eng.Build([]RawNode{{Name: "stack", Uses: []string{"A", "B"}}})
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, ErrDuplicateChild) {
			t.Fatalf("expected ErrDuplicateChild, got %v", err)
		}
		mustContain(t, err.Error(), "phase=expand", "path=stack")
	})
}

func TestSpec_NameRules_CaseSensitiveUniqueness(t *testing.T) {
	cmd := "echo"
	// build and Build should accept siblings that differ only by case
	reg := NewRegistry()
	eng := NewEngine(reg)
	root, err := eng.Build([]RawNode{{Name: "backend", Children: []RawNode{{Name: "Build", Command: strPtr(cmd)}, {Name: "build", Command: strPtr(cmd)}}}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root == nil {
		t.Fatalf("expected root")
	}
}

func TestSpec_RuntimeValidation_RootMustNotBeEmpty(t *testing.T) {
	reg := NewRegistry()
	eng := NewEngine(reg)
	_, err := eng.Build([]RawNode{})
	if err == nil {
		t.Fatalf("expected runtime error")
	}
	mustContain(t, err.Error(), "phase=runtime", "path=<root>")
}

func TestSpec_Determinism_SameInputSameOutput(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register("T", TypeDefinition{Expand: RawNode{Name: "r", Command: strPtr("echo ok")}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	eng := NewEngine(reg)
	raw := []RawNode{{Name: "stack", Uses: []string{"T"}}}

	root1, err := eng.Build(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	root2, err := eng.Build(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Structure determinism
	if snap1, snap2 := snapshotTree(root1), snapshotTree(root2); snap1 != snap2 {
		t.Fatalf("expected deterministic expansion\n---1---\n%s\n---2---\n%s", snap1, snap2)
	}

	// Also ensure no hidden mutation (re-building doesn't reuse pointers in surprising ways)
	if reflect.ValueOf(root1).Pointer() == reflect.ValueOf(root2).Pointer() {
		t.Fatalf("expected distinct root instances")
	}
}
