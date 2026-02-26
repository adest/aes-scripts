# Execution DSL Specification

Version 1.2 (Draft)

---

# 1. Overview

This DSL defines a hierarchical execution model composed of:

* **Containers**: group nodes
* **Runnables**: define executable commands
* **Abstract nodes**: type-driven nodes whose structure is defined by expansion

The DSL is **declarative**. A node's final type is determined either:

* explicitly (`command` or `children`), or
* implicitly through type expansion (`uses`).

The configuration represents an execution tree.

Containers can only contain other nodes.
Runnables define commands that are executed by the application.

---

# 2. Top-Level Structure

The DSL can be provided in one of two YAML shapes.

## 2.1 Document Form (Preferred)

A **complete DSL file** is a mapping with two top-level keys:

* `types`: a mapping of type name → type definition
* `nodes`: the root execution list (the root itself is an **implicit container**)

This format allows building a runtime model with an **empty registry**, because the type registry is fully provided by the YAML file.

Example:

```yaml
types:
  docker-compose:
    params:
      file: ~
      profile: dev
    children:
      - name: up
        command: docker compose -f {{ .file }} --profile {{ .profile }} up -d
      - name: down
        command: docker compose -f {{ .file }} down

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
```

### 2.1.1 Types

* Each entry under `types` is a **type definition**.
* A type definition is a node body (same shape as a `RawNode`) with an optional `params` block.
* A type name must be unique within the document.
* A type definition is expanded when referenced by an abstract node via `uses`.

## 2.2 Shorthand Form (Backward-Compatible)

For backward compatibility, the YAML document may be a **list of nodes** directly.
In that case, it is interpreted as `nodes` and `types` is empty.

Example:

```yaml
- name: backend
  children:
    - name: build
      command: go build ./...
```

---

# 3. Node Definition

Each node MUST define **exactly one** of the following properties:

* `command`
* `children`
* `uses`
* `steps`

This is a strict **XOR rule**.

---

## 3.1 Runnable Node

A **runnable node** defines a command to execute. The `command` key supports three equivalent forms.

---

**Compact string form**

```yaml
- name: up
  command: docker compose up -d
```

The string is split into argv (shlex-style, deterministic, no implicit shell).

---

**Array form**

```yaml
- name: up
  command: ["docker", "compose", "up", "-d"]
```

Each element is an argv token. No splitting is performed.

---

**Long form with `args`**

```yaml
- name: up
  command: docker
  args:
    - compose
    - up
    - "-d"
```

`command` is the executable token (single word). `args` provides the additional arguments.

---

All three forms produce the same argv: `["docker", "compose", "up", "-d"]`.

**Properties:**

| Key | Required | Type | Description |
|-----|----------|------|-------------|
| `command` | yes | string or sequence of strings | The executable and its arguments |
| `args` | no | sequence of strings | Additional arguments. Forbidden if `command` is an array |
| `cwd` | no | string | Working directory for command execution |
| `env` | no | map<string, string> | Environment variables for command execution |

**Constraints:**

* MUST define `command`
* `command` MUST NOT be empty (empty string, empty array, or empty first token)
* If `command` is an **array** → `args` MUST NOT be present
* If `command` is a **string** and `args` is present → `command` MUST be a single token (no whitespace)
* If `command` is a **string** and `args` is absent → the string is split as argv (shlex-style)
* MUST NOT define `children`
* MUST NOT define `uses`

**Template substitution** (applicable inside type bodies only):

* String form: template applied to the full string **before** argv splitting
* Array form: template applied to **each element** independently
* Long form: template applied to the `command` token and to **each element** of `args` independently

---

## 3.2 Container Node

A **container node** groups child nodes.

Example:

```yaml
- name: backend
  children:
    - name: build
      command: go build
    - name: test
      command: go test ./...
```

**Constraints:**

* MUST define `children`
* MUST NOT define `command`
* MUST NOT define `uses`
* MUST contain at least **one child**

---

## 3.3 Abstract Node (Type-Driven)

An **abstract node** delegates its structure to one or more types.

Example (shorthand string form — single type):

```yaml
- name: stack
  uses: docker-compose
```

Example (shorthand string form with params):

```yaml
- name: stack
  uses: docker-compose
  with:
    file: docker-compose.yml
    profile: production
```

Example (list form — multiple types):

