# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Go library (`github.com/bntso/hclconfig`) for parsing HCL configuration files with dependency-aware variable resolution. It decodes HCL into Go structs, automatically resolving cross-block references, labeled block references, nested block references, and environment variables in the correct order via topological sorting.

## Commands

```bash
# Run all tests
go test ./...

# Run a specific test
go test -v -run TestLoadFile_Simple

# Run tests with coverage
go test -cover ./...
```

There is no Makefile, linter config, or CI pipeline. Standard `go build`/`go test` tooling only.

## Architecture

The library has a single-package design with a clear pipeline:

1. **Parse** HCL source into AST (`loader.go`)
2. **Extract** schema from target Go struct via reflection and `hcl` struct tags
3. **Build dependency graph** from variable references in block/attribute bodies (`resolve.go`)
4. **Topological sort** (Kahn's algorithm) with cycle detection (`resolve.go`)
5. **Decode** blocks/attributes in dependency order, updating the eval context incrementally (`loader.go`)
6. **Convert** decoded Go structs back to `cty.Value` so later blocks can reference earlier ones (`convert.go`)

### Key files

- **`loader.go`** — Public API (`LoadFile`, `Load`, `WithEvalContext`), schema extraction, ordered decoding loop
- **`resolve.go`** — Dependency graph construction, topological sort, cycle detection (`CycleError`)
- **`convert.go`** — Bidirectional Go struct ↔ `cty.Value` conversion using reflection
- **`context.go`** — Base eval context with built-in `env()` function
- **`errors.go`** — `CycleError` and `DiagnosticsError` types

### HCL struct tag conventions

- `hcl:"name,attr"` / `hcl:"name,optional"` — top-level attribute
- `hcl:"name,block"` — block (use pointer for optional, slice for repeatable)
- `hcl:"name,label"` — block label field

### Test structure

Tests live alongside source files (`*_test.go`). Each test file defines its own struct types. Integration tests in `loader_test.go` use HCL fixtures from `testdata/`.
