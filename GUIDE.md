# Development Guide - Code Conventions and Design Decisions

This document explains the code conventions, design patterns, and architectural decisions used in the Overnight LLM Code Generator PoC.

## 🎯 Core Philosophy

**Simplicity Over Features**: Every design decision prioritizes simplicity and clarity. This is a proof of concept, not a production system.

## 📐 Architecture Decisions

### Why a Fixed Pipeline?

The system uses a fixed, sequential pipeline instead of a dynamic task graph:

```go
tasks := []Task{
    {Type: TaskGenerateModels},
    {Type: TaskGenerateHandlers},
    {Type: TaskGenerateRepository},
    {Type: TaskGenerateTests},
}
```

**Rationale:**
- Predictable execution order
- Easier debugging and monitoring
- Reduced complexity in orchestration
- Sufficient for proving the concept

### Why SQLite?

We embed SQLite instead of using an external database:

```go
//go:embed schema.sql
var schemaSQL string
```

**Rationale:**
- Zero external dependencies at runtime
- Single binary distribution
- Persistent task history for debugging
- Sufficient for single-user PoC

### Why No Streaming?

The Ollama client waits for complete responses:

```go
payload := generateRequest{
    Stream: false,  // Wait for complete response
}
```

**Rationale:**
- Simpler error handling
- Easier response validation
- Reduced complexity in HTTP client
- Generation happens overnight (time not critical)

## 🏗️ Code Organization

### Package Structure

```
internal/
├── orchestrator/   # High-level coordination
├── llm/           # External service integration
├── storage/       # Data persistence
├── validator/     # Code quality checks
└── generator/     # (Would contain templates in v2)
```

Each package has a single, clear responsibility following the Single Responsibility Principle.

### Interface Design

Minimal interface usage, only where absolutely necessary:

```go
type LLMProvider interface {
    Complete(ctx context.Context, prompt string) (string, error)
    HealthCheck(ctx context.Context) error
}
```

**Rationale:**
- Interfaces add complexity
- PoC uses only Ollama (no need for abstraction)
- Can add interfaces in v2 if needed

## 📝 Coding Conventions

### Error Handling

Consistent error wrapping with context:

```go
if err != nil {
    return fmt.Errorf("failed to create task: %w", err)
}
```

**Pattern:**
- Always wrap errors with context
- Use `%w` for error chain preservation
- Fail fast - no complex retry logic

### Comments

Every exported type and function has a comment:

```go
// OllamaClient provides a simple HTTP client for interacting with Ollama API
// It handles code generation requests without streaming for simplicity
type OllamaClient struct {
    endpoint string       // Base URL for Ollama API
    model    string       // Model to use for generation
    client   *http.Client // HTTP client with timeout
}
```

**Style:**
- First line is a complete sentence
- Additional details on subsequent lines
- Inline comments for non-obvious fields

### Context Usage

All long-running operations accept context:

```go
func (o *Orchestrator) GenerateTodoAPI(ctx context.Context, projectName string) error {
    ctx, cancel := context.WithTimeout(ctx, o.limits.MaxRuntime)
    defer cancel()
    // ...
}
```

**Pattern:**
- Accept context as first parameter
- Apply timeouts at orchestration level
- Pass context through entire call chain

### Resource Management

Consistent cleanup with defer:

```go
db, err := storage.InitDB(*dbPath)
if err != nil {
    log.Fatal("Failed to initialize database:", err)
}
defer db.Close()
```

**Pattern:**
- Open resources
- Check errors immediately
- Defer cleanup right after successful open

## 🔧 Implementation Patterns

### Embedded Resources

Use `//go:embed` for compile-time inclusion:

```go
//go:embed schema.sql
var schemaSQL string

//go:embed prompts/*.txt
var promptFiles embed.FS
```

**Benefits:**
- Single binary distribution
- No runtime file dependencies
- Version-locked resources

### Status Reporting

Clear, emoji-prefixed console output:

```go
fmt.Printf("✓ Completed: %s\n", task.Type)
fmt.Printf("❌ Failed: %s\n", err)
fmt.Printf("🚀 Starting generation\n")
```

**Convention:**
- ✅/✓ for success
- ❌ for errors
- 🚀 for start
- 📦 for packaging/building
- 🔍 for checking/validating
- 💡 for tips/hints

### Configuration

Flag-based configuration with sensible defaults:

```go
var (
    output = flag.String("output", "./generated", "Output directory")
    model  = flag.String("model", "codellama:7b", "Model to use")
)
```

**Principles:**
- Every flag has a default
- Defaults work out-of-the-box
- No configuration files for PoC

## 🧪 Testing Strategy

### Unit Tests

Table-driven tests for comprehensive coverage:

```go
tests := []struct {
    name    string
    input   string
    want    string
    wantErr bool
}{
    {"valid input", "test", "expected", false},
    {"invalid input", "", "", true},
}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        // test implementation
    })
}
```

### Mock Strategy

Simple mock implementations in test files:

```go
type mockLLMProvider struct {
    response string
    err      error
}

func (m *mockLLMProvider) Complete(ctx context.Context, prompt string) (string, error) {
    return m.response, m.err
}
```

**Rationale:**
- No mocking frameworks (simplicity)
- Mocks defined near tests
- Only mock external dependencies

## 🚫 What We Don't Do

### No Premature Optimization

- No connection pooling (SQLite doesn't benefit)
- No caching (overnight execution)
- No parallel execution (sequential is simpler)

### No Over-Engineering

- No dependency injection frameworks
- No complex configuration systems
- No plugin architecture
- No microservices

### No External Dependencies (Beyond Essential)

Only two external dependencies:
1. `github.com/mattn/go-sqlite3` - SQLite driver
2. Standard library for everything else

## 📊 Metrics and Monitoring

### Simple JSON Status File

```json
{
  "stats": {
    "total_tasks": 4,
    "completed_tasks": 4,
    "duration": "5m30s"
  },
  "timestamp": "2024-01-01T12:00:00Z"
}
```

**Design:**
- Human-readable format
- Single file output
- No complex telemetry

## 🔐 Security Considerations

### Input Validation

- Validate all file paths
- Check output size limits
- Timeout all operations

### No Secrets

- No API keys in code
- No credentials in prompts
- Local execution only

## 🎓 Learning from This Code

### Key Takeaways

1. **Start Simple**: Begin with the minimum viable implementation
2. **Fail Fast**: Don't hide errors with complex retry logic
3. **Clear Intent**: Code should be self-documenting with clear names
4. **Explicit Over Implicit**: Be explicit about what the code does
5. **Composition Over Inheritance**: Use composition and interfaces sparingly

### Good Practices Demonstrated

- Consistent error handling
- Context propagation
- Resource cleanup with defer
- Embedded resources
- Clear package boundaries
- Comprehensive comments

### Areas for Improvement (v2)

- Dynamic task graphs
- Parallel execution
- Template system for prompts
- Web UI for monitoring
- Multiple LLM providers
- Incremental regeneration

## 📚 Additional Resources

### Go Best Practices
- [Effective Go](https://golang.org/doc/effective_go)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)

### LLM Code Generation
- [Ollama Documentation](https://github.com/ollama/ollama)
- [Prompt Engineering Guide](https://www.promptingguide.ai/)

### SQLite with Go
- [go-sqlite3 Documentation](https://github.com/mattn/go-sqlite3)

---

Remember: This is a proof of concept. The code prioritizes clarity and simplicity over performance and feature completeness. Use it as a foundation for learning and experimentation, not as production-ready software.