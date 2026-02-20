package main

import (
	"fmt"
	"log"

	"go-tools/cmd/devshell/dsl"
	"go-tools/cmd/devshell/dslyaml"

	"gopkg.in/yaml.v3"
)

type outNode struct {
	Name     string    `yaml:"name"`
	Command  string    `yaml:"command,omitempty"`
	Children []outNode `yaml:"children,omitempty"`
}

func toOutNode(n dsl.Node) outNode {
	switch x := n.(type) {
	case *dsl.Runnable:
		return outNode{Name: x.Name(), Command: x.Command}
	case *dsl.Container:
		children := make([]outNode, 0, len(x.Children))
		for _, c := range x.Children {
			children = append(children, toOutNode(c))
		}
		return outNode{Name: x.Name(), Children: children}
	default:
		return outNode{Name: n.Name()}
	}
}

func main() {
	// Example 1: full YAML document (types + nodes in a single document)
	exampleYAML := `
types:
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
      - name: backend
        children:
          - name: build
            command: go build ./...
          - name: test
            command: go test ./...
      - name: stack
        uses:
          - docker-compose

  - name: frontend
    command: npm run dev
`
	fmt.Println("--- Example 1: Source YAML ---")
	fmt.Print(exampleYAML)
	fmt.Println("--- Example 1: Derived Model YAML ---")
	printDerived(exampleYAML)

	// Example 2: split YAML (types in one document, nodes in another)
	typesYAML := `
types:
  docker-compose:
    name: compose
    children:
      - name: up
        command: docker compose up -d
      - name: down
        command: docker compose down
`

	nodesYAML := `
- name: app
  children:
    - name: stack
      uses:
        - docker-compose
    - name: backend
      children:
        - name: build
          command: go build ./...
- name: frontend
  command: npm run dev
`

	fmt.Println("--- Example 2: Source YAML (types) ---")
	fmt.Print(typesYAML)
	fmt.Println("--- Example 2: Source YAML (nodes) ---")
	fmt.Print(nodesYAML)
	fmt.Println("--- Example 2: Derived Model YAML ---")
	root, err := dslyaml.BuildMany([]byte(typesYAML), []byte(nodesYAML))
	if err != nil {
		log.Fatalf("engine build: %v", err)
	}
	printDerivedFromRoot(root)
}

func printDerived(exampleYAML string) {
	root, err := dslyaml.Build([]byte(exampleYAML))
	if err != nil {
		log.Fatalf("engine build: %v", err)
	}
	printDerivedFromRoot(root)
}

func printDerivedFromRoot(root *dsl.Container) {
	outs := make([]outNode, 0, len(root.Children))
	for _, n := range root.Children {
		outs = append(outs, toOutNode(n))
	}

	outBytes, err := yaml.Marshal(outs)
	if err != nil {
		log.Fatalf("yaml marshal: %v", err)
	}

	fmt.Print(string(outBytes))
}