```yaml
- name: stack
  uses:
    - docker-compose
    - kubernetes
```

Example (list form, shared param bag):

```yaml
- name: stack
  uses:
    - docker-compose
  with:
    file: docker-compose.yml
    profile: production
```

Example (per-type params, for multi-type with distinct param sets):

```yaml
- name: stack
  uses:
    - docker-compose
    - kubernetes
  with:
    - type: docker-compose
      file: docker-compose.yml
    - type: kubernetes
      namespace: production
```

**Properties:**

- `uses` (required): string or sequence of strings. A string is normalized to a single-element list at parse time.
- `with` (optional): parameters passed to the expanded type(s). Two forms are accepted:
  - **Mapping form** — a flat map of scalar values, shared across all types in `uses`.
  - **List form** — a list of objects, each with a `type` key and scalar param entries. Used when different types in `uses` require distinct param sets.

**Constraints:**

* MUST define `uses`
* MUST NOT define `command`
* MUST NOT define `children`
* `uses` MUST NOT be empty (non-empty string, or list with at least one entry)
* `with` values MUST be scalars (string or number)
* In list form, each entry MUST declare a `type` that is present in `uses`

**Note:** Expansion determines whether this node becomes:

* Runnable
* Container
* Pipeline
* Container with intermediate nodes

---

## 3.4 Pipeline Node

A **pipeline node** defines an ordered sequence of steps executed synchronously, in declaration order.

Example:

```yaml
- name: deploy
  steps:
    - command: docker compose pull
    - command: docker compose up -d
```

**Properties:**

| Key | Required | Type | Description |
|-----|----------|------|-------------|
| `steps` | yes | sequence of steps | Ordered list of steps to execute |

**Constraints:**

* MUST define `steps`
* MUST NOT define `command`, `children`, or `uses`
* `steps` MUST NOT be empty (at least one step required)

Pipeline nodes are **executable** (selectable by path), not grouping nodes. Execution is **fail-fast by default**: if a step fails, the pipeline stops immediately unless that step declares `on-fail: continue` or `on-fail: { action: retry, ... }`.

---

### 3.4.1 Step Structure

Each step in a pipeline defines a command and its execution context.

**Properties:**

| Key | Required | Type | Description |
|-----|----------|------|-------------|
| `command` | yes | string or sequence of strings | Same forms as a runnable node |
| `args` | no | sequence of strings | Same semantics as a runnable node |
| `id` | no | string | Unique identifier for this step within the pipeline |
| `cwd` | no | string | Working directory |
| `env` | no | map<string, string> | Environment variables |
| `capture` | no | `stdout` \| `stderr` \| `both` | Streams to buffer in memory |
| `tee` | no | boolean | If `true`, captured streams are also forwarded to the terminal (default: `false`) |
| `stdin` | no | step-ref | Stdin source. Format: `steps.<id>.stdout` or `steps.<id>.stderr` |
| `on-fail` | no | string or mapping | Failure behavior (see below). Default: `fail` |

**Step constraints:**

* MUST have a non-empty `command` (same emptiness rules as runnable: empty string, empty array, and empty first token are all forbidden)
* If `command` is an **array** → `args` MUST NOT be present
* If `command` is a **string** and `args` is present → `command` MUST be a single token (no whitespace)
* `id` MUST be unique within the step list (case-sensitive)
* `id` MUST NOT be empty if declared
* `id` MUST be a static identifier — `{{ }}` expressions are forbidden inside `id` values
* `capture` MUST be one of: `stdout`, `stderr`, `both`
* `capture` requires `id` — a step without `id` MUST NOT declare `capture` (the buffered output would be unreachable)
* `tee: true` requires `capture` — declaring `tee` without `capture` is an error
* `stdin` MUST reference a step that: (1) exists in the same step list, (2) appears before the current step, (3) has a `capture` that includes the referenced stream

---

### 3.4.2 `on-fail` Field

Controls the behavior when a step exits with a non-zero code.

**String shorthand** (for `fail` and `continue`):

```yaml
steps:
  - command: docker compose down
    on-fail: continue       # swallow failure, proceed to next step

  - command: docker compose up -d
    # on-fail: fail         # implicit default
```

**Structured form** (required for `retry`):

