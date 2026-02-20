# Execution DSL Specification

Version 1.0 (Draft)

---

# 1. Overview

This DSL defines a hierarchical execution model composed of:

* **Containers**: group nodes
* **Runnables**: define executable commands
* **Abstract nodes**: type-driven nodes whose structure is defined by expansion

The DSL is **declarative**. A node’s final type is determined either:

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
```

### 2.1.1 Types

* Each entry under `types` is a **type definition** expressed as a `RawNode`.
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
```

**Constraints:**

* MUST define `command`
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

Example:

```yaml
- name: stack
  uses:
    - docker-compose
```

**Constraints:**

* MUST define `uses`
* MUST NOT define `command`
* MUST NOT define `children`
* MUST contain at least **one entry in `uses`**

**Note:** Expansion determines whether this node becomes:

* Runnable
* Container
* Container with intermediate nodes

---

# 4. Type Expansion

Types are **registered in a type registry**.

Each type:

* Receives a raw node
* May generate:

  * A runnable command
  * Child nodes
  * Intermediate containers
* Must produce a valid final node

### 4.1 Expansion Rules

* `uses` are **only allowed on abstract nodes**
* After expansion, each node must satisfy the runtime validation rules
* Multiple `uses` are expanded **in order**, and children are appended sequentially
* Node names must remain **unique among siblings** after expansion

### 4.2 Name Resolution (Abstract Nodes)

When an abstract node is expanded via `uses`, the **node name from the execution tree** (the abstract node's `name`) has priority.

Specifically:

* If a type definition also declares a `name` at its root, and the abstract node declares a `name`, the **abstract node name MUST be used** for the resulting expanded node.
* The type root `name` MAY still be used for intermediate/internal nodes produced by the type, or when a type is expanded as a child in a multi-type expansion.

---

# 5. Name Rules

* `name` is mandatory
* Names must be **unique among siblings**
* Names are **case-sensitive**
* Names must not be empty

---

# 6. Validation Phases

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

---

## Phase 2 — Expansion

* All `uses` are expanded using the type registry
* Types may inject commands, containers, runnables, or intermediate containers
* After expansion, no node may contain `uses`
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

# 7. Execution Model

* The application **selects a runnable node** by path and executes its command
* Containers are **not executable**
* Path example: `backend.build`
* Execution is outside the DSL engine responsibility

---

# 8. Error Handling

The engine must return descriptive errors including:

* Node path
* Validation phase
* Reason for failure

Examples:

* Duplicate sibling name
* Invalid XOR rule
* Empty container after expansion
* Type expansion failure

---

# 9. Determinism

Given:

* The same configuration
* The same registered types

The expansion process **must be deterministic**.

---

# 10. Examples

### Runnable Node Example

```yaml
- name: build
  command: go build ./...
```

### Container Node Example

```yaml
- name: backend
  children:
    - name: build
      command: go build
    - name: test
      command: go test ./...
```

### Abstract Node Example

```yaml
- name: stack
  uses:
    - docker-compose
```

### Multi-level Example

```yaml
- name: app
  children:
    - name: backend
      uses:
        - docker-compose
    - name: frontend
      command: npm run start
```

---

# 11. Future Extensions (Non-Normative)

Potential features:

* Parameter validation for types
* JSON Schema generation
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
