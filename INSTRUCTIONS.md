# Claude Code Instructions: Build Overnight LLM Development PoC

## Project Overview
Build a single Go binary that autonomously generates a complete REST API with tests overnight using local LLMs via Ollama. This is a MINIMAL proof of concept - simplicity over features.

## Core Requirements
- **Language**: Go 1.21+
- **Database**: Embedded SQLite (no external DB)
- **LLM**: Ollama API client (local models only)
- **Output**: Single compiled binary ~30MB
- **External Runtime Deps**: Only Ollama service running locally
- **Human Readable Documentation**: Describe how to launch the application the README, and extensive comments to make the code as human readable as possible, and a GUIDE.md document explaining code conventions and design decisions used.

## Project Structure
```
overnight-llm-poc/
├── cmd/
│   └── generator/
│       └── main.go              # Entry point, CLI handling
├── internal/
│   ├── orchestrator/
│   │   ├── orchestrator.go      # Main orchestration logic
│   │   └── orchestrator_test.go
│   ├── llm/
│   │   ├── ollama.go           # Ollama API client
│   │   └── ollama_test.go
│   ├── generator/
│   │   ├── code_generator.go   # Code generation logic
│   │   ├── test_generator.go   # Test generation logic
│   │   └── templates.go        # Embedded templates
│   ├── storage/
│   │   ├── storage.go          # SQLite task storage
│   │   └── schema.sql          # Embedded schema
│   └── validator/
│       ├── validator.go        # Code validation (go fmt, vet, build)
│       └── validator_test.go
├── prompts/
│   ├── generate_models.txt     # Prompt templates
│   ├── generate_handlers.txt
│   ├── generate_repository.txt
│   └── generate_tests.txt
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

## Implementation Instructions

### Step 1: Core Types and Interfaces

Create the fundamental types first in `internal/orchestrator/orchestrator.go`:

```go
package orchestrator

import (
    "context"
    "database/sql"
    "time"
)

type Orchestrator struct {
    llm      LLMProvider
    storage  *sql.DB
    workDir  string
    limits   SafetyLimits
}

type LLMProvider interface {
    Complete(ctx context.Context, prompt string) (string, error)
}

type Task struct {
    ID        string
    Type      TaskType
    Input     string
    Output    string
    Status    TaskStatus
    CreatedAt time.Time
    UpdatedAt time.Time
    Error     string
}

type TaskType string

const (
    TaskGenerateModels     TaskType = "generate_models"
    TaskGenerateHandlers   TaskType = "generate_handlers"
    TaskGenerateRepository TaskType = "generate_repository"
    TaskGenerateTests      TaskType = "generate_tests"
)

type TaskStatus string

const (
    StatusPending  TaskStatus = "pending"
    StatusRunning  TaskStatus = "running"
    StatusComplete TaskStatus = "complete"
    StatusFailed   TaskStatus = "failed"
)

type SafetyLimits struct {
    MaxRetries    int
    MaxRuntime    time.Duration
    MaxOutputSize int  // bytes
}
```

### Step 2: Ollama Client Implementation

Create `internal/llm/ollama.go` with a SIMPLE HTTP client:

```go
package llm

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

type OllamaClient struct {
    endpoint string  // "http://localhost:11434"
    model    string  // "codellama:7b"
    client   *http.Client
}

func NewOllamaClient(endpoint, model string) *OllamaClient {
    return &OllamaClient{
        endpoint: endpoint,
        model:    model,
        client: &http.Client{
            Timeout: 30 * time.Second,
        },
    }
}

func (o *OllamaClient) Complete(ctx context.Context, prompt string) (string, error) {
    // Call /api/generate endpoint
    // No streaming - wait for complete response
    // Return just the response text
    
    payload := map[string]interface{}{
        "model":  o.model,
        "prompt": prompt,
        "stream": false,
        "options": map[string]interface{}{
            "temperature": 0.2,  // Low for code generation
            "top_p":       0.9,
        },
    }
    
    // Implementation here...
}

