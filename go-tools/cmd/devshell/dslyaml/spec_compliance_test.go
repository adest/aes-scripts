package dslyaml

import (
	"testing"

	"go-tools/cmd/devshell/dsl"
)

func TestDocumentYAML_TypesAndNodes_BuildsWithEmptyRegistry(t *testing.T) {
	yml := `types:
  docker-compose:
    name: compose
    children:
      - name: up
        command: docker compose up -d
      - name: down
        command: docker compose down

nodes:
  - name: app
    children:
      - name: stack
        uses:
          - docker-compose
`

	doc, err := Parse([]byte(yml))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(doc.Types) != 1 {
		t.Fatalf("expected 1 type, got %d", len(doc.Types))
	}
	if len(doc.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(doc.Nodes))
	}

	root, err := Build([]byte(yml))
	if err != nil {
		t.Fatalf("unexpected build error: %v", err)
	}
	if root == nil || len(root.Children) != 1 {
		t.Fatalf("expected a root with one child")
	}

	app, ok := root.Children[0].(*dsl.Container)
	if !ok {
		t.Fatalf("expected app to be Container, got %T", root.Children[0])
	}
	if app.Name() != "app" {
		t.Fatalf("expected app name 'app', got %q", app.Name())
	}
	if len(app.Children) != 1 {
		t.Fatalf("expected app to have 1 child, got %d", len(app.Children))
	}
	stack, ok := app.Children[0].(*dsl.Container)
	if !ok {
		t.Fatalf("expected stack to be Container, got %T", app.Children[0])
	}
	if stack.Name() != "stack" {
		t.Fatalf("expected abstract node name preserved as 'stack', got %q", stack.Name())
	}
	if stack.Name() == "compose" {
		t.Fatalf("expected abstract node name to override type root name")
	}
	if len(stack.Children) != 2 {
		t.Fatalf("expected expanded stack to have 2 children, got %d", len(stack.Children))
	}
	if stack.Children[0].Name() != "up" || stack.Children[1].Name() != "down" {
		t.Fatalf("expected expanded children up/down, got %q/%q", stack.Children[0].Name(), stack.Children[1].Name())
	}
}

func TestDocumentYAML_ShorthandNodesOnly(t *testing.T) {
	yml := `- name: frontend
  command: npm run dev
`
	doc, err := Parse([]byte(yml))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(doc.Types) != 0 {
		t.Fatalf("expected no types")
	}
	if len(doc.Nodes) != 1 {
		t.Fatalf("expected 1 node")
	}
}

func TestDocumentYAML_MissingNodesIsError(t *testing.T) {
	yml := `types:
  t:
    name: x
    command: echo ok
`
	_, err := Build([]byte(yml))
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestDocumentYAML_BuildMany_MergesTypesAndConcatsNodes(t *testing.T) {
	typesYML := `types:
  t:
    name: r
    command: echo ok
`
	nodesYML := `- name: stack
  uses:
    - t
`

	root, err := BuildMany([]byte(typesYML), []byte(nodesYML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root == nil || len(root.Children) != 1 {
		t.Fatalf("expected root with 1 child")
	}
	if root.Children[0].Name() != "stack" {
		t.Fatalf("expected node name stack, got %q", root.Children[0].Name())
	}
}
