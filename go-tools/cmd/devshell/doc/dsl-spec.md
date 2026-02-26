# Execution DSL Specification

Version 1.1 (Draft)

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

This is a strict **XOR rule**.

---

## 3.1 Runnable Node

A **runnable node** defines a shell command.

Example:

```yaml
- name: build
  command: go build ./...
  cwd: ./backend
  env:
    GOFLAGS: "-mod=vendor"
```

**Optional properties:**

- `cwd`: string (optional) — working directory for command execution
- `env`: map<string, string> (optional) — environment variables for command execution

**Constraints:**

* MUST define `command`
* MAY define `cwd` (optional)
* MAY define `env` (optional)
* MUST NOT define `children`
* MUST NOT define `uses`
* `command` MUST NOT be empty

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

Example (no params):

```yaml
- name: stack
  uses:
    - docker-compose
```

Example (shared param bag):

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

- `with` (optional): parameters passed to the expanded type(s). Two forms are accepted:
  - **Mapping form** — a flat map of scalar values, shared across all types in `uses`.
  - **List form** — a list of objects, each with a `type` key and scalar param entries. Used when different types in `uses` require distinct param sets.

**Constraints:**

* MUST define `uses`
* MUST NOT define `command`
* MUST NOT define `children`
* MUST contain at least **one entry in `uses`**
* `with` values MUST be scalars (string or number)
* In list form, each entry MUST declare a `type` that is present in `uses`

**Note:** Expansion determines whether this node becomes:

* Runnable
* Container
* Container with intermediate nodes

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
* Exactly one of `command`, `children`, or `uses` must be defined
* Children are recursively validated
* Sibling names must be unique
* `uses` is only allowed on abstract nodes
* Node must not be empty
* `with` mapping form: values must be scalars
* `with` list form: each entry must have a `type` key; values must be scalars

---

## Phase 2 — Expansion

* Required params are checked: missing required param → error
* Unknown params (not declared in `params`) → error
* Default values are applied for omitted optional params
* Template substitution is applied to all string values in the type body (including `name` fields)
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
* Containers must have **at least one child**
* Runnables must have **non-empty command**
* No duplicate names among siblings

---

# 8. Execution Model

* The application **selects a runnable node** by path and executes its command
* Containers are **not executable**
* Path example: `backend.build`
* Execution is outside the DSL engine responsibility

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

### Runnable Node

```yaml
- name: build
  command: go build ./...
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

### Abstract Node (no params)

```yaml
- name: stack
  uses:
    - docker-compose
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
