# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go-based proof of concept for autonomous overnight code generation using local LLMs via Ollama. The system generates a complete REST API with tests as a single compiled binary (~30MB) with only Ollama as an external runtime dependency.

## Build and Development Commands

### Building the Project
```bash
# Build the main binary
make build

# Build for multiple platforms (Mac ARM64, Linux AMD64)
make build-all

# Clean build artifacts and generated files
make clean
```

### Running the Application
```bash
# Run with default settings (generates Todo API in ./demo directory)
make run

# Run with custom parameters
./overnight-llm -output ./my-api -model codellama:7b -ollama http://localhost:11434

# Available flags:
# -prompt: Description of what to generate (default: "REST API for todo list")
# -output: Output directory for generated code (default: "./generated")
# -ollama: Ollama endpoint URL (default: "http://localhost:11434")
# -model: LLM model to use (default: "codellama:7b")
```

### Testing
```bash
# Run all tests
make test

# Run tests with verbose output
go test -v ./...

# Test specific package
go test -v ./internal/orchestrator
```

### Prerequisites
```bash
# Start Ollama service
ollama serve

# Pull required model
ollama pull codellama:7b
```

## Architecture

### Core Components

**Orchestrator** (`internal/orchestrator/`)
- Central control flow managing the fixed pipeline of code generation tasks
- Executes tasks sequentially: Models → Handlers → Repository → Tests
- Enforces safety limits (timeouts, retries, output size)
- Validates generated code using Go toolchain

**LLM Integration** (`internal/llm/`)
- Simple HTTP client for Ollama API
- No streaming - waits for complete responses
- Includes health check to verify Ollama availability

**Storage** (`internal/storage/`)
- Embedded SQLite for task tracking and generated file storage
- Schema embedded in binary using `//go:embed`
- Tracks task status, inputs, outputs, and errors

**Code Generation** (`internal/generator/`)
- Templates and logic for generating Go code
- Validation through `go fmt`, `go vet`, `go build`

**Prompt Templates** (`prompts/`)
- Fixed prompts for each generation phase
- Designed for consistency with local LLMs

### Task Pipeline

The system follows a fixed pipeline:
1. **generate_models**: Create data structures (Todo, TodoList)
2. **generate_handlers**: Create REST API endpoints
3. **generate_repository**: Create SQLite storage layer
4. **generate_tests**: Create unit tests
5. **validation**: Verify code compiles and passes basic checks

### Generated Output Structure
```
generated/
├── cmd/server/main.go          # Entry point for generated API
├── internal/
│   ├── models/todo.go          # Data models
│   ├── handlers/todo_handler.go # HTTP handlers
│   └── repository/todo_repo.go  # Database layer
├── tests/                       # Unit tests
├── go.mod                       # Go module file
└── README.md                    # Documentation
```

## Key Design Decisions

- **Single Binary**: Everything compiles to one ~30MB executable
- **Embedded Resources**: SQL schemas and prompts embedded using `//go:embed`
- **No External Dependencies**: Only requires Ollama service at runtime
- **Fixed Pipeline**: Deterministic task sequence, no dynamic graphs
- **Fail Fast**: No complex retry logic - errors stop execution immediately
- **Local Only**: Uses Ollama exclusively, no cloud API calls
- **Simple HTTP**: No streaming, websockets, or complex protocols

## Development Guidelines

- Keep implementations simple - avoid premature abstractions
- Use Go standard library wherever possible
- Validate all generated code before marking tasks complete
- Use context for proper timeout and cancellation handling
- Write clear progress messages to stdout for monitoring
- Maintain task status in SQLite for persistence and debugging

## Success Metrics

- Generates working Todo REST API
- Generated code compiles without errors
- Tests achieve >70% coverage
- Completes in under 30 minutes
- Uses $0 in API costs (local Ollama only)
- Binary size under 50MB

## Best Practices

### Code Quality Standards

#### Before Committing Code
Always run these commands before committing any Go code:
```bash
# Format all Go files
go fmt ./...

# Check for common mistakes
go vet ./...

# Ensure dependencies are correct
go mod tidy

# Run all tests
go test ./...

# Build to ensure compilation
go build ./...
```

#### Linting Rules
- **Never ignore linter warnings** - They often catch real bugs
- **Fix unused variables immediately** - They clutter code and may indicate logic errors
- **Always format code with gofmt** - Consistent formatting improves readability
- **Run go vet before commits** - Catches suspicious constructs

#### Editor Configuration
Configure your editor to:
- Auto-format Go files on save using gofmt
- Show linter warnings inline
- Run go vet automatically
- Highlight unused variables

#### Common Linting Issues and Fixes
1. **Unused variables**: Remove or use them (common in test code)
2. **Formatting issues**: Run `gofmt -w <file>` or `go fmt ./...`
3. **Missing error checks**: Always check returned errors
4. **Inefficient type conversions**: Use proper type assertions
5. **Unreachable code**: Remove dead code paths

#### Testing Standards
- Always ensure tests compile: `go test -c ./...`
- Fix test compilation errors immediately
- Remove unused test helpers and variables
- Keep test code as clean as production code

#### CI/CD Integration
For continuous integration:
```bash
# Makefile targets for CI
make fmt    # Format check
make vet    # Run go vet
make lint   # Run all linters
make test   # Run tests
make build  # Ensure compilation
```