```yaml
steps:
  - command: curl https://api.example.com/health
    on-fail:
      action: retry
      attempts: 3           # total attempts including the first (minimum: 2)
      delay: 2s             # wait between attempts (optional, default: 0s)
```

**`on-fail` constraints:**

* String form: value MUST be `fail` or `continue`
* Structured form: `action` MUST be `retry`; string shorthand MUST NOT be used for `retry`
* `attempts` MUST be an integer ≥ 2
* `delay` is a duration string in Go format (`1s`, `500ms`, `1m30s`); defaults to `0s` if absent
* If all retry attempts are exhausted, the step is considered failed; the pipeline stops (fail-fast applies normally at that point)

**Semantics of `on-fail: continue`:**

* The step's non-zero exit code is recorded but does not stop execution
* The pipeline continues to the next step
* If all remaining steps succeed (or also declare `on-fail: continue`), the pipeline exits with code 0
* A step with `on-fail: continue` that also has `capture` will have its captured output set to whatever was produced before the failure

---

## 3.5 Step Output Substitution

Step output references allow a step to inject the buffered output of a preceding step into its `command`, `args`, `env`, or `cwd`.

**Syntax:** `{{ steps.<id>.<stream> }}`

Where:
* `<id>` is the `id` of a step defined **before** the current step in the same pipeline
* `<stream>` is `stdout` or `stderr`

**Valid substitution locations:**

| Field | Valid | Notes |
|-------|-------|-------|
| `args` (string values) | yes | Each element is an arg atom — no splitting occurs, even if the output contains spaces |
| `command` (array form, per element) | yes | Each element is an arg atom — no splitting occurs |
| `command` (string form) | **no** | Forbidden: substitution happens before argv splitting; spaces in captured output would produce incorrect token boundaries |
| `env` (values) | yes | Applied per value, no splitting |
| `cwd` | yes | Applied to the full string, no splitting |
| `id` | **no** | Static identifier — substitution forbidden |
| `capture` | **no** | Enum value — substitution forbidden |
| `stdin` | **no** | Already a step-ref, not a string |

**Substitution rules:**

* The referenced step MUST exist before the current step in declaration order
* The referenced step MUST have `capture` set to include the referenced stream
* The captured output is trimmed of **trailing newlines only** before substitution — no other transformation is applied
* Multiple substitutions within a single string are resolved left to right
* Substitution is performed at **execution time**, after the referenced step completes

**Distinction from type parameter substitution:**

| Syntax | Resolved at | Scope | Leading character |
|--------|-------------|-------|-------------------|
| `{{ .param }}` | Expansion (Phase 2) | Type bodies | Leading dot (`.`) |
| `{{ steps.id.stream }}` | Execution | Pipeline steps | `steps.` prefix, no leading dot |

These syntaxes are distinguishable by the presence or absence of the leading dot. When a type body contains a pipeline step with step output references, those references are **preserved as literals** during Phase 2 template expansion and resolved at execution time.

---

# 4. Type Parameters

A type definition MAY declare a `params` block to specify the parameters it accepts.

## 4.1 Param Declaration

```yaml
types:
  docker-compose:
    params:
      file: ~        # null  → required, no default
      profile: dev   # scalar → optional, default value is "dev"
    children:
      - name: up
        command: docker compose -f {{ .file }} --profile {{ .profile }} up -d
      - name: stop
        command: docker compose -f {{ .file }} stop
```

Rules:

* A param with value `null` (`~`) is **required**. The engine MUST error if the caller does not supply it.
* A param with a scalar value is **optional**. That value is used as the default when the caller omits the param.
* All param values are treated as **strings** at substitution time, regardless of whether they were declared or passed as numbers.
* Unknown params passed via `with` that are not declared in `params` MUST cause a validation error.

## 4.2 Template Syntax

Type definitions use **Go template syntax** (`{{ .paramName }}`) for substitution.

* Templates are applied to **all string values** in the type body, including `name`, `command`, `cwd`, `env` values, and nested `with` values.
* The `name` field of a type's root node is also subject to template substitution. This provides a default name for the expanded node, derived from params (see section 5.2).
* Name uniqueness is checked **after** template substitution, on the resolved values.
* Substitution happens before child types are expanded, so template values flow correctly into nested `uses`.

Example — dynamic child names:

```yaml
types:
  service:
    params:
      name: ~
    children:
      - name: "{{ .name }}-up"
        command: docker compose up {{ .name }}
      - name: "{{ .name }}-down"
        command: docker compose down {{ .name }}
```

Example — type root name as default:

```yaml
types:
  docker-compose:
    name: "compose-{{ .file }}"   # used as fallback name in multi-type expansion
    params:
      file: ~
    children:
      - name: up
        command: docker compose -f {{ .file }} up -d
```

Example with nested parameterized types:

```yaml
types:
  full-stack:
    params:
      compose-file: ~
      k8s-namespace: staging
    children:
      - name: docker
        uses:
          - docker-compose
        with:
          file: "{{ .compose-file }}"
      - name: k8s
        uses:
          - kubernetes
        with:
          namespace: "{{ .k8s-namespace }}"
```

---

# 5. Type Expansion

Types are **registered in a type registry**.

Each type:

* Receives a raw node and a resolved param map
* Applies template substitution to all string values (including `name` fields)
* May generate:

  * A runnable command
  * Child nodes
  * Intermediate containers
* Must produce a valid final node

### 5.1 Expansion Rules

* `uses` are **only allowed on abstract nodes**
* After expansion, each node must satisfy the runtime validation rules
* Multiple `uses` are expanded **in order**, and children are appended sequentially
* Node names must remain **unique among siblings** after expansion (checked on resolved names)
* Expansion is **recursive**: if the expanded body itself contains abstract nodes, those are expanded in subsequent passes

### 5.2 Name Resolution (Abstract Nodes)

