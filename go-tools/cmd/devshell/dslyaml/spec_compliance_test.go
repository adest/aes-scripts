package dslyaml

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"go-tools/cmd/devshell/dsl"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func requireBuildOK(t *testing.T, yml string) *dsl.Container {
	t.Helper()
	root, err := Build([]byte(yml))
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	return root
}

func requireBuildErr(t *testing.T, yml string, wantSubstrs ...string) error {
	t.Helper()
	_, err := Build([]byte(yml))
	if err == nil {
		t.Fatalf("expected error but got none")
	}
	for _, sub := range wantSubstrs {
		if !strings.Contains(err.Error(), sub) {
			t.Errorf("error %q does not contain %q", err.Error(), sub)
		}
	}
	return err
}

func requireParseOK(t *testing.T, yml string) Document {
	t.Helper()
	doc, err := Parse([]byte(yml))
	if err != nil {
		t.Fatalf("expected parse success, got: %v", err)
	}
	return doc
}

func requireParseErr(t *testing.T, yml string, wantSubstrs ...string) {
	t.Helper()
	_, err := Parse([]byte(yml))
	if err == nil {
		t.Fatalf("expected parse error but got none")
	}
	for _, sub := range wantSubstrs {
		if !strings.Contains(err.Error(), sub) {
			t.Errorf("parse error %q does not contain %q", err.Error(), sub)
		}
	}
}

func requireRunnable(t *testing.T, node dsl.Node, wantName, wantCmd string) *dsl.Runnable {
	t.Helper()
	r, ok := node.(*dsl.Runnable)
	if !ok {
		t.Fatalf("expected *dsl.Runnable, got %T", node)
	}
	if r.Name() != wantName {
		t.Errorf("runnable name: want %q, got %q", wantName, r.Name())
	}
	gotCmd := strings.Join(r.Argv, " ")
	if gotCmd != wantCmd {
		t.Errorf("runnable argv: want %q, got %q (argv=%v)", wantCmd, gotCmd, r.Argv)
	}
	return r
}

func requireContainer(t *testing.T, node dsl.Node, wantName string, wantChildCount int) *dsl.Container {
	t.Helper()
	c, ok := node.(*dsl.Container)
	if !ok {
		t.Fatalf("expected *dsl.Container, got %T", node)
	}
	if c.Name() != wantName {
		t.Errorf("container name: want %q, got %q", wantName, c.Name())
	}
	if len(c.Children) != wantChildCount {
		t.Errorf("container %q child count: want %d, got %d", wantName, wantChildCount, len(c.Children))
	}
	return c
}

// requirePipeline asserts that node is a *dsl.Pipeline with the expected name
// and number of steps, and returns it for further inspection.
func requirePipeline(t *testing.T, node dsl.Node, wantName string, wantStepCount int) *dsl.Pipeline {
	t.Helper()
	p, ok := node.(*dsl.Pipeline)
	if !ok {
		t.Fatalf("expected *dsl.Pipeline, got %T", node)
	}
	if p.Name() != wantName {
		t.Errorf("pipeline name: want %q, got %q", wantName, p.Name())
	}
	if len(p.Steps) != wantStepCount {
		t.Errorf("pipeline %q step count: want %d, got %d", wantName, wantStepCount, len(p.Steps))
	}
	return p
}

// snapshotTree produces a deterministic string representation of the runtime tree.
func snapshotTree(n dsl.Node) string {
	var b strings.Builder
	var walk func(dsl.Node, string)
	walk = func(node dsl.Node, path string) {
		switch x := node.(type) {
		case *dsl.Runnable:
			fmt.Fprintf(&b, "R %s cmd=%q cwd=%q\n", path, strings.Join(x.Argv, " "), x.Cwd)
		case *dsl.Container:
			fmt.Fprintf(&b, "C %s\n", path)
			for _, c := range x.Children {
				walk(c, path+"."+c.Name())
			}
		case *dsl.Pipeline:
			fmt.Fprintf(&b, "P %s steps=%d\n", path, len(x.Steps))
		}
	}
	walk(n, n.Name())
	return b.String()
}

// ---------------------------------------------------------------------------
// §2 — Top-level structure
// ---------------------------------------------------------------------------

func TestParse_Section2_TopLevelStructure(t *testing.T) {
	t.Run("shorthand: bare list is parsed as nodes with empty types", func(t *testing.T) {
		yml := `
- name: build
  command: go build ./...
`
		doc := requireParseOK(t, yml)
		if len(doc.Types) != 0 {
			t.Errorf("expected no types, got %d", len(doc.Types))
		}
		if len(doc.Nodes) != 1 {
			t.Errorf("expected 1 node, got %d", len(doc.Nodes))
		}
		if doc.Nodes[0].Name != "build" {
			t.Errorf("expected node name 'build', got %q", doc.Nodes[0].Name)
		}
	})

	t.Run("document form: mapping with types and nodes", func(t *testing.T) {
		yml := `
types:
  my-type:
    name: gen
    command: echo generated

nodes:
  - name: app
    uses:
      - my-type
`
		doc := requireParseOK(t, yml)
		if len(doc.Types) != 1 {
			t.Errorf("expected 1 type, got %d", len(doc.Types))
		}
		if _, ok := doc.Types["my-type"]; !ok {
			t.Errorf("expected type 'my-type' to be present")
		}
		if len(doc.Nodes) != 1 {
			t.Errorf("expected 1 node, got %d", len(doc.Nodes))
		}
	})

	t.Run("document form: multiple types registered", func(t *testing.T) {
		yml := `
types:
  type-a:
    name: a
    command: echo a
  type-b:
    name: b
    command: echo b

nodes:
  - name: x
    command: echo x
`
		doc := requireParseOK(t, yml)
		if len(doc.Types) != 2 {
			t.Errorf("expected 2 types, got %d", len(doc.Types))
		}
	})

	t.Run("document form with types but no nodes key → error", func(t *testing.T) {
		yml := `
types:
  t:
    name: x
    command: echo ok
`
		requireBuildErr(t, yml, "missing or empty 'nodes'")
	})

	t.Run("document form with explicit empty nodes list → error", func(t *testing.T) {
		yml := `
types:
  t:
    name: x
    command: echo ok
nodes: []
`
		requireBuildErr(t, yml, "missing or empty 'nodes'")
	})

	t.Run("empty YAML → parse error", func(t *testing.T) {
		requireParseErr(t, "", "phase=parse")
	})

	t.Run("YAML scalar root → parse error", func(t *testing.T) {
		requireParseErr(t, "just a string", "phase=parse", "unexpected YAML root kind")
	})

	t.Run("invalid YAML → parse error", func(t *testing.T) {
		requireParseErr(t, "[broken: yaml: :")
	})

	t.Run("shorthand: multiple top-level nodes", func(t *testing.T) {
		yml := `
- name: build
  command: go build
- name: test
  command: go test ./...
- name: lint
  command: golangci-lint run
`
		doc := requireParseOK(t, yml)
		if len(doc.Nodes) != 3 {
			t.Errorf("expected 3 nodes, got %d", len(doc.Nodes))
		}
	})
}

// ---------------------------------------------------------------------------
// §3 — XOR rule (raw validation)
// ---------------------------------------------------------------------------