// Add a health check
func (o *OllamaClient) HealthCheck(ctx context.Context) error {
    // Call /api/tags to verify Ollama is running and model exists
}
```

### Step 3: Embedded Resources

Create `internal/storage/schema.sql`:
```sql
CREATE TABLE IF NOT EXISTS tasks (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,
    input TEXT,
    output TEXT,
    status TEXT NOT NULL,
    error TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS files_generated (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id TEXT,
    file_path TEXT NOT NULL,
    content TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (task_id) REFERENCES tasks(id)
);
```

Embed it in `internal/storage/storage.go`:
```go
package storage

import (
    "database/sql"
    _ "embed"
    _ "github.com/mattn/go-sqlite3"
)

//go:embed schema.sql
var schemaSQL string

func InitDB(dbPath string) (*sql.DB, error) {
    db, err := sql.Open("sqlite3", dbPath)
    if err != nil {
        return nil, err
    }
    
    if _, err := db.Exec(schemaSQL); err != nil {
        return nil, err
    }
    
    return db, nil
}
```

### Step 4: Prompt Templates

Create simple, clear prompts in `prompts/` directory.

`prompts/generate_models.txt`:
```
You are a Go code generator following these STRICT rules:
1. Use ONLY Go standard library
2. Add validation methods to all types
3. Use explicit types, no interface{}
4. Include comments

Generate a Go model for a Todo item with these requirements:
- Fields: ID (int64), Title (string), Description (string), Done (bool), CreatedAt, UpdatedAt (time.Time)
- Add a Validate() method that ensures Title is not empty and under 200 characters
- Add a ToDo struct and a TodoList type (slice of Todo)

Output ONLY the Go code, no explanations.
```

### Step 5: Main Orchestration Logic

In `internal/orchestrator/orchestrator.go`, add the main pipeline:

```go
func (o *Orchestrator) GenerateTodoAPI(ctx context.Context, projectName string) error {
    startTime := time.Now()
    
    // Check safety limits
    ctx, cancel := context.WithTimeout(ctx, o.limits.MaxRuntime)
    defer cancel()
    
    // Fixed pipeline of tasks
    tasks := []Task{
        {ID: "task-001", Type: TaskGenerateModels, Input: "Todo with CRUD"},
        {ID: "task-002", Type: TaskGenerateHandlers, Input: "REST endpoints"},
        {ID: "task-003", Type: TaskGenerateRepository, Input: "SQLite storage"},
        {ID: "task-004", Type: TaskGenerateTests, Input: "Unit tests"},
    }
    
    for _, task := range tasks {
        if err := o.executeTask(ctx, task); err != nil {
            o.logError(task.ID, err)
            return fmt.Errorf("task %s failed: %w", task.ID, err)
        }
        
        // Simple progress indicator
        fmt.Printf("✓ Completed: %s\n", task.Type)
    }
    
    // Run validation
    if err := o.validateGeneratedCode(); err != nil {
        return fmt.Errorf("validation failed: %w", err)
    }
    
    fmt.Printf("\n✅ Successfully generated API in %v\n", time.Since(startTime))
    return nil
}

func (o *Orchestrator) executeTask(ctx context.Context, task Task) error {
    // Update status to running
    o.updateTaskStatus(task.ID, StatusRunning)
    
    // Load prompt template
    prompt := o.loadPrompt(task.Type)
    
    // Call LLM
    response, err := o.llm.Complete(ctx, prompt)
    if err != nil {
        o.updateTaskStatus(task.ID, StatusFailed)
        return err
    }
    
    // Save output
    if err := o.saveOutput(task, response); err != nil {
        return err
    }
    
    o.updateTaskStatus(task.ID, StatusComplete)
    return nil
}
```

### Step 6: CLI Entry Point

Create `cmd/generator/main.go`:

```go
package main

import (
    "context"
    "flag"
    "fmt"
    "log"
    "os"
    
    "overnight-llm-poc/internal/orchestrator"
    "overnight-llm-poc/internal/llm"
    "overnight-llm-poc/internal/storage"
)

func main() {
    var (
        prompt      = flag.String("prompt", "REST API for todo list", "What to generate")
        output      = flag.String("output", "./generated", "Output directory")
        ollamaHost  = flag.String("ollama", "http://localhost:11434", "Ollama endpoint")
        model       = flag.String("model", "codellama:7b", "Model to use")
    )
    flag.Parse()
    
    // Initialize components
    db, err := storage.InitDB("./poc.db")
    if err != nil {
        log.Fatal("Failed to init database:", err)
    }
    defer db.Close()
    
    ollama := llm.NewOllamaClient(*ollamaHost, *model)
    
    // Health check
    if err := ollama.HealthCheck(context.Background()); err != nil {
        fmt.Println("❌ Ollama not running or model not found")
        fmt.Printf("   Start Ollama: ollama serve\n")
        fmt.Printf("   Pull model:  ollama pull %s\n", *model)
        os.Exit(1)
    }
    
    // Create orchestrator
    orch := orchestrator.New(ollama, db, *output)
    
    // Run generation
    fmt.Printf("🚀 Starting generation: %s\n", *prompt)
    fmt.Printf("📁 Output directory: %s\n", *output)
    fmt.Printf("🤖 Using model: %s\n\n", *model)
    
    ctx := context.Background()
    if err := orch.GenerateTodoAPI(ctx, *prompt); err != nil {
        log.Fatal("Generation failed:", err)
    }
    
    // Show summary
    orch.PrintSummary()
}
```

### Step 7: Makefile for Building

Create `Makefile`:
```makefile
BINARY_NAME=overnight-llm
VERSION=0.1.0

.PHONY: build
build:
	CGO_ENABLED=1 go build -o $(BINARY_NAME) \
		-ldflags="-s -w -X main.Version=$(VERSION)" \
		./cmd/generator

.PHONY: build-all
build-all:
	# Mac ARM64
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 go build -o dist/$(BINARY_NAME)-mac-arm64 ./cmd/generator
	# Linux AMD64
	GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -o dist/$(BINARY_NAME)-linux-amd64 ./cmd/generator

.PHONY: test
test:
	go test -v ./...

.PHONY: run
run: build
	./$(BINARY_NAME) -output ./demo

.PHONY: clean
clean:
	rm -f $(BINARY_NAME)
	rm -rf dist/
	rm -rf generated/
	rm -f poc.db
```

## Critical Implementation Rules

### DO:
- ✅ Keep everything SIMPLE - no abstractions until needed
- ✅ Fail fast - if something breaks, stop and report
- ✅ Use embedded resources (SQL schema, prompts)
- ✅ Write clear progress messages to stdout
- ✅ Create a status.json file for monitoring
- ✅ Validate generated code (go fmt, go vet, go build)
- ✅ Use context for timeouts and cancellation
- ✅ Test with real Ollama calls

### DON'T:
- ❌ Add external dependencies beyond sqlite3 driver
- ❌ Create complex abstractions or interfaces
- ❌ Implement retry logic (just fail fast for PoC)
- ❌ Stream LLM responses (wait for complete response)
- ❌ Build a web UI (CLI only)
- ❌ Support multiple LLM providers (Ollama only)
- ❌ Create dynamic task graphs (fixed pipeline)

## Testing Instructions

1. **Unit Tests**: Mock the Ollama client for testing
2. **Integration Test**: Real Ollama call with small prompt
3. **End-to-End Test**: Generate actual code and validate it compiles

## File Output Structure

The generator should create this structure in the output directory:
```
generated/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── models/
│   │   └── todo.go
│   ├── handlers/
│   │   └── todo_handler.go
│   └── repository/
│       └── todo_repo.go
├── tests/
│   └── todo_handler_test.go
├── go.mod
├── go.sum
└── README.md
```

## Success Criteria

The PoC is successful if:
1. It generates a working Todo REST API
2. The generated code compiles without errors
3. Tests pass with >70% coverage
4. It completes in under 30 minutes
5. It uses $0 in API costs (local Ollama only)
6. The binary is under 50MB

## Example Usage

```bash
# Build the binary
make build

# Run generation
./overnight-llm -output ./my-api

# Test generated code
cd my-api
go test ./...
go run cmd/server/main.go
```

## Next Steps After PoC Works

Once this basic version works, we can add:
1. Multiple agents working in parallel
2. Dynamic task decomposition
3. Cloud LLM fallback for complex tasks
4. Web UI for monitoring
5. Git integration for commits

But NONE of these are needed for the initial proof of concept.

---

**IMPORTANT**: Start with the simplest possible implementation that demonstrates the core value: autonomous overnight code generation. Every feature beyond the minimum should be deferred to v2.