When an abstract node is expanded via `uses`, the **node name from the execution tree** (the abstract node's `name`) has priority.

Specifically:

* If the abstract node declares a `name`, that name is used for the resulting expanded node — even if the type definition also declares a `name` at its root.
* The type root `name` (after template substitution) is used as a fallback when the type is expanded as a child in a multi-type expansion and no explicit name is provided by the caller.

---

# 6. Name Rules

* `name` is mandatory
* Names must be **unique among siblings**
* Names are **case-sensitive**
* Names must not be empty
* Names are resolved after template substitution

---

# 7. Validation Phases

The DSL engine processes nodes in **three phases**:

---

## Phase 1 — Raw Validation

Validates syntax and structure **before expansion**.

Rules:

* `name` is required
* Exactly one of `command`, `children`, `uses`, or `steps` must be defined
* Children are recursively validated
* Sibling names must be unique
* `uses` is only allowed on abstract nodes
* Node must not be empty
* `with` mapping form: values must be scalars
* `with` list form: each entry must have a `type` key; values must be scalars
* `uses` string form is normalized to a single-element list; the resulting list must be non-empty
* `command` MUST NOT be empty (empty string, empty array, or empty first token)
* If `command` is an **array** → `args` MUST NOT be present
* If `command` is a **string** and `args` is present → `command` MUST be a single token (no whitespace)
* `args` is only valid on runnable nodes or steps (requires `command`)

**Additional rules for pipeline nodes (`steps`):**

* `steps` MUST NOT be empty
* Each step MUST have a non-empty `command` (same emptiness rules as runnable)
* If a step `command` is an **array** → `args` MUST NOT be present on that step
* If a step `command` is a **string** and `args` is present → `command` MUST be a single token
* Step `id` values MUST be unique within the step list (case-sensitive)
* Step `id` MUST NOT be empty if declared
* Step `id` MUST be a static identifier — `{{ }}` expressions are forbidden in `id` values
* `capture` MUST be one of: `stdout`, `stderr`, `both`
* `capture` requires `id` — declaring `capture` without `id` is an error
* `tee: true` requires `capture` — declaring `tee` without `capture` is an error
* `stdin` MUST match the format `steps.<id>.<stream>` where `<stream>` is `stdout` or `stderr`
* `stdin` reference: the referenced step MUST exist earlier in the list AND have a `capture` that includes the referenced stream
* Step output references (`{{ steps.<id>.<stream> }}`) are valid in `args` elements, `command` array form elements, `env` values, and `cwd` — and are **forbidden in `command` string form**
* For valid step output references: the referenced step MUST exist earlier in the list AND have a `capture` that includes the referenced stream
* `on-fail` string form: value MUST be `fail` or `continue`
* `on-fail` structured form: `action` MUST be `retry`; `attempts` MUST be an integer ≥ 2; `delay` if present MUST be a valid duration string

---

## Phase 2 — Expansion

* Required params are checked: missing required param → error
* Unknown params (not declared in `params`) → error
* Default values are applied for omitted optional params
* Template substitution is applied to all string values in the type body (including `name` fields)
* For `command`:
  * String form: template applied to the full string **before** argv splitting
  * Array form: template applied to **each element** independently
  * Long form: template applied to the `command` token and to **each element** of `args` independently
* Step output references (`{{ steps.<id>.<stream> }}`) are **preserved as literals** during Phase 2 template expansion — they are not resolved by the Go template engine and remain intact for execution-time resolution
* Name uniqueness is checked after substitution
* All `uses` are expanded using the type registry
* Expansion is recursive until no abstract nodes remain
* After full expansion, no node may contain `uses`
* Expansion must be **deterministic**

---

## Phase 3 — Runtime Validation

After expansion, nodes must satisfy:

* Exactly **one** of:

  * `command`
  * `children`
  * `steps`
* Containers must have **at least one child**
* Runnables must have **non-empty command**
* Pipelines must have **at least one step** with a non-empty argv
* No duplicate names among siblings

---

# 8. Execution Model

* The application **selects a runnable or pipeline node** by path and executes it
* Containers are **not executable**
* Path example: `backend.build`, `deploy`, `login`
* Execution is outside the DSL engine responsibility

## 8.1 Runnable Execution

The command argv is executed directly (no implicit shell). `cwd` and `env` are applied if set.

## 8.2 Pipeline Execution

Steps are executed **synchronously, in declaration order**. For each step:

1. Resolve all step output references (`{{ steps.<id>.<stream> }}`) in `command`, `args`, `env`, and `cwd` — using the buffered output of the referenced step, trimmed of trailing newlines
2. Connect `stdin` if specified — the buffered output of the referenced step is fed to the process stdin
3. Execute the command directly (no implicit shell)
4. Buffer the output streams declared in `capture`; if `tee: true`, also forward them to the terminal
5. Streams not declared in `capture` are forwarded to the terminal as usual
6. Evaluate the exit code against `on-fail`:
   * `fail` (default): non-zero exit → stop pipeline, report failure
   * `continue`: non-zero exit → record failure silently, proceed to next step
   * `retry`: re-execute the step up to `attempts` times total, waiting `delay` between each attempt; if all attempts fail → fail-fast

**Pipeline exit code:** the pipeline succeeds (exit code 0) if and only if every step either succeeded or declared `on-fail: continue`. The first step that fails without `on-fail: continue` (including a retry that exhausted its attempts) determines the pipeline failure.

---

# 9. Error Handling

The engine must return descriptive errors including:

* Node path
* Validation phase
* Reason for failure

Examples:

* Duplicate sibling name
* Invalid XOR rule
* Empty container after expansion
* Type expansion failure
* Missing required param
* Unknown param supplied

---

# 10. Determinism

Given:

* The same configuration
* The same registered types

The expansion process **must be deterministic**.

---

# 11. Examples

### Runnable Node — compact string form

```yaml
- name: build
  command: go build ./...
```

### Runnable Node — array form

```yaml
- name: run
  command: ["docker", "run", "--rm", "myimage"]
```

### Runnable Node — long form with args

```yaml
- name: up
  command: docker
  args:
    - compose
    - up
    - "-d"
```

### Runnable Node — with cwd and env

```yaml
- name: build
  command: go build ./...
  cwd: ./backend
  env:
    GOFLAGS: "-mod=vendor"
```

### Container Node

```yaml
- name: backend
  children:
    - name: build
      command: go build
    - name: test
      command: go test ./...
```

### Abstract Node — shorthand string form

```yaml
- name: stack
  uses: docker-compose
```

### Abstract Node — shorthand with params

```yaml
- name: stack
  uses: docker-compose
  with:
    file: docker-compose.yml
```

### Abstract Node — list form (multiple types)

```yaml
- name: stack
  uses:
    - docker-compose
    - kubernetes
```

### Parameterized type with shared params

```yaml
types:
  docker-compose:
    params:
      file: ~
      profile: dev
    children:
      - name: lifecycle
        children:
          - name: up
            command: docker compose -f {{ .file }} --profile {{ .profile }} up -d
          - name: stop
            command: docker compose -f {{ .file }} stop

nodes:
  - name: stack
    uses:
      - docker-compose
    with:
      file: docker-compose.yml
```

Resulting tree:

```
stack
 └─ lifecycle
      ├─ up
      └─ stop
```

### Abstract Node with per-type params

```yaml
- name: infra
  uses:
    - docker-compose
    - kubernetes
  with:
    - type: docker-compose
      file: docker-compose.yml
    - type: kubernetes
      namespace: production
      replicas: 3
```

### Nested parameterized types

```yaml
types:
  full-stack:
    params:
      compose-file: ~
      k8s-namespace: staging
    children:
      - name: docker
        uses:
          - docker-compose
        with:
          file: "{{ .compose-file }}"
      - name: k8s
        uses:
          - kubernetes
        with:
          namespace: "{{ .k8s-namespace }}"

nodes:
  - name: prod
    uses:
      - full-stack
    with:
      compose-file: docker-compose.prod.yml
      k8s-namespace: production
```

### Pipeline Node — sequential steps

```yaml
- name: deploy
  steps:
    - command: docker compose pull
    - command: docker compose up -d
```

### Pipeline Node — capture and inject into args

```yaml
- name: login
  steps:
    - id: bw
      command: bw get password my-registry
      capture: stdout

    - command: docker
      args:
        - login
        - registry.example.com
        - --username
        - user
        - --password
        - "{{ steps.bw.stdout }}"
```

### Pipeline Node — explicit pipe via stdin

```yaml
- name: find-go-files
  steps:
    - id: list
      command: ls
      capture: stdout

    - command: grep
      args:
        - ".go"
      stdin: steps.list.stdout
```

### Pipeline Node — tee (capture + display)

```yaml
- name: build-and-deploy
  steps:
    - id: version
      command: git describe --tags
      capture: stdout
      tee: true             # also printed to terminal

    - command: docker build
      args:
        - --tag
        - "myapp:{{ steps.version.stdout }}"
        - .
```

### Pipeline Node — on-fail: continue

```yaml
- name: restart
  steps:
    - command: docker compose down
      on-fail: continue     # may fail if nothing is running; that's fine

    - command: docker compose up -d
```

### Pipeline Node — on-fail: retry

```yaml
- name: wait-for-api
  steps:
    - command: curl
      args:
        - --fail
        - --silent
        - https://api.example.com/health
      on-fail:
        action: retry
        attempts: 5
        delay: 3s

    - command: run-migrations
```

### Pipeline Node — step output in cwd and env

```yaml
- name: build-in-workspace
  steps:
    - id: find-root
      command: find-project-root
      capture: stdout

    - command: make
      args: [build]
      cwd: "{{ steps.find-root.stdout }}"
      env:
        BUILD_DIR: "{{ steps.find-root.stdout }}/dist"
```

### Pipeline type with params

```yaml
types:
  docker-login:
    params:
      registry: ~
      username: ~
    steps:
      - id: token
        command: get-token
        args:
          - "{{ .registry }}"
        capture: stdout

      - command: docker
        args:
          - login
          - "{{ .registry }}"
          - --username
          - "{{ .username }}"
          - --password
          - "{{ steps.token.stdout }}"

nodes:
  - name: login
    uses: docker-login
    with:
      registry: registry.example.com
      username: ci-bot
```

### Multi-level Example

```yaml
- name: app
  children:
    - name: backend
      uses:
        - docker-compose
      with:
        file: backend/docker-compose.yml
    - name: frontend
      command: npm run start
```

---

# 12. Future Extensions (Non-Normative)

Potential features:

* JSON Schema generation from `params` declarations
* Linting support
* IDE tooling support
* Documentation generation from type registry
* `on-fail: { action: retry, ..., then: continue }` — fallback to continue after retry exhaustion
* `timeout` per step — kill a step after a duration, counts as failure
* `capture: both` with independent access to `steps.id.stdout` and `steps.id.stderr`

---

# Summary

This DSL defines a **strict, deterministic, and extensible execution tree model**:

* Clear separation between **declaration** and **expansion**
* Strong **validation guarantees**
* No ambiguous node types
* Deterministic runtime structure
* Supports abstract, reusable, composable types
* Parameterized types with required/optional params and Go template substitution
* Dynamic names via template substitution, resolved before uniqueness checks