func TestBuild_Section3_XORRule(t *testing.T) {
	cases := []struct {
		name        string
		yml         string
		wantErr     bool
		wantSubstrs []string
	}{
		{
			name: "command only: valid",
			yml: `
- name: build
  command: go build
`,
		},
		{
			name: "children only: valid",
			yml: `
- name: backend
  children:
    - name: build
      command: go build
`,
		},
		{
			name: "uses only: valid (raw)",
			yml: `
types:
  t:
    name: r
    command: echo ok
nodes:
  - name: stack
    uses:
      - t
`,
		},
		{
			name: "command + children: invalid",
			yml: `
- name: invalid
  command: go build
  children:
    - name: child
      command: go test
`,
			wantErr:     true,
			wantSubstrs: []string{"phase=raw", "cannot combine"},
		},
		{
			name: "command + uses: invalid",
			yml: `
- name: invalid
  command: go build
  uses:
    - some-type
`,
			wantErr:     true,
			wantSubstrs: []string{"phase=raw", "cannot combine"},
		},
		{
			name: "children + uses: invalid",
			yml: `
- name: invalid
  children:
    - name: child
      command: go test
  uses:
    - some-type
`,
			wantErr:     true,
			wantSubstrs: []string{"phase=raw", "cannot combine"},
		},
		{
			name: "command + children + uses: all three: invalid",
			yml: `
- name: invalid
  command: go build
  children:
    - name: child
      command: go test
  uses:
    - some-type
`,
			wantErr:     true,
			wantSubstrs: []string{"phase=raw", "cannot combine"},
		},
		{
			name: "none of the three: invalid",
			yml: `
- name: empty-node
`,
			wantErr:     true,
			wantSubstrs: []string{"phase=raw", "must define exactly one"},
		},
		{
			name: "missing name at root level",
			yml: `
- command: go build
`,
			wantErr:     true,
			wantSubstrs: []string{"phase=raw", "missing a name", "path=<root>"},
		},
		{
			name: "empty name string",
			yml: `
- name: ""
  command: go build
`,
			wantErr:     true,
			wantSubstrs: []string{"phase=raw", "missing a name"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.wantErr {
				requireBuildErr(t, tc.yml, tc.wantSubstrs...)
			} else {
				requireBuildOK(t, tc.yml)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// §3.1 — Runnable node
// ---------------------------------------------------------------------------

func TestBuild_Section3_1_RunnableNode(t *testing.T) {
	t.Run("basic runnable: command propagated to runtime model", func(t *testing.T) {
		yml := `
- name: build
  command: go build ./...
`
		root := requireBuildOK(t, yml)
		requireRunnable(t, root.Children[0], "build", "go build ./...")
	})

	t.Run("empty command string → raw error", func(t *testing.T) {
		yml := `
- name: build
  command: ""
`
		requireBuildErr(t, yml, "phase=raw", "command must not be empty", "path=build")
	})

	t.Run("whitespace-only command → raw error", func(t *testing.T) {
		yml := `
- name: build
  command: "   "
`
		requireBuildErr(t, yml, "phase=raw", "command must not be empty", "path=build")
	})

	t.Run("runnable with cwd: propagated to runtime model", func(t *testing.T) {
		yml := `
- name: build
  command: go build ./...
  cwd: ./backend
`
		root := requireBuildOK(t, yml)
		r := requireRunnable(t, root.Children[0], "build", "go build ./...")
		if r.Cwd != "./backend" {
			t.Errorf("expected Cwd './backend', got %q", r.Cwd)
		}
	})

	t.Run("runnable with env: propagated to runtime model", func(t *testing.T) {
		yml := `
- name: build
  command: go build ./...
  env:
    GOFLAGS: "-mod=vendor"
    CGO_ENABLED: "0"
`
		root := requireBuildOK(t, yml)
		r := requireRunnable(t, root.Children[0], "build", "go build ./...")
		if r.Env["GOFLAGS"] != "-mod=vendor" {
			t.Errorf("expected GOFLAGS='-mod=vendor', got %q", r.Env["GOFLAGS"])
		}
		if r.Env["CGO_ENABLED"] != "0" {
			t.Errorf("expected CGO_ENABLED='0', got %q", r.Env["CGO_ENABLED"])
		}
	})

	t.Run("runnable with cwd and env: both propagated", func(t *testing.T) {
		yml := `
- name: build
  command: go build ./...
  cwd: ./backend
  env:
    GOFLAGS: "-mod=vendor"
    FOO: bar
    BAR: baz
`
		root := requireBuildOK(t, yml)
		r := requireRunnable(t, root.Children[0], "build", "go build ./...")
		if r.Cwd != "./backend" {
			t.Errorf("Cwd: want './backend', got %q", r.Cwd)
		}
		if r.Env["GOFLAGS"] != "-mod=vendor" {
			t.Errorf("GOFLAGS: want '-mod=vendor', got %q", r.Env["GOFLAGS"])
		}
		if r.Env["FOO"] != "bar" {
			t.Errorf("FOO: want 'bar', got %q", r.Env["FOO"])
		}
		if r.Env["BAR"] != "baz" {
			t.Errorf("BAR: want 'baz', got %q", r.Env["BAR"])
		}
	})

	t.Run("runnable with no cwd: Cwd is empty string", func(t *testing.T) {
		yml := `
- name: build
  command: go build
`
		root := requireBuildOK(t, yml)
		r := requireRunnable(t, root.Children[0], "build", "go build")
		if r.Cwd != "" {
			t.Errorf("expected empty Cwd, got %q", r.Cwd)
		}
	})

	t.Run("runnable with no env: Env is nil or empty", func(t *testing.T) {
		yml := `
- name: build
  command: go build
`
		root := requireBuildOK(t, yml)
		r := requireRunnable(t, root.Children[0], "build", "go build")
		if len(r.Env) != 0 {
			t.Errorf("expected empty Env, got %v", r.Env)
		}
	})

	t.Run("command with special characters and spaces", func(t *testing.T) {
		yml := `
- name: run
  command: "docker run --rm -e FOO=bar -v /host:/container my-image:latest"
`
		root := requireBuildOK(t, yml)
		requireRunnable(t, root.Children[0], "run", "docker run --rm -e FOO=bar -v /host:/container my-image:latest")
	})

	// --- new command forms (§3.1) ---

	t.Run("array form: argv tokens used directly", func(t *testing.T) {
		yml := `
- name: up
  command: ["docker", "compose", "up", "-d"]
`
		root := requireBuildOK(t, yml)
		r := requireRunnable(t, root.Children[0], "up", "docker compose up -d")
		if len(r.Argv) != 4 {
			t.Errorf("expected 4 argv tokens, got %d: %v", len(r.Argv), r.Argv)
		}
	})

	t.Run("array form: equivalent to string form", func(t *testing.T) {
		ymlString := `
- name: build
  command: go build ./...
`
		ymlArray := `
- name: build
  command: ["go", "build", "./..."]
`
		r1 := requireBuildOK(t, ymlString)
		r2 := requireBuildOK(t, ymlArray)
		n1 := r1.Children[0].(*dsl.Runnable)
		n2 := r2.Children[0].(*dsl.Runnable)
		if strings.Join(n1.Argv, " ") != strings.Join(n2.Argv, " ") {
			t.Errorf("string and array forms differ: %v vs %v", n1.Argv, n2.Argv)
		}
	})

	t.Run("long form: command token + args merged into argv", func(t *testing.T) {
		yml := `
- name: up
  command: docker
  args:
    - compose
    - up
    - "-d"
`
		root := requireBuildOK(t, yml)
		r := requireRunnable(t, root.Children[0], "up", "docker compose up -d")
		if len(r.Argv) != 4 {
			t.Errorf("expected 4 argv tokens, got %d: %v", len(r.Argv), r.Argv)
		}
	})

	t.Run("long form: equivalent to string and array forms", func(t *testing.T) {
		ymlLong := `
- name: build
  command: go
  args: ["build", "./..."]
`
		root := requireBuildOK(t, ymlLong)
		requireRunnable(t, root.Children[0], "build", "go build ./...")
	})

	t.Run("array form + args: parse error", func(t *testing.T) {
		yml := `
- name: up
  command: ["docker", "compose"]
  args: ["up"]
`
		requireBuildErr(t, yml, "args is forbidden when command is an array")
	})

	t.Run("long form: multi-token command string → parse error", func(t *testing.T) {
		yml := `
- name: up
  command: docker compose
  args: ["up"]
`
		requireBuildErr(t, yml, "single token")
	})

	t.Run("array form: empty array → raw error", func(t *testing.T) {
		yml := `
- name: up
  command: []
`
		requireBuildErr(t, yml, "phase=raw", "command must not be empty")
	})
}

// ---------------------------------------------------------------------------
// §3.2 — Container node
// ---------------------------------------------------------------------------

func TestBuild_Section3_2_ContainerNode(t *testing.T) {
	t.Run("basic container: children accessible", func(t *testing.T) {
		yml := `
- name: backend
  children:
    - name: build
      command: go build
    - name: test
      command: go test ./...
`
		root := requireBuildOK(t, yml)
		c := requireContainer(t, root.Children[0], "backend", 2)
		requireRunnable(t, c.Children[0], "build", "go build")
		requireRunnable(t, c.Children[1], "test", "go test ./...")
	})

	t.Run("empty children list → raw error", func(t *testing.T) {
		yml := `
- name: backend
  children: []
`
		requireBuildErr(t, yml, "phase=raw", "at least one child", "path=backend")
	})

	t.Run("3-level nested containers", func(t *testing.T) {
		yml := `
- name: app
  children:
    - name: backend
      children:
        - name: api
          children:
            - name: start
              command: go run ./cmd/api
`
		root := requireBuildOK(t, yml)
		app := requireContainer(t, root.Children[0], "app", 1)
		backend := requireContainer(t, app.Children[0], "backend", 1)
		api := requireContainer(t, backend.Children[0], "api", 1)
		requireRunnable(t, api.Children[0], "start", "go run ./cmd/api")
	})

	t.Run("invalid child inside container: error path includes parent", func(t *testing.T) {
		yml := `
- name: backend
  children:
    - name: build
      command: go build
    - name: invalid
      command: ""
`
		requireBuildErr(t, yml, "phase=raw", "path=backend.invalid", "command must not be empty")
	})

	t.Run("deeply nested invalid node: full dot-separated path in error", func(t *testing.T) {
		yml := `
- name: app
  children:
    - name: backend
      children:
        - name: broken
          command: "   "
`
		requireBuildErr(t, yml, "phase=raw", "path=app.backend.broken", "command must not be empty")
	})

	t.Run("container mixed with runnable siblings at top level", func(t *testing.T) {
		yml := `
- name: app
  children:
    - name: build
      command: go build
    - name: test
      command: go test ./...
- name: frontend
  command: npm run dev
`
		root := requireBuildOK(t, yml)
		if len(root.Children) != 2 {
			t.Fatalf("expected 2 top-level nodes, got %d", len(root.Children))
		}
		requireContainer(t, root.Children[0], "app", 2)
		requireRunnable(t, root.Children[1], "frontend", "npm run dev")
	})
}

// ---------------------------------------------------------------------------
// §3.3 — Abstract node + `with`
// ---------------------------------------------------------------------------

func TestBuild_Section3_3_AbstractNode(t *testing.T) {
	t.Run("uses with single type: valid", func(t *testing.T) {
		yml := `
types:
  t:
    name: r
    command: echo ok
nodes:
  - name: stack
    uses:
      - t
`
		requireBuildOK(t, yml)
	})

	t.Run("uses with multiple types: valid", func(t *testing.T) {
		yml := `
types:
  a:
    name: na
    command: echo a
  b:
    name: nb
    command: echo b
nodes:
  - name: stack
    uses:
      - a
      - b
`
		requireBuildOK(t, yml)
	})

	t.Run("empty uses list → raw error", func(t *testing.T) {
		yml := `
- name: abstract
  uses: []
`
		requireBuildErr(t, yml, "phase=raw", "at least one entry", "path=abstract")
	})

	// --- uses shorthand (§3.3) ---

	t.Run("uses shorthand string: equivalent to single-element list", func(t *testing.T) {
		yml := `
types:
  t:
    name: r
    command: echo ok
nodes:
  - name: stack
    uses: t
`
		requireBuildOK(t, yml)
	})

	t.Run("uses shorthand string: expanded correctly", func(t *testing.T) {
		yml := `
types:
  docker-compose:
    params:
      file: ~
    children:
      - name: up
        command: docker compose -f {{ params.file }} up -d
      - name: down
        command: docker compose -f {{ params.file }} down
nodes:
  - name: stack
    uses: docker-compose
    with:
      file: docker-compose.yml
`
		root := requireBuildOK(t, yml)
		c := requireContainer(t, root.Children[0], "stack", 2)
		requireRunnable(t, c.Children[0], "up", "docker compose -f docker-compose.yml up -d")
		requireRunnable(t, c.Children[1], "down", "docker compose -f docker-compose.yml down")
	})

	t.Run("uses shorthand: string and list forms produce identical result", func(t *testing.T) {
		ymlString := `
types:
  t:
    name: gen
    command: echo hello
nodes:
  - name: stack
    uses: t
`
		ymlList := `
types:
  t:
    name: gen
    command: echo hello
nodes:
  - name: stack
    uses:
      - t
`
		r1 := requireBuildOK(t, ymlString)
		r2 := requireBuildOK(t, ymlList)
		if snapshotTree(r1) != snapshotTree(r2) {
			t.Errorf("string and list uses forms differ:\nstring: %s\nlist:   %s",
				snapshotTree(r1), snapshotTree(r2))
		}
	})

	t.Run("uses shorthand: empty string → parse error", func(t *testing.T) {
		yml := `
- name: abstract
  uses: ""
`
		requireBuildErr(t, yml, "uses")
	})

	t.Run("with mapping form: shared params passed to type", func(t *testing.T) {
		yml := `
types:
  docker-compose:
    params:
      file: ~
    children:
      - name: up
        command: docker compose -f {{ params.file }} up -d
      - name: stop
        command: docker compose -f {{ params.file }} stop
nodes:
  - name: stack
    uses:
      - docker-compose
    with:
      file: docker-compose.yml
`
		root := requireBuildOK(t, yml)
		c := requireContainer(t, root.Children[0], "stack", 2)
		requireRunnable(t, c.Children[0], "up", "docker compose -f docker-compose.yml up -d")
		requireRunnable(t, c.Children[1], "stop", "docker compose -f docker-compose.yml stop")
	})

	t.Run("with list form: per-type params", func(t *testing.T) {
		yml := `
types:
  svc-a:
    params:
      host: ~
    name: service-a
    command: start {{ params.host }}
  svc-b:
    params:
      port: ~
    name: service-b
    command: start :{{ params.port }}
nodes:
  - name: stack
    uses:
      - svc-a
      - svc-b
    with:
      - type: svc-a
        host: localhost
      - type: svc-b
        port: "8080"
`
		root := requireBuildOK(t, yml)
		c := requireContainer(t, root.Children[0], "stack", 2)
		requireRunnable(t, c.Children[0], "service-a", "start localhost")
		requireRunnable(t, c.Children[1], "service-b", "start :8080")
	})

	t.Run("with on non-abstract node (runnable) → raw error", func(t *testing.T) {
		// Construct RawNode directly to bypass YAML parsing (YAML would
		// normally not parse `with` on a runnable node, but we test the
		// raw validator's guard regardless).
		cmd := "go build"
		r := dsl.RawNode{
			Name:    "build",
			Command: &cmd,
			With:    &dsl.WithBlock{Shared: map[string]string{"file": "x"}},
		}
		reg := dsl.NewRegistry()
		_, err := dsl.NewEngine(reg).Build([]dsl.RawNode{r})
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "phase=raw") {
			t.Errorf("expected phase=raw in error, got: %v", err)
		}
		if !strings.Contains(err.Error(), "'with' can only be used") {
			t.Errorf("expected with-only-on-abstract message in error, got: %v", err)
		}
	})

	t.Run("with list form: entry missing 'type' → raw error", func(t *testing.T) {
		r := dsl.RawNode{
			Name: "stack",
			Uses: []string{"some-type"},
			With: &dsl.WithBlock{
				PerType: []dsl.TypedWith{{Type: ""}},
			},
		}
		reg := dsl.NewRegistry()
		_, err := dsl.NewEngine(reg).Build([]dsl.RawNode{r})
		if err == nil {
			t.Fatal("expected error for empty type in with list")
		}
		if !strings.Contains(err.Error(), "phase=raw") {
			t.Errorf("expected phase=raw in error, got: %v", err)
		}
	})

	t.Run("with list form: type references type not in uses → expand error", func(t *testing.T) {
		yml := `
types:
  known:
    name: k
    command: echo k
nodes:
  - name: stack
    uses:
      - known
    with:
      - type: unknown-type
        key: val
`
		requireBuildErr(t, yml, "phase=expand", "not in uses")
	})
}

// ---------------------------------------------------------------------------
// §4 — Type parameters
// ---------------------------------------------------------------------------

func TestBuild_Section4_TypeParameters(t *testing.T) {
	t.Run("required param provided: success", func(t *testing.T) {
		yml := `
types:
  greeter:
    params:
      name: ~
    command: echo hello {{ params.name }}
nodes:
  - name: greet
    uses:
      - greeter
    with:
      name: world
`
		root := requireBuildOK(t, yml)
		requireRunnable(t, root.Children[0], "greet", "echo hello world")
	})

	t.Run("required param missing → expand error with ErrMissingParam", func(t *testing.T) {
		yml := `
types:
  greeter:
    params:
      name: ~
    command: echo hello {{ params.name }}
nodes:
  - name: greet
    uses:
      - greeter
`
		err := requireBuildErr(t, yml, "phase=expand")
		if !errors.Is(err, dsl.ErrMissingParam) {
			t.Errorf("expected ErrMissingParam, got %v", err)
		}
	})

	t.Run("unknown param provided → expand error with ErrUnknownParam", func(t *testing.T) {
		yml := `
types:
  greeter:
    params:
      name: ~
    command: echo hello {{ params.name }}
nodes:
  - name: greet
    uses:
      - greeter
    with:
      name: world
      typo: oops
`
		err := requireBuildErr(t, yml, "phase=expand")
		if !errors.Is(err, dsl.ErrUnknownParam) {
			t.Errorf("expected ErrUnknownParam, got %v", err)
		}
	})

	t.Run("type with no params + no with: success", func(t *testing.T) {
		yml := `
types:
  simple:
    command: echo ok
nodes:
  - name: x
    uses:
      - simple
`
		requireBuildOK(t, yml)
	})

	t.Run("type with no params + with provided → unknown param error", func(t *testing.T) {
		yml := `
types:
  simple:
    command: echo ok
nodes:
  - name: x
    uses:
      - simple
    with:
      unexpected: value
`
		err := requireBuildErr(t, yml, "phase=expand")
		if !errors.Is(err, dsl.ErrUnknownParam) {
			t.Errorf("expected ErrUnknownParam, got %v", err)
		}
	})

	t.Run("optional param omitted: default value applied", func(t *testing.T) {
		yml := `
types:
  docker-compose:
    params:
      file: ~
      profile: dev
    command: docker compose -f {{ params.file }} --profile {{ params.profile }} up -d
nodes:
  - name: stack
    uses:
      - docker-compose
    with:
      file: docker-compose.yml
`
		root := requireBuildOK(t, yml)
		requireRunnable(t, root.Children[0], "stack",
			"docker compose -f docker-compose.yml --profile dev up -d")
	})

	t.Run("optional param overridden by caller", func(t *testing.T) {
		yml := `
types:
  docker-compose:
    params:
      file: ~
      profile: dev
    command: docker compose -f {{ params.file }} --profile {{ params.profile }} up -d
nodes:
  - name: stack
    uses:
      - docker-compose
    with:
      file: docker-compose.yml
      profile: production
`
		root := requireBuildOK(t, yml)
		requireRunnable(t, root.Children[0], "stack",
			"docker compose -f docker-compose.yml --profile production up -d")
	})

	t.Run("number default value normalised to string", func(t *testing.T) {
		yml := `
types:
  server:
    params:
      port: 8080
    command: ./server --port {{ params.port }}
nodes:
  - name: srv
    uses:
      - server
`
		root := requireBuildOK(t, yml)
		requireRunnable(t, root.Children[0], "srv", "./server --port 8080")
	})

	t.Run("number param passed via with normalised to string", func(t *testing.T) {
		yml := `
types:
  server:
    params:
      port: ~
    command: ./server --port {{ params.port }}
nodes:
  - name: srv
    uses:
      - server
    with:
      port: 3000
`
		root := requireBuildOK(t, yml)
		requireRunnable(t, root.Children[0], "srv", "./server --port 3000")
	})

	t.Run("multiple required params all provided: success", func(t *testing.T) {
		yml := `
types:
  docker-run:
    params:
      image: ~
      tag: ~
    command: docker run {{ params.image }}:{{ params.tag }}
nodes:
  - name: run
    uses:
      - docker-run
    with:
      image: myapp
      tag: v1.2.3
`
		root := requireBuildOK(t, yml)
		requireRunnable(t, root.Children[0], "run", "docker run myapp:v1.2.3")
	})

	t.Run("all params have defaults: no with required", func(t *testing.T) {
		yml := `
types:
  server:
    params:
      host: localhost
      port: 8080
    command: ./server {{ params.host }}:{{ params.port }}
nodes:
  - name: srv
    uses:
      - server
`
		root := requireBuildOK(t, yml)
		requireRunnable(t, root.Children[0], "srv", "./server localhost:8080")
	})
}

// ---------------------------------------------------------------------------
// §4.2 — Template syntax
// ---------------------------------------------------------------------------

func TestBuild_Section4_2_TemplateSyntax(t *testing.T) {
	t.Run("template in command: substituted correctly", func(t *testing.T) {
		yml := `
types:
  runner:
    params:
      script: ~
    command: bash {{ params.script }}
nodes:
  - name: x
    uses:
      - runner
    with:
      script: ./run.sh
`
		root := requireBuildOK(t, yml)
		requireRunnable(t, root.Children[0], "x", "bash ./run.sh")
	})

	t.Run("template in cwd: substituted correctly", func(t *testing.T) {
		yml := `
types:
  builder:
    params:
      dir: ~
    command: go build ./...
    cwd: "{{ params.dir }}"
nodes:
  - name: build
    uses:
      - builder
    with:
      dir: ./backend
`
		root := requireBuildOK(t, yml)
		r := requireRunnable(t, root.Children[0], "build", "go build ./...")
		if r.Cwd != "./backend" {
			t.Errorf("Cwd: want './backend', got %q", r.Cwd)
		}
	})

	t.Run("template in env value: substituted correctly", func(t *testing.T) {
		yml := `
types:
  builder:
    params:
      mod: vendor
    command: go build ./...
    env:
      GOFLAGS: "-mod={{ params.mod }}"
nodes:
  - name: build
    uses:
      - builder
    with:
      mod: readonly
`
		root := requireBuildOK(t, yml)
		r := requireRunnable(t, root.Children[0], "build", "go build ./...")
		if r.Env["GOFLAGS"] != "-mod=readonly" {
			t.Errorf("GOFLAGS: want '-mod=readonly', got %q", r.Env["GOFLAGS"])
		}
	})

	t.Run("template in child node name (dynamic name)", func(t *testing.T) {
		yml := `
types:
  service:
    params:
      svc: ~
    children:
      - name: "{{ params.svc }}-up"
        command: docker compose up {{ params.svc }}
      - name: "{{ params.svc }}-down"
        command: docker compose down {{ params.svc }}
nodes:
  - name: stack
    uses:
      - service
    with:
      svc: api
`
		root := requireBuildOK(t, yml)
		c := requireContainer(t, root.Children[0], "stack", 2)
		requireRunnable(t, c.Children[0], "api-up", "docker compose up api")
		requireRunnable(t, c.Children[1], "api-down", "docker compose down api")
	})

	t.Run("template in type root name used as child name in multi-type expansion", func(t *testing.T) {
		// Two uses of the same type with different per-type params.
		// After substitution each gets a distinct name.
		yml := `
types:
  svc:
    params:
      id: ~
    name: "svc-{{ params.id }}"
    command: start {{ params.id }}
nodes:
  - name: stack
    uses:
      - svc
      - svc
    with:
      - type: svc
        id: alpha
      - type: svc
        id: beta
`
		root := requireBuildOK(t, yml)
		c := requireContainer(t, root.Children[0], "stack", 2)
		requireRunnable(t, c.Children[0], "svc-alpha", "start alpha")
		requireRunnable(t, c.Children[1], "svc-beta", "start beta")
	})

	t.Run("duplicate names after template substitution → expand error", func(t *testing.T) {
		yml := `
types:
  svc:
    params:
      id: ~
    name: "{{ params.id }}"
    command: start {{ params.id }}
nodes:
  - name: stack
    uses:
      - svc
      - svc
    with:
      - type: svc
        id: same
      - type: svc
        id: same
`
		err := requireBuildErr(t, yml, "phase=expand")
		if !errors.Is(err, dsl.ErrDuplicateChild) {
			t.Errorf("expected ErrDuplicateChild, got %v", err)
		}
	})

	t.Run("template in nested with value flows into inner type", func(t *testing.T) {
		// Param names used in {{ params.name }} templates must be valid Go identifiers.
		// Hyphens are not allowed (they are the subtraction operator in templates).
		// Use underscores: "compose_file" not "compose-file".
		yml := `
types:
  docker-compose:
    params:
      file: ~
    command: docker compose -f {{ params.file }} up -d

  full-stack:
    params:
      compose_file: ~
    children:
      - name: docker
        uses:
          - docker-compose
        with:
          file: "{{ params.compose_file }}"

nodes:
  - name: prod
    uses:
      - full-stack
    with:
      compose_file: docker-compose.prod.yml
`
		root := requireBuildOK(t, yml)
		c := requireContainer(t, root.Children[0], "prod", 1)
		requireRunnable(t, c.Children[0], "docker",
			"docker compose -f docker-compose.prod.yml up -d")
	})

	t.Run("param name with hyphen → accepted", func(t *testing.T) {
		// Hyphens in param names are allowed after the first character.
		yml := `
types:
  svc:
    params:
      compose-file: ~
    command: echo hi
nodes:
  - name: s
    uses:
      - svc
    with:
      compose-file: val
`
		requireBuildOK(t, yml)
	})

	t.Run("template with no markers: string returned unchanged (fast path)", func(t *testing.T) {
		yml := `
types:
  simple:
    params:
      x: unused
    command: echo static
nodes:
  - name: n
    uses:
      - simple
    with:
      x: anything
`
		root := requireBuildOK(t, yml)
		requireRunnable(t, root.Children[0], "n", "echo static")
	})
}

// ---------------------------------------------------------------------------
// §5 — Type expansion
// ---------------------------------------------------------------------------

func TestBuild_Section5_TypeExpansion(t *testing.T) {
	t.Run("single use expands to runnable, abstract node name overrides type root name", func(t *testing.T) {
		yml := `
types:
  my-cmd:
    name: generated-name
    command: echo hello

nodes:
  - name: stack
    uses:
      - my-cmd
`
		root := requireBuildOK(t, yml)
		r := requireRunnable(t, root.Children[0], "stack", "echo hello")
		if r.Name() == "generated-name" {
			t.Errorf("type root name must NOT be used; abstract node name must take priority")
		}
	})

	t.Run("single use expands to container, abstract node name overrides type root name", func(t *testing.T) {
		yml := `
types:
  docker-compose:
    name: compose
    children:
      - name: up
        command: docker compose up -d
      - name: down
        command: docker compose down

nodes:
  - name: stack
    uses:
      - docker-compose
`
		root := requireBuildOK(t, yml)
		c := requireContainer(t, root.Children[0], "stack", 2)
		if c.Name() == "compose" {
			t.Errorf("type root name 'compose' must be overridden by abstract node name 'stack'")
		}
		requireRunnable(t, c.Children[0], "up", "docker compose up -d")
		requireRunnable(t, c.Children[1], "down", "docker compose down")
	})

	t.Run("multiple uses: children appended sequentially in declared order", func(t *testing.T) {
		yml := `
types:
  type-a:
    name: a
    command: echo a
  type-b:
    name: b
    command: echo b
  type-c:
    name: c
    command: echo c

nodes:
  - name: stack
    uses:
      - type-a
      - type-b
      - type-c
`
		root := requireBuildOK(t, yml)
		c := requireContainer(t, root.Children[0], "stack", 3)
		if c.Children[0].Name() != "a" {
			t.Errorf("first child: want 'a', got %q", c.Children[0].Name())
		}
		if c.Children[1].Name() != "b" {
			t.Errorf("second child: want 'b', got %q", c.Children[1].Name())
		}
		if c.Children[2].Name() != "c" {
			t.Errorf("third child: want 'c', got %q", c.Children[2].Name())
		}
	})

	t.Run("unknown type → expand error with phase and path", func(t *testing.T) {
		yml := `
- name: stack
  uses:
    - nonexistent-type
`
		err := requireBuildErr(t, yml, "phase=expand", "path=stack")
		if !errors.Is(err, dsl.ErrUnknownType) {
			t.Errorf("expected ErrUnknownType, got %v", err)
		}
	})

	t.Run("cycle: type references itself → expand error", func(t *testing.T) {
		yml := `
types:
  cyclic:
    name: gen
    uses:
      - cyclic

nodes:
  - name: stack
    uses:
      - cyclic
`
		err := requireBuildErr(t, yml, "phase=expand", "path=stack")
		if !errors.Is(err, dsl.ErrCycleDetected) {
			t.Errorf("expected ErrCycleDetected, got %v", err)
		}
	})

	t.Run("cycle: indirect A → B → A → expand error", func(t *testing.T) {
		yml := `
types:
  type-a:
    name: a
    uses:
      - type-b
  type-b:
    name: b
    uses:
      - type-a

nodes:
  - name: stack
    uses:
      - type-a
`
		err := requireBuildErr(t, yml, "phase=expand")
		if !errors.Is(err, dsl.ErrCycleDetected) {
			t.Errorf("expected ErrCycleDetected, got %v", err)
		}
	})

	t.Run("duplicate sibling names after multi-use expansion → expand error", func(t *testing.T) {
		yml := `
types:
  type-a:
    name: dup
    command: echo first
  type-b:
    name: dup
    command: echo second

nodes:
  - name: stack
    uses:
      - type-a
      - type-b
`
		err := requireBuildErr(t, yml, "phase=expand", "path=stack")
		if !errors.Is(err, dsl.ErrDuplicateChild) {
			t.Errorf("expected ErrDuplicateChild, got %v", err)
		}
	})

	t.Run("type produces runnable with empty command → runtime error", func(t *testing.T) {
		yml := `
types:
  bad-type:
    name: broken
    command: ""

nodes:
  - name: stack
    uses:
      - bad-type
`
		requireBuildErr(t, yml, "phase=runtime", "path=stack")
	})

	t.Run("expansion preserves cwd and env from type definition", func(t *testing.T) {
		yml := `
types:
  typed-build:
    name: build
    command: go build ./...
    cwd: ./backend
    env:
      GOFLAGS: "-mod=vendor"

nodes:
  - name: mybuild
    uses:
      - typed-build
`
		root := requireBuildOK(t, yml)
		r := requireRunnable(t, root.Children[0], "mybuild", "go build ./...")
		if r.Cwd != "./backend" {
			t.Errorf("Cwd: want './backend', got %q", r.Cwd)
		}
		if r.Env["GOFLAGS"] != "-mod=vendor" {
			t.Errorf("GOFLAGS: want '-mod=vendor', got %q", r.Env["GOFLAGS"])
		}
	})

	t.Run("nested uses: abstract node inside container referencing a type", func(t *testing.T) {
		yml := `
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
		root := requireBuildOK(t, yml)
		if len(root.Children) != 2 {
			t.Fatalf("expected 2 top-level nodes, got %d", len(root.Children))
		}
		app := requireContainer(t, root.Children[0], "app", 2)
		requireContainer(t, app.Children[0], "backend", 2)
		stack := requireContainer(t, app.Children[1], "stack", 2)
		requireRunnable(t, stack.Children[0], "up", "docker compose up -d")
		requireRunnable(t, stack.Children[1], "down", "docker compose down")
		requireRunnable(t, root.Children[1], "frontend", "npm run dev")
	})

	t.Run("expansion is recursive: type body contains abstract nodes", func(t *testing.T) {
		yml := `
types:
  inner:
    name: inner-node
    command: echo inner

  outer:
    name: outer-node
    children:
      - name: sub
        uses:
          - inner

nodes:
  - name: top
    uses:
      - outer
`
		root := requireBuildOK(t, yml)
		c := requireContainer(t, root.Children[0], "top", 1)
		requireRunnable(t, c.Children[0], "sub", "echo inner")
	})
}

// ---------------------------------------------------------------------------
// §5.2 — Name resolution for abstract nodes
// ---------------------------------------------------------------------------

func TestBuild_Section5_2_NameResolution(t *testing.T) {
	t.Run("single use: abstract node name takes priority over type root name", func(t *testing.T) {
		yml := `
types:
  my-type:
    name: type-root-name
    command: echo ok

nodes:
  - name: abstract-name
    uses:
      - my-type
`
		root := requireBuildOK(t, yml)
		if root.Children[0].Name() != "abstract-name" {
			t.Errorf("expected 'abstract-name', got %q", root.Children[0].Name())
		}
		if root.Children[0].Name() == "type-root-name" {
			t.Error("type root name must not be used when abstract node has a name")
		}
	})

	t.Run("multi-use: wrapping container takes abstract node name; children keep their type names", func(t *testing.T) {
		yml := `
types:
  svc-a:
    name: service-a
    command: start service-a
  svc-b:
    name: service-b
    command: start service-b

nodes:
  - name: services
    uses:
      - svc-a
      - svc-b
`
		root := requireBuildOK(t, yml)
		c := requireContainer(t, root.Children[0], "services", 2)
		if c.Children[0].Name() != "service-a" {
			t.Errorf("first child: want 'service-a', got %q", c.Children[0].Name())
		}
		if c.Children[1].Name() != "service-b" {
			t.Errorf("second child: want 'service-b', got %q", c.Children[1].Name())
		}
	})

	t.Run("single use: abstract node name overrides template-generated type root name", func(t *testing.T) {
		yml := `
types:
  docker-compose:
    params:
      env: ~
    name: "compose-{{ params.env }}"
    children:
      - name: up
        command: docker compose up -d
      - name: down
        command: docker compose down
nodes:
  - name: infra
    uses:
      - docker-compose
    with:
      env: prod
`
		// Single-use: abstract node name "infra" overrides the resolved type name "compose-prod".
		root := requireBuildOK(t, yml)
		c := requireContainer(t, root.Children[0], "infra", 2)
		if c.Name() == "compose-prod" {
			t.Error("abstract node name 'infra' must override template-resolved type root name")
		}
	})
}

// ---------------------------------------------------------------------------
// §6 — Name rules
// ---------------------------------------------------------------------------

func TestBuild_Section6_NameRules(t *testing.T) {
	t.Run("duplicate sibling names at top level → raw error", func(t *testing.T) {
		yml := `
- name: build
  command: go build
- name: build
  command: go test
`
		requireBuildErr(t, yml, "phase=raw", "duplicate sibling name")
	})

	t.Run("duplicate sibling names inside container → raw error with path", func(t *testing.T) {
		yml := `
- name: backend
  children:
    - name: build
      command: go build
    - name: build
      command: go test
`
		requireBuildErr(t, yml, "phase=raw", "duplicate sibling name", "backend")
	})

	t.Run("case-sensitive names: Build and build are distinct siblings", func(t *testing.T) {
		yml := `
- name: backend
  children:
    - name: Build
      command: go build
    - name: build
      command: go build ./...
`
		requireBuildOK(t, yml)
	})

	t.Run("case-sensitive at top level: App and app coexist", func(t *testing.T) {
		yml := `
- name: App
  command: echo App
- name: app
  command: echo app
`
		requireBuildOK(t, yml)
	})

	t.Run("duplicate names in deeply nested container → path includes full ancestry", func(t *testing.T) {
		yml := `
- name: app
  children:
    - name: backend
      children:
        - name: run
          command: go run .
        - name: run
          command: go run ./cmd
`
		requireBuildErr(t, yml, "phase=raw", "duplicate sibling name", "app.backend")
	})

	t.Run("name with spaces is allowed (spec does not restrict charset)", func(t *testing.T) {
		yml := `
- name: "my build"
  command: go build
`
		requireBuildOK(t, yml)
	})

	t.Run("names resolved after template substitution: two distinct resolved names succeed", func(t *testing.T) {
		yml := `
types:
  svc:
    params:
      id: ~
    name: "svc-{{ params.id }}"
    command: start {{ params.id }}
nodes:
  - name: stack
    uses:
      - svc
      - svc
    with:
      - type: svc
        id: one
      - type: svc
        id: two
`
		root := requireBuildOK(t, yml)
		c := requireContainer(t, root.Children[0], "stack", 2)
		if c.Children[0].Name() != "svc-one" {
			t.Errorf("first child: want 'svc-one', got %q", c.Children[0].Name())
		}
		if c.Children[1].Name() != "svc-two" {
			t.Errorf("second child: want 'svc-two', got %q", c.Children[1].Name())
		}
	})
}

// ---------------------------------------------------------------------------
// §7 / §9 — Validation phases and error messages
// ---------------------------------------------------------------------------

func TestBuild_Section7_ValidationPhases(t *testing.T) {
	t.Run("phase=raw error includes phase label, path, and reason", func(t *testing.T) {
		yml := `
- name: broken
  command: ""
`
		requireBuildErr(t, yml, "phase=raw", "path=broken", "command must not be empty")
	})

	t.Run("phase=expand error includes phase label and path", func(t *testing.T) {
		yml := `
- name: stack
  uses:
    - missing-type
`
		requireBuildErr(t, yml, "phase=expand", "path=stack")
	})

	t.Run("phase=expand: missing param includes param name", func(t *testing.T) {
		yml := `
types:
  t:
    params:
      x: ~
    command: echo {{ params.x }}
nodes:
  - name: n
    uses:
      - t
`
		requireBuildErr(t, yml, "phase=expand", "missing required param", "x")
	})

	t.Run("phase=expand: unknown param includes param name", func(t *testing.T) {
		yml := `
types:
  t:
    params:
      x: ~
    command: echo {{ params.x }}
nodes:
  - name: n
    uses:
      - t
    with:
      x: hello
      y: oops
`
		requireBuildErr(t, yml, "phase=expand", "unknown param", "y")
	})

	t.Run("phase=runtime error includes phase label and path", func(t *testing.T) {
		yml := `
types:
  broken:
    name: b
    command: ""

nodes:
  - name: x
    uses:
      - broken
`
		requireBuildErr(t, yml, "phase=runtime", "path=x")
	})

	t.Run("phase=parse: empty top-level list → error", func(t *testing.T) {
		_, err := BuildFromDocuments(Document{Nodes: nil})
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "missing or empty") {
			t.Errorf("expected 'missing or empty' in error, got: %v", err)
		}
	})

	t.Run("raw validation runs before expansion: raw error not masked by expand", func(t *testing.T) {
		yml := `
- name: node
  command: go build
  uses:
    - some-type
`
		requireBuildErr(t, yml, "phase=raw")
	})
}

// ---------------------------------------------------------------------------
// §10 — Determinism
// ---------------------------------------------------------------------------

func TestBuild_Section10_Determinism(t *testing.T) {
	t.Run("same YAML input produces identical runtime tree structure", func(t *testing.T) {
		yml := `
types:
  my-type:
    name: gen
    command: echo hello

nodes:
  - name: app
    children:
      - name: backend
        uses:
          - my-type
      - name: frontend
        command: npm run dev
`
		root1 := requireBuildOK(t, yml)
		root2 := requireBuildOK(t, yml)

		snap1 := snapshotTree(root1)
		snap2 := snapshotTree(root2)
		if snap1 != snap2 {
			t.Fatalf("non-deterministic output:\n--- run1 ---\n%s\n--- run2 ---\n%s", snap1, snap2)
		}
	})

	t.Run("determinism with params and templates", func(t *testing.T) {
		yml := `
types:
  docker-compose:
    params:
      file: ~
      profile: dev
    children:
      - name: up
        command: docker compose -f {{ params.file }} --profile {{ params.profile }} up -d
      - name: stop
        command: docker compose -f {{ params.file }} stop

nodes:
  - name: stack
    uses:
      - docker-compose
    with:
      file: docker-compose.yml
`
		root1 := requireBuildOK(t, yml)
		root2 := requireBuildOK(t, yml)

		snap1 := snapshotTree(root1)
		snap2 := snapshotTree(root2)
		if snap1 != snap2 {
			t.Fatalf("non-deterministic output:\n--- run1 ---\n%s\n--- run2 ---\n%s", snap1, snap2)
		}
	})

	t.Run("each build call returns a distinct root instance (no pointer reuse)", func(t *testing.T) {
		yml := `
- name: build
  command: go build
`
		root1 := requireBuildOK(t, yml)
		root2 := requireBuildOK(t, yml)
		if root1 == root2 {
			t.Error("expected distinct root instances")
		}
	})
}

// ---------------------------------------------------------------------------
// BuildMany — multi-document merging
// ---------------------------------------------------------------------------

func TestBuildMany_MultiDocumentMerging(t *testing.T) {
	t.Run("types from doc1, nodes from doc2 (shorthand): merged correctly", func(t *testing.T) {
		typesYML := `
types:
  my-cmd:
    name: r
    command: echo ok
`
		nodesYML := `
- name: stack
  uses:
    - my-cmd
`
		root, err := BuildMany([]byte(typesYML), []byte(nodesYML))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		requireRunnable(t, root.Children[0], "stack", "echo ok")
	})

	t.Run("nodes from multiple docs are concatenated in order", func(t *testing.T) {
		doc1 := `
- name: first
  command: echo first
`
		doc2 := `
- name: second
  command: echo second
`
		doc3 := `
- name: third
  command: echo third
`
		root, err := BuildMany([]byte(doc1), []byte(doc2), []byte(doc3))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(root.Children) != 3 {
			t.Fatalf("expected 3 children, got %d", len(root.Children))
		}
		if root.Children[0].Name() != "first" {
			t.Errorf("1st child: want 'first', got %q", root.Children[0].Name())
		}
		if root.Children[1].Name() != "second" {
			t.Errorf("2nd child: want 'second', got %q", root.Children[1].Name())
		}
		if root.Children[2].Name() != "third" {
			t.Errorf("3rd child: want 'third', got %q", root.Children[2].Name())
		}
	})

	t.Run("types merged from all documents", func(t *testing.T) {
		doc1 := `
types:
  type-a:
    name: a
    command: echo a
`
		doc2 := `
types:
  type-b:
    name: b
    command: echo b
nodes:
  - name: combo
    uses:
      - type-a
      - type-b
`
		root, err := BuildMany([]byte(doc1), []byte(doc2))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		requireContainer(t, root.Children[0], "combo", 2)
	})

	t.Run("duplicate type name across documents → error", func(t *testing.T) {
		doc1 := `
types:
  conflicting:
    name: a
    command: echo a
`
		doc2 := `
types:
  conflicting:
    name: b
    command: echo b
nodes:
  - name: x
    command: echo x
`
		_, err := BuildMany([]byte(doc1), []byte(doc2))
		if err == nil {
			t.Fatal("expected error for duplicate type name across documents")
		}
		if !errors.Is(err, dsl.ErrTypeAlreadyExists) {
			t.Errorf("expected ErrTypeAlreadyExists, got %v", err)
		}
	})

	t.Run("no document has any nodes → error", func(t *testing.T) {
		doc1 := `
types:
  t:
    name: r
    command: echo ok
`
		doc2 := `
types:
  t2:
    name: r2
    command: echo ok2
`
		_, err := BuildMany([]byte(doc1), []byte(doc2))
		if err == nil {
			t.Fatal("expected error when no nodes are provided across all docs")
		}
		if !strings.Contains(err.Error(), "missing or empty") {
			t.Errorf("expected 'missing or empty' in error, got: %v", err)
		}
	})

	t.Run("single document in BuildMany behaves like Build", func(t *testing.T) {
		yml := `
- name: build
  command: go build
`
		rootMany, err := BuildMany([]byte(yml))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		rootBuild := requireBuildOK(t, yml)

		if snapshotTree(rootMany) != snapshotTree(rootBuild) {
			t.Error("BuildMany with single doc must produce same result as Build")
		}
	})
}

// ---------------------------------------------------------------------------
// NewRegistryFromDocuments
// ---------------------------------------------------------------------------

func TestNewRegistryFromDocuments(t *testing.T) {
	t.Run("empty docs: empty registry, no error", func(t *testing.T) {
		reg, err := NewRegistryFromDocuments()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if reg == nil {
			t.Fatal("expected non-nil registry")
		}
	})

	t.Run("single doc: all types registered", func(t *testing.T) {
		yml := `
types:
  type-a:
    name: a
    command: echo a
  type-b:
    name: b
    command: echo b
nodes:
  - name: x
    command: echo x
`
		doc := requireParseOK(t, yml)
		reg, err := NewRegistryFromDocuments(doc)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := reg.Get("type-a"); !ok {
			t.Error("expected type-a to be registered")
		}
		if _, ok := reg.Get("type-b"); !ok {
			t.Error("expected type-b to be registered")
		}
	})

	t.Run("duplicate type across docs → error with phase=parse", func(t *testing.T) {
		yml1 := `
types:
  shared:
    name: x
    command: echo x
nodes:
  - name: a
    command: echo a
`
		yml2 := `
types:
  shared:
    name: y
    command: echo y
nodes:
  - name: b
    command: echo b
`
		doc1 := requireParseOK(t, yml1)
		doc2 := requireParseOK(t, yml2)
		_, err := NewRegistryFromDocuments(doc1, doc2)
		if err == nil {
			t.Fatal("expected error for duplicate type across documents")
		}
		if !strings.Contains(err.Error(), "phase=parse") {
			t.Errorf("expected 'phase=parse' in error, got: %v", err)
		}
		if !errors.Is(err, dsl.ErrTypeAlreadyExists) {
			t.Errorf("expected ErrTypeAlreadyExists, got %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// §11 — Full spec examples end-to-end
// ---------------------------------------------------------------------------

func TestBuild_SpecExample_ParameterisedDockerCompose(t *testing.T) {
	// Reproduces the parameterised docker-compose example from §11 of the spec.
	yml := `
types:
  docker-compose:
    params:
      file: ~
      profile: dev
    children:
      - name: lifecycle
        children:
          - name: up
            command: docker compose -f {{ params.file }} --profile {{ params.profile }} up -d
          - name: stop
            command: docker compose -f {{ params.file }} stop

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
        with:
          file: docker-compose.yml
  - name: frontend
    command: npm run dev
`
	root := requireBuildOK(t, yml)

	if len(root.Children) != 2 {
		t.Fatalf("expected 2 top-level children, got %d", len(root.Children))
	}

	app := requireContainer(t, root.Children[0], "app", 2)
	backend := requireContainer(t, app.Children[0], "backend", 2)
	requireRunnable(t, backend.Children[0], "build", "go build ./...")
	requireRunnable(t, backend.Children[1], "test", "go test ./...")

	// stack must use abstract node name, not type root name
	stack := requireContainer(t, app.Children[1], "stack", 1)
	lifecycle := requireContainer(t, stack.Children[0], "lifecycle", 2)
	requireRunnable(t, lifecycle.Children[0], "up",
		"docker compose -f docker-compose.yml --profile dev up -d")
	requireRunnable(t, lifecycle.Children[1], "stop",
		"docker compose -f docker-compose.yml stop")

	requireRunnable(t, root.Children[1], "frontend", "npm run dev")
}

func TestBuild_SpecExample_NestedParameterisedTypes(t *testing.T) {
	// Reproduces the full-stack nested parameterised types example from §11.
	// Note: param names support hyphens after the first character.
	yml := `
types:
  docker-compose:
    params:
      file: ~
    command: docker compose -f {{ params.file }} up -d

  full-stack:
    params:
      compose_file: ~
      k8s_namespace: staging
    children:
      - name: docker
        uses:
          - docker-compose
        with:
          file: "{{ params.compose_file }}"
      - name: k8s
        command: kubectl apply -n {{ params.k8s_namespace }}

nodes:
  - name: prod
    uses:
      - full-stack
    with:
      compose_file: docker-compose.prod.yml
      k8s_namespace: production
`
	root := requireBuildOK(t, yml)
	c := requireContainer(t, root.Children[0], "prod", 2)
	requireRunnable(t, c.Children[0], "docker",
		"docker compose -f docker-compose.prod.yml up -d")
	requireRunnable(t, c.Children[1], "k8s",
		"kubectl apply -n production")
}

// ---------------------------------------------------------------------------
// Implicit root container (§2)
// ---------------------------------------------------------------------------

func TestBuild_ImplicitRootContainer(t *testing.T) {
	t.Run("top-level list is wrapped in implicit root container", func(t *testing.T) {
		yml := `
- name: build
  command: go build
`
		root := requireBuildOK(t, yml)
		if root.NodeName != "root" {
			t.Errorf("expected implicit root name 'root', got %q", root.NodeName)
		}
		if len(root.Children) != 1 {
			t.Fatalf("expected 1 child under root, got %d", len(root.Children))
		}
		if root.Children[0].Name() != "build" {
			t.Errorf("expected child name 'build', got %q", root.Children[0].Name())
		}
	})

	t.Run("multiple top-level nodes all become direct children of implicit root", func(t *testing.T) {
		yml := `
- name: a
  command: echo a
- name: b
  command: echo b
- name: c
  command: echo c
`
		root := requireBuildOK(t, yml)
		if root.NodeName != "root" {
			t.Errorf("expected root name 'root', got %q", root.NodeName)
		}
		if len(root.Children) != 3 {
			t.Errorf("expected 3 children, got %d", len(root.Children))
		}
	})
}

// ---------------------------------------------------------------------------
// Model helpers — AsRunnable, AsContainer, Container.Find
// ---------------------------------------------------------------------------

func TestModel_AsRunnable(t *testing.T) {
	yml := `
- name: backend
  children:
    - name: build
      command: go build ./...
`
	root := requireBuildOK(t, yml)
	backend := root.Children[0]
	build := root.Children[0].(*dsl.Container).Children[0]

	t.Run("AsRunnable on a Runnable returns the node and true", func(t *testing.T) {
		r, ok := dsl.AsRunnable(build)
		if !ok {
			t.Fatal("expected ok=true")
		}
		if strings.Join(r.Argv, " ") != "go build ./..." {
			t.Errorf("unexpected argv: %v", r.Argv)
		}
	})

	t.Run("AsRunnable on a Container returns nil and false", func(t *testing.T) {
		r, ok := dsl.AsRunnable(backend)
		if ok {
			t.Fatal("expected ok=false")
		}
		if r != nil {
			t.Fatal("expected nil")
		}
	})
}

func TestModel_AsContainer(t *testing.T) {
	yml := `
- name: backend
  children:
    - name: build
      command: go build ./...
`
	root := requireBuildOK(t, yml)
	backend := root.Children[0]
	build := root.Children[0].(*dsl.Container).Children[0]

	t.Run("AsContainer on a Container returns the node and true", func(t *testing.T) {
		c, ok := dsl.AsContainer(backend)
		if !ok {
			t.Fatal("expected ok=true")
		}
		if c.Name() != "backend" {
			t.Errorf("unexpected name: %q", c.Name())
		}
	})

	t.Run("AsContainer on a Runnable returns nil and false", func(t *testing.T) {
		c, ok := dsl.AsContainer(build)
		if ok {
			t.Fatal("expected ok=false")
		}
		if c != nil {
			t.Fatal("expected nil")
		}
	})
}

func TestModel_ContainerFind(t *testing.T) {
	yml := `
- name: backend
  children:
    - name: build
      command: go build ./...
    - name: test
      command: go test ./...
`
	root := requireBuildOK(t, yml)
	backend, _ := dsl.AsContainer(root.Children[0])

	t.Run("Find returns the matching child and true", func(t *testing.T) {
		node, ok := backend.Find("build")
		if !ok {
			t.Fatal("expected ok=true")
		}
		if node.Name() != "build" {
			t.Errorf("unexpected name: %q", node.Name())
		}
	})

	t.Run("Find returns false for an unknown name", func(t *testing.T) {
		_, ok := backend.Find("nonexistent")
		if ok {
			t.Fatal("expected ok=false")
		}
	})

	t.Run("Find is case-sensitive", func(t *testing.T) {
		_, ok := backend.Find("Build")
		if ok {
			t.Fatal("expected ok=false: Find must be case-sensitive")
		}
	})

	t.Run("Find only searches direct children, not descendants", func(t *testing.T) {
		yml2 := `
- name: app
  children:
    - name: backend
      children:
        - name: build
          command: go build
`
		root2 := requireBuildOK(t, yml2)
		app, _ := dsl.AsContainer(root2.Children[0])

		// "build" is a grandchild of app, not a direct child.
		_, ok := app.Find("build")
		if ok {
			t.Fatal("Find must not search recursively")
		}
	})
}

// ---------------------------------------------------------------------------
// §3.4 — Pipeline Node
// ---------------------------------------------------------------------------

func TestBuild_Section3_4_PipelineNode(t *testing.T) {

	// --- §3.4: basic structure ---

	t.Run("sequential steps: pipeline node builds correctly", func(t *testing.T) {
		yml := `
- name: deploy
  steps:
    - command: docker compose pull
    - command: docker compose up -d
`
		root := requireBuildOK(t, yml)
		p := requirePipeline(t, root.Children[0], "deploy", 2)
		if strings.Join(p.Steps[0].Argv, " ") != "docker compose pull" {
			t.Errorf("step[0] argv: want %q, got %q", "docker compose pull", strings.Join(p.Steps[0].Argv, " "))
		}
		if strings.Join(p.Steps[1].Argv, " ") != "docker compose up -d" {
			t.Errorf("step[1] argv: want %q, got %q", "docker compose up -d", strings.Join(p.Steps[1].Argv, " "))
		}
	})

	t.Run("pipeline node is selectable (AsPipeline works)", func(t *testing.T) {
		yml := `
- name: deploy
  steps:
    - command: echo hello
`
		root := requireBuildOK(t, yml)
		p, ok := dsl.AsPipeline(root.Children[0])
		if !ok {
			t.Fatal("expected AsPipeline to succeed")
		}
		if p.Name() != "deploy" {
			t.Errorf("name: want 'deploy', got %q", p.Name())
		}
	})

	t.Run("pipeline coexists with runnables and containers in the same tree", func(t *testing.T) {
		yml := `
- name: build
  command: go build ./...
- name: deploy
  steps:
    - command: docker compose pull
    - command: docker compose up -d
- name: group
  children:
    - name: test
      command: go test ./...
`
		root := requireBuildOK(t, yml)
		if len(root.Children) != 3 {
			t.Fatalf("expected 3 children, got %d", len(root.Children))
		}
		requireRunnable(t, root.Children[0], "build", "go build ./...")
		requirePipeline(t, root.Children[1], "deploy", 2)
		requireContainer(t, root.Children[2], "group", 1)
	})

	t.Run("XOR rule: steps + command → raw error", func(t *testing.T) {
		yml := `
- name: bad
  command: echo hi
  steps:
    - command: echo world
`
		requireBuildErr(t, yml, "phase=raw", "cannot combine")
	})

	t.Run("XOR rule: steps + children → raw error", func(t *testing.T) {
		yml := `
- name: bad
  children:
    - name: child
      command: echo hi
  steps:
    - command: echo world
`
		requireBuildErr(t, yml, "phase=raw", "cannot combine")
	})

	t.Run("empty steps list → raw error", func(t *testing.T) {
		yml := `
- name: bad
  steps: []
`
		requireBuildErr(t, yml, "phase=raw", "at least one step")
	})

	t.Run("step with empty command string → raw error", func(t *testing.T) {
		yml := `
- name: bad
  steps:
    - command: ""
`
		requireBuildErr(t, yml, "phase=raw", "step command must not be empty")
	})

	// --- §3.4.1: step structure ---

	t.Run("step long form (single token command + args): argv assembled correctly", func(t *testing.T) {
		yml := `
- name: login
  steps:
    - command: docker
      args:
        - login
        - registry.example.com
`
		root := requireBuildOK(t, yml)
		p := requirePipeline(t, root.Children[0], "login", 1)
		want := "docker login registry.example.com"
		if got := strings.Join(p.Steps[0].Argv, " "); got != want {
			t.Errorf("argv: want %q, got %q", want, got)
		}
	})

	t.Run("step with cwd and env: preserved in runtime Pipeline", func(t *testing.T) {
		yml := `
- name: build
  steps:
    - command: make build
      cwd: ./frontend
      env:
        NODE_ENV: production
`
		root := requireBuildOK(t, yml)
		p := requirePipeline(t, root.Children[0], "build", 1)
		if p.Steps[0].Cwd != "./frontend" {
			t.Errorf("Cwd: want './frontend', got %q", p.Steps[0].Cwd)
		}
		if p.Steps[0].Env["NODE_ENV"] != "production" {
			t.Errorf("NODE_ENV: want 'production', got %q", p.Steps[0].Env["NODE_ENV"])
		}
	})

	t.Run("capture stdout: preserved in PipelineStep", func(t *testing.T) {
		yml := `
- name: get-token
  steps:
    - id: auth
      command: get-token
      capture: stdout
    - command: echo done
`
		root := requireBuildOK(t, yml)
		p := requirePipeline(t, root.Children[0], "get-token", 2)
		if p.Steps[0].Capture != dsl.CaptureStdout {
			t.Errorf("Capture: want CaptureStdout, got %v", p.Steps[0].Capture)
		}
		if p.Steps[0].ID != "auth" {
			t.Errorf("ID: want 'auth', got %q", p.Steps[0].ID)
		}
	})

	t.Run("capture: requires id → raw error", func(t *testing.T) {
		yml := `
- name: bad
  steps:
    - command: echo hi
      capture: stdout
`
		requireBuildErr(t, yml, "phase=raw", "capture requires id")
	})

	t.Run("tee: requires capture → raw error", func(t *testing.T) {
		yml := `
- name: bad
  steps:
    - id: s
      command: echo hi
      tee: true
`
		requireBuildErr(t, yml, "phase=raw", "tee requires capture")
	})

	t.Run("tee: valid when capture is set", func(t *testing.T) {
		yml := `
- name: ok
  steps:
    - id: s
      command: echo hi
      capture: stdout
      tee: true
    - command: echo done
`
		root := requireBuildOK(t, yml)
		p := requirePipeline(t, root.Children[0], "ok", 2)
		if !p.Steps[0].Tee {
			t.Error("Tee: want true")
		}
	})

	t.Run("stdin: wires captured output to next step", func(t *testing.T) {
		yml := `
- name: find-go-files
  steps:
    - id: list
      command: ls
      capture: stdout
    - command: grep
      args: [".go"]
      stdin: steps.list.stdout
`
		root := requireBuildOK(t, yml)
		p := requirePipeline(t, root.Children[0], "find-go-files", 2)
		if p.Steps[1].Stdin == nil {
			t.Fatal("expected Stdin to be set")
		}
		if p.Steps[1].Stdin.StepID != "list" {
			t.Errorf("Stdin.StepID: want 'list', got %q", p.Steps[1].Stdin.StepID)
		}
		if p.Steps[1].Stdin.Stream != dsl.CaptureStdout {
			t.Errorf("Stdin.Stream: want CaptureStdout, got %v", p.Steps[1].Stdin.Stream)
		}
	})

	t.Run("stdin: forward reference → raw error", func(t *testing.T) {
		yml := `
- name: bad
  steps:
    - command: grep
      args: [".go"]
      stdin: steps.list.stdout
    - id: list
      command: ls
      capture: stdout
`
		requireBuildErr(t, yml, "phase=raw", "step reference to unknown id")
	})

	t.Run("stdin: references uncaptured stream → raw error", func(t *testing.T) {
		yml := `
- name: bad
  steps:
    - id: lister
      command: ls
      capture: stderr
    - command: grep
      args: [".go"]
      stdin: steps.lister.stdout
`
		requireBuildErr(t, yml, "phase=raw", "uncaptured stream")
	})

	t.Run("stdin: bad format → raw error", func(t *testing.T) {
		yml := `
- name: bad
  steps:
    - id: first
      command: ls
      capture: stdout
    - command: grep
      stdin: not.a.valid.ref
`
		requireBuildErr(t, yml, "phase=raw", "invalid step reference format")
	})

	t.Run("duplicate step ids → raw error", func(t *testing.T) {
		yml := `
- name: bad
  steps:
    - id: dup
      command: echo a
      capture: stdout
    - id: dup
      command: echo b
      capture: stdout
`
		requireBuildErr(t, yml, "phase=raw")
	})

	// --- §3.4.2: on-fail ---

	t.Run("on-fail: continue — preserved in PipelineStep", func(t *testing.T) {
		yml := `
- name: restart
  steps:
    - command: docker compose down
      on-fail: continue
    - command: docker compose up -d
`
		root := requireBuildOK(t, yml)
		p := requirePipeline(t, root.Children[0], "restart", 2)
		if p.Steps[0].OnFail.Action != "continue" {
			t.Errorf("OnFail.Action: want 'continue', got %q", p.Steps[0].OnFail.Action)
		}
		// Second step has no on-fail → zero value (empty action = fail-fast).
		if p.Steps[1].OnFail.Action != "" {
			t.Errorf("OnFail.Action[1]: want '' (default), got %q", p.Steps[1].OnFail.Action)
		}
	})

	t.Run("on-fail: retry with attempts and delay — preserved in PipelineStep", func(t *testing.T) {
		yml := `
- name: wait-for-api
  steps:
    - command: curl
      args: [--fail, --silent, https://api.example.com/health]
      on-fail:
        action: retry
        attempts: 3
        delay: 2s
    - command: run-migrations
`
		root := requireBuildOK(t, yml)
		p := requirePipeline(t, root.Children[0], "wait-for-api", 2)
		of := p.Steps[0].OnFail
		if of.Action != "retry" {
			t.Errorf("Action: want 'retry', got %q", of.Action)
		}
		if of.Attempts != 3 {
			t.Errorf("Attempts: want 3, got %d", of.Attempts)
		}
		if of.Delay != "2s" {
			t.Errorf("Delay: want '2s', got %q", of.Delay)
		}
	})

	t.Run("on-fail: retry with attempts < 2 → raw error", func(t *testing.T) {
		yml := `
- name: bad
  steps:
    - command: echo hi
      on-fail:
        action: retry
        attempts: 1
`
		requireBuildErr(t, yml, "phase=raw", "attempts must be >= 2")
	})

	t.Run("on-fail: invalid action string → raw error", func(t *testing.T) {
		yml := `
- name: bad
  steps:
    - command: echo hi
      on-fail: maybe
`
		requireBuildErr(t, yml, "phase=raw", "action must be")
	})

	t.Run("on-fail: invalid delay duration → raw error", func(t *testing.T) {
		yml := `
- name: bad
  steps:
    - command: echo hi
      on-fail:
        action: retry
        attempts: 3
        delay: notaduration
`
		requireBuildErr(t, yml, "phase=raw", "delay", "not a valid duration")
	})
}

// ---------------------------------------------------------------------------
// §3.5 — Step Output Substitution
// ---------------------------------------------------------------------------

func TestBuild_Section3_5_StepOutputSubstitution(t *testing.T) {

	t.Run("step ref in args: preserved as literal in runtime argv", func(t *testing.T) {
		// The ref is not resolved at build time — it remains in argv as a literal
		// to be resolved by the executor at run time.
		yml := `
- name: login
  steps:
    - id: bw
      command: bw get password my-registry
      capture: stdout
    - command: docker
      args:
        - login
        - --password
        - "{{ steps.bw.stdout }}"
`
		root := requireBuildOK(t, yml)
		p := requirePipeline(t, root.Children[0], "login", 2)
		lastArg := p.Steps[1].Argv[len(p.Steps[1].Argv)-1]
		if lastArg != "{{ steps.bw.stdout }}" {
			t.Errorf("step ref in argv: want %q, got %q", "{{ steps.bw.stdout }}", lastArg)
		}
	})

	t.Run("step ref in env value: preserved as literal", func(t *testing.T) {
		yml := `
- name: build
  steps:
    - id: version
      command: git describe --tags
      capture: stdout
    - command: ["docker", "build", "."]
      env:
        TAG: "{{ steps.version.stdout }}"
`
		root := requireBuildOK(t, yml)
		p := requirePipeline(t, root.Children[0], "build", 2)
		if p.Steps[1].Env["TAG"] != "{{ steps.version.stdout }}" {
			t.Errorf("env TAG: want ref literal, got %q", p.Steps[1].Env["TAG"])
		}
	})

	t.Run("step ref in cwd: preserved as literal", func(t *testing.T) {
		yml := `
- name: work
  steps:
    - id: find-root
      command: find-project-root
      capture: stdout
    - command: make
      args: [build]
      cwd: "{{ steps.find-root.stdout }}"
`
		root := requireBuildOK(t, yml)
		p := requirePipeline(t, root.Children[0], "work", 2)
		if p.Steps[1].Cwd != "{{ steps.find-root.stdout }}" {
			t.Errorf("cwd: want ref literal, got %q", p.Steps[1].Cwd)
		}
	})

	t.Run("step ref: forward reference → raw error", func(t *testing.T) {
		yml := `
- name: bad
  steps:
    - command: docker
      args:
        - login
        - --password
        - "{{ steps.bw.stdout }}"
    - id: bw
      command: bw get password
      capture: stdout
`
		requireBuildErr(t, yml, "phase=raw", "step reference to unknown id")
	})

	t.Run("step ref: referenced step has no capture → raw error", func(t *testing.T) {
		yml := `
- name: bad
  steps:
    - id: lister
      command: ls
    - command: grep
      args:
        - "{{ steps.lister.stdout }}"
`
		requireBuildErr(t, yml, "phase=raw", "uncaptured stream")
	})

	t.Run("step ref: unknown step id in args → raw error", func(t *testing.T) {
		yml := `
- name: bad
  steps:
    - command: echo
      args:
        - "{{ steps.nobody.stdout }}"
`
		requireBuildErr(t, yml, "phase=raw", "step reference to unknown id")
	})

	// §3.5: step refs inside a type body are preserved through Phase 2 expansion.
	t.Run("step ref inside type body: preserved as literal after type expansion", func(t *testing.T) {
		yml := `
types:
  docker-login:
    params:
      registry: ~
      username: ~
    steps:
      - id: token
        command: get-token
        args:
          - "{{ params.registry }}"
        capture: stdout
      - command: docker
        args:
          - login
          - "{{ params.registry }}"
          - --username
          - "{{ params.username }}"
          - --password
          - "{{ steps.token.stdout }}"

nodes:
  - name: login
    uses: docker-login
    with:
      registry: registry.example.com
      username: ci-bot
`
		root := requireBuildOK(t, yml)
		p := requirePipeline(t, root.Children[0], "login", 2)

		// Step 0: type param {{ params.registry }} must be substituted.
		if p.Steps[0].Argv[0] != "get-token" {
			t.Errorf("step[0] cmd: want 'get-token', got %q", p.Steps[0].Argv[0])
		}
		if p.Steps[0].Argv[1] != "registry.example.com" {
			t.Errorf("step[0] registry arg: want 'registry.example.com', got %q", p.Steps[0].Argv[1])
		}

		// Step 1: type param {{ params.registry }} and {{ params.username }} must be substituted;
		// step ref {{ steps.token.stdout }} must be preserved as a literal.
		// Argv = ["docker", "login", registry, "--username", username, "--password", step-ref]
		step1 := p.Steps[1]
		registryArg := step1.Argv[2] // docker login <registry> ...
		if registryArg != "registry.example.com" {
			t.Errorf("step[1] registry arg: want 'registry.example.com', got %q", registryArg)
		}
		usernameArg := step1.Argv[4] // --username <username>
		if usernameArg != "ci-bot" {
			t.Errorf("step[1] username arg: want 'ci-bot', got %q", usernameArg)
		}
		passwordArg := step1.Argv[6] // --password <step-ref>
		if passwordArg != "{{ steps.token.stdout }}" {
			t.Errorf("step[1] password arg: want step ref literal, got %q", passwordArg)
		}
	})

	t.Run("capture: both — CaptureMode set correctly", func(t *testing.T) {
		yml := `
- name: noisy
  steps:
    - id: cmd
      command: build-tool
      capture: both
    - command: echo done
`
		root := requireBuildOK(t, yml)
		p := requirePipeline(t, root.Children[0], "noisy", 2)
		if p.Steps[0].Capture != dsl.CaptureBoth {
			t.Errorf("Capture: want CaptureBoth, got %v", p.Steps[0].Capture)
		}
	})

	t.Run("stdin referencing stderr: wired correctly", func(t *testing.T) {
		yml := `
- name: debug
  steps:
    - id: run
      command: noisy-cmd
      capture: stderr
    - command: grep
      args: [ERROR]
      stdin: steps.run.stderr
`
		root := requireBuildOK(t, yml)
		p := requirePipeline(t, root.Children[0], "debug", 2)
		if p.Steps[1].Stdin == nil {
			t.Fatal("expected Stdin to be set")
		}
		if p.Steps[1].Stdin.Stream != dsl.CaptureStderr {
			t.Errorf("Stdin.Stream: want CaptureStderr, got %v", p.Steps[1].Stdin.Stream)
		}
	})
}

// ---------------------------------------------------------------------------
// §5 — Runtime inputs
// ---------------------------------------------------------------------------

func TestBuild_Section5_RuntimeInputs(t *testing.T) {

	t.Run("required input on runnable: propagated to Runnable.Inputs", func(t *testing.T) {
		yml := `
- name: deploy
  inputs:
    env: ~
  command: ./deploy.sh {{ inputs.env }}
`
		root := requireBuildOK(t, yml)
		r := requireRunnable(t, root.Children[0], "deploy", "./deploy.sh {{ inputs.env }}")
		if r.Inputs == nil {
			t.Fatal("expected Inputs to be set")
		}
		if _, ok := r.Inputs["env"]; !ok {
			t.Error("expected input 'env' to be declared")
		}
		if r.Inputs["env"] != nil {
			t.Error("expected input 'env' to be required (nil default)")
		}
	})

	t.Run("optional input on runnable: default value propagated", func(t *testing.T) {
		yml := `
- name: greet
  inputs:
    name: world
  command: echo hello {{ inputs.name }}
`
		root := requireBuildOK(t, yml)
		r := requireRunnable(t, root.Children[0], "greet", "echo hello {{ inputs.name }}")
		if r.Inputs == nil {
			t.Fatal("expected Inputs to be set")
		}
		def := r.Inputs["name"]
		if def == nil {
			t.Fatal("expected input 'name' to have a default")
		}
		if *def != "world" {
			t.Errorf("input 'name' default: want %q, got %q", "world", *def)
		}
	})

	t.Run("inputs on pipeline: propagated to Pipeline.Inputs", func(t *testing.T) {
		yml := `
- name: build
  inputs:
    target: ~
  steps:
    - command: go build {{ inputs.target }}
`
		root := requireBuildOK(t, yml)
		p := requirePipeline(t, root.Children[0], "build", 1)
		if p.Inputs == nil {
			t.Fatal("expected Pipeline.Inputs to be set")
		}
		if _, ok := p.Inputs["target"]; !ok {
			t.Error("expected input 'target' to be declared")
		}
	})

	t.Run("inputs in type definition: propagated to concrete node", func(t *testing.T) {
		yml := `
types:
  deployer:
    params:
      script: ~
    inputs:
      env: ~
    command: "{{ params.script }} {{ inputs.env }}"

nodes:
  - name: deploy
    uses: deployer
    with:
      script: ./deploy.sh
`
		root := requireBuildOK(t, yml)
		r := requireRunnable(t, root.Children[0], "deploy", "./deploy.sh {{ inputs.env }}")
		if r.Inputs == nil {
			t.Fatal("expected Inputs to be propagated from type definition")
		}
		if _, ok := r.Inputs["env"]; !ok {
			t.Error("expected input 'env' to be declared")
		}
	})

	t.Run("inputs forbidden on container node: error", func(t *testing.T) {
		yml := `
- name: group
  inputs:
    env: ~
  children:
    - name: sub
      command: echo hi
`
		requireBuildErr(t, yml, "phase=raw", "inputs", "runnable or pipeline")
	})

	t.Run("inputs forbidden on abstract node (uses): error", func(t *testing.T) {
		yml := `
types:
  svc:
    command: echo svc

nodes:
  - name: s
    inputs:
      env: ~
    uses:
      - svc
`
		requireBuildErr(t, yml, "phase=raw", "inputs", "runnable or pipeline")
	})

	t.Run("undeclared {{ inputs.name }} ref in command: error", func(t *testing.T) {
		yml := `
- name: deploy
  command: ./deploy.sh {{ inputs.env }}
`
		requireBuildErr(t, yml, "phase=raw", "unknown input", "env")
	})

	t.Run("undeclared {{ inputs.name }} ref in pipeline step: error", func(t *testing.T) {
		yml := `
- name: build
  inputs:
    target: ~
  steps:
    - command: go build {{ inputs.target }}
    - command: go test {{ inputs.pkg }}
`
		requireBuildErr(t, yml, "phase=raw", "unknown input", "pkg")
	})

	t.Run("{{ inputs.name }} in command string form: allowed", func(t *testing.T) {
		yml := `
- name: run
  inputs:
    file: ~
  command: bash {{ inputs.file }}
`
		root := requireBuildOK(t, yml)
		requireRunnable(t, root.Children[0], "run", "bash {{ inputs.file }}")
	})

	t.Run("{{ inputs.name }} preserved as literal after type expansion", func(t *testing.T) {
		yml := `
types:
  runner:
    inputs:
      file: ~
    command: bash
    args:
      - "{{ inputs.file }}"

nodes:
  - name: run
    uses: runner
`
		root := requireBuildOK(t, yml)
		r := requireRunnable(t, root.Children[0], "run", "bash {{ inputs.file }}")
		// The inputs.file reference must be preserved as a literal (not substituted).
		if r.Argv[1] != "{{ inputs.file }}" {
			t.Errorf("argv[1]: want literal '{{ inputs.file }}', got %q", r.Argv[1])
		}
	})

	t.Run("unrecognized template expression (old {{ .param }} syntax): error", func(t *testing.T) {
		yml := `
types:
  svc:
    params:
      port: ~
    command: ./server --port {{ .port }}

nodes:
  - name: s
    uses: svc
    with:
      port: "8080"
`
		requireBuildErr(t, yml, "unrecognized template expression", "{{ .port }}", "params.name")
	})

	t.Run("unrecognized {{ unknown.x }} namespace: error", func(t *testing.T) {
		yml := `
types:
  svc:
    params:
      port: ~
    command: ./server --port {{ env.PORT }}

nodes:
  - name: s
    uses: svc
    with:
      port: "8080"
`
		requireBuildErr(t, yml, "unrecognized template expression", "{{ env.PORT }}")
	})

	t.Run("input name with hyphen: accepted", func(t *testing.T) {
		yml := `
- name: deploy
  inputs:
    target-env: staging
  command: ./deploy.sh {{ inputs.target-env }}
`
		root := requireBuildOK(t, yml)
		r := requireRunnable(t, root.Children[0], "deploy", "./deploy.sh {{ inputs.target-env }}")
		if r.Inputs == nil {
			t.Fatal("expected Inputs to be set")
		}
		def := r.Inputs["target-env"]
		if def == nil || *def != "staging" {
			t.Errorf("input 'target-env' default: want 'staging', got %v", def)
		}
	})

	t.Run("inputs: multiple required and optional, all propagated", func(t *testing.T) {
		yml := `
- name: release
  inputs:
    version: ~
    registry: docker.io
    tag: latest
  command: docker push {{ inputs.registry }}/app:{{ inputs.version }}-{{ inputs.tag }}
`
		root := requireBuildOK(t, yml)
		r := requireRunnable(t, root.Children[0], "release",
			"docker push {{ inputs.registry }}/app:{{ inputs.version }}-{{ inputs.tag }}")
		if len(r.Inputs) != 3 {
			t.Errorf("expected 3 inputs, got %d", len(r.Inputs))
		}
		// version: required
		if r.Inputs["version"] != nil {
			t.Error("input 'version' should be required (nil default)")
		}
		// registry: optional with default
		if r.Inputs["registry"] == nil || *r.Inputs["registry"] != "docker.io" {
			t.Errorf("input 'registry' default: want 'docker.io'")
		}
		// tag: optional with default
		if r.Inputs["tag"] == nil || *r.Inputs["tag"] != "latest" {
			t.Errorf("input 'tag' default: want 'latest'")
		}
	})

	t.Run("inputs: error message references unrecognized {{ inputs.x }} against dsl.ErrUnknownInput", func(t *testing.T) {
		yml := `
- name: deploy
  inputs:
    env: ~
  command: ./deploy.sh {{ inputs.env }} {{ inputs.region }}
`
		_, err := Build([]byte(yml))
		if err == nil {
			t.Fatal("expected error but got none")
		}
		if !errors.Is(err, dsl.ErrUnknownInput) {
			t.Errorf("expected ErrUnknownInput in error chain, got: %v", err)
		}
	})
}
