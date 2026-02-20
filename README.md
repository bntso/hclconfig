# hclconfig

A Go library that parses HCL configuration files with **cross-block variable resolution** and a built-in `env()` function. Define your config schema as Go structs with `hcl` struct tags — the library handles dependency-aware ordered decoding so that `${database.host}` in one block resolves to the value from another block automatically.

## Install

```bash
go get github.com/bntso/hclconfig@v0.1.0
```

## Usage

### Define your config schema

```go
type Config struct {
    Database DatabaseConfig `hcl:"database,block"`
    App      AppConfig      `hcl:"app,block"`
}

type DatabaseConfig struct {
    Host string `hcl:"host,attr"`
    Port int    `hcl:"port,attr"`
}

type AppConfig struct {
    DBUrl string `hcl:"db_url,attr"`
}
```

### Write an HCL config file

```hcl
database {
    host = "localhost"
    port = 5432
}

app {
    db_url = "postgres://${database.host}:${database.port}/mydb"
}
```

### Load it

```go
var cfg Config
err := hclconfig.LoadFile("config.hcl", &cfg)
// cfg.App.DBUrl == "postgres://localhost:5432/mydb"
```

Block order in the HCL file doesn't matter — dependencies are resolved automatically.

## Features

### Cross-block references

Reference values from other blocks using `${block.attribute}` syntax. Dependencies are analyzed and blocks are decoded in the correct order.

```hcl
database {
    host = "localhost"
    port = 5432
}

app {
    db_url = "postgres://${database.host}:${database.port}/mydb"
}
```

### Environment variables

Use the built-in `env()` function to read environment variables.

```hcl
database {
    host     = env("DB_HOST")
    password = env("DB_PASSWORD")
}
```

### Labeled blocks

Blocks with labels are accessible by their label name.

```hcl
service "api" {
    host = "api.example.com"
    port = 8080
}

service "web" {
    host = "web.example.com"
    port = 3000
}

app {
    api_url = "http://${service.api.host}:${service.api.port}"
    web_url = "http://${service.web.host}:${service.web.port}"
}
```

```go
type ServiceConfig struct {
    Name string `hcl:"name,label"`
    Host string `hcl:"host,attr"`
    Port int    `hcl:"port,attr"`
}

type Config struct {
    Services []ServiceConfig `hcl:"service,block"`
    App      AppConfig       `hcl:"app,block"`
}
```

### Nested blocks

Nested blocks are converted to nested objects, allowing deep references.

```hcl
database {
    host = "localhost"
    port = 5432

    credentials {
        username = "admin"
        password = "secret"
    }
}

app {
    conn = "postgres://${database.credentials.username}:${database.credentials.password}@${database.host}:${database.port}/mydb"
}
```

### Optional blocks

Use pointer fields for blocks that may not be present.

```go
type Config struct {
    Database DatabaseConfig `hcl:"database,block"`
    App      *AppConfig     `hcl:"app,block"` // nil if not in config file
}
```

### Custom EvalContext

Pass additional variables or functions via `WithEvalContext`.

```go
ctx := &hcl.EvalContext{
    Variables: map[string]cty.Value{
        "region": cty.StringVal("us-east-1"),
    },
}

var cfg Config
err := hclconfig.LoadFile("config.hcl", &cfg, hclconfig.WithEvalContext(ctx))
```

## API

```go
func LoadFile(filename string, dst interface{}, opts ...Option) error
func Load(src []byte, filename string, dst interface{}, opts ...Option) error
func WithEvalContext(ctx *hcl.EvalContext) Option
```

### Error types

- **`CycleError`** — returned when blocks have circular dependencies
- **`DiagnosticsError`** — wraps HCL diagnostics (parse errors, unknown variables, etc.)

```go
var cfg Config
err := hclconfig.LoadFile("config.hcl", &cfg)

var cycleErr *hclconfig.CycleError
if errors.As(err, &cycleErr) {
    fmt.Println("cycle:", cycleErr.Cycle)
}

var diagErr *hclconfig.DiagnosticsError
if errors.As(err, &diagErr) {
    for _, d := range diagErr.Diags {
        fmt.Println(d.Summary)
    }
}
```

## License

MIT
