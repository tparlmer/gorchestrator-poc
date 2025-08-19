package orchestrator

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorchestrator-poc/internal/storage"
)

// LLMProvider defines the interface for LLM interactions
// Implementations must provide code completion capabilities
type LLMProvider interface {
	Complete(ctx context.Context, prompt string) (string, error)
	HealthCheck(ctx context.Context) error
}

// TaskType represents different types of code generation tasks
type TaskType string

const (
	TaskGenerateModels     TaskType = "generate_models"
	TaskGenerateHandlers   TaskType = "generate_handlers"
	TaskGenerateRepository TaskType = "generate_repository"
	TaskGenerateTests      TaskType = "generate_tests"
)

// TaskStatus represents the current state of a task
type TaskStatus string

const (
	StatusPending  TaskStatus = "pending"
	StatusRunning  TaskStatus = "running"
	StatusComplete TaskStatus = "complete"
	StatusFailed   TaskStatus = "failed"
)

// Task represents a single code generation task
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

// SafetyLimits defines operational boundaries for safe execution
type SafetyLimits struct {
	MaxRetries    int           // Maximum retry attempts per task
	MaxRuntime    time.Duration // Maximum total runtime
	MaxOutputSize int           // Maximum size of generated output in bytes
}

// Orchestrator manages the code generation pipeline
// Coordinates LLM calls, stores results, and ensures safety limits
type Orchestrator struct {
	llm         LLMProvider
	storage     *storage.Storage
	workDir     string
	limits      SafetyLimits
	promptsPath string
	startTime   time.Time
}

// GenerationStats tracks statistics for the generation session
type GenerationStats struct {
	TotalTasks      int
	CompletedTasks  int
	FailedTasks     int
	TotalDuration   time.Duration
	FilesGenerated  int
	TotalOutputSize int64
}

// New creates a new Orchestrator instance with default safety limits
func New(llm LLMProvider, db *sql.DB, workDir string) *Orchestrator {
	return &Orchestrator{
		llm:         llm,
		storage:     storage.NewStorage(db),
		workDir:     workDir,
		promptsPath: "prompts",
		limits: SafetyLimits{
			MaxRetries:    3,
			MaxRuntime:    30 * time.Minute,
			MaxOutputSize: 10 * 1024 * 1024, // 10MB
		},
	}
}

// GenerateTodoAPI runs the main code generation pipeline
// Executes a fixed sequence of tasks to generate a complete Todo REST API
func (o *Orchestrator) GenerateTodoAPI(ctx context.Context, projectName string) error {
	o.startTime = time.Now()

	// Apply global timeout for safety
	ctx, cancel := context.WithTimeout(ctx, o.limits.MaxRuntime)
	defer cancel()

	// Ensure output directory exists
	if err := os.MkdirAll(o.workDir, 0755); err != nil {
		return fmt.Errorf("failed to create work directory: %w", err)
	}

	// Generate unique run ID to avoid database conflicts
	runID := fmt.Sprintf("run_%d", time.Now().Unix())
	fmt.Printf("Run ID: %s\n\n", runID)

	// Define the fixed pipeline of tasks with unique IDs per run
	tasks := []Task{
		{ID: fmt.Sprintf("%s_task_001", runID), Type: TaskGenerateModels, Input: "Todo with CRUD", Status: StatusPending},
		{ID: fmt.Sprintf("%s_task_002", runID), Type: TaskGenerateHandlers, Input: "REST endpoints", Status: StatusPending},
		{ID: fmt.Sprintf("%s_task_003", runID), Type: TaskGenerateRepository, Input: "SQLite storage", Status: StatusPending},
		{ID: fmt.Sprintf("%s_task_004", runID), Type: TaskGenerateTests, Input: "Unit tests", Status: StatusPending},
	}

	// Store tasks in database
	for _, task := range tasks {
		if err := o.storage.CreateTask(storage.Task{
			ID:     task.ID,
			Type:   string(task.Type),
			Input:  task.Input,
			Status: string(task.Status),
		}); err != nil {
			return fmt.Errorf("failed to create task %s: %w", task.ID, err)
		}
	}

	// Execute each task in sequence
	for _, task := range tasks {
		select {
		case <-ctx.Done():
			return fmt.Errorf("generation timeout exceeded")
		default:
			if err := o.executeTask(ctx, task); err != nil {
				o.logError(task.ID, err)
				return fmt.Errorf("task %s (%s) failed: %w", task.ID, task.Type, err)
			}
			fmt.Printf("[DONE] Completed: %s\n", task.Type)
		}
	}

	// Generate server main.go entry point
	if err := o.generateServerMain(); err != nil {
		return fmt.Errorf("failed to generate server main: %w", err)
	}
	fmt.Printf("[DONE] Completed: server main.go\n")

	// Generate go.mod for the output project
	if err := o.generateGoMod(); err != nil {
		return fmt.Errorf("failed to generate go.mod: %w", err)
	}

	// Generate README for the output project
	if err := o.generateREADME(); err != nil {
		return fmt.Errorf("failed to generate README: %w", err)
	}

	// Run validation on generated code
	if err := o.validateGeneratedCode(ctx); err != nil {
		fmt.Printf("WARNING: Validation issues: %v\n", err)
		// Don't fail on validation errors for PoC
	}

	// Write status file for monitoring
	if err := o.writeStatusFile(); err != nil {
		fmt.Printf("Failed to write status file: %v\n", err)
	}

	fmt.Printf("\n[SUCCESS] Generated API in %v\n", time.Since(o.startTime))
	return nil
}

// cleanLLMOutput removes markdown code blocks and extra text from LLM output
// This is critical because LLMs often wrap code in ```go blocks despite instructions
func cleanLLMOutput(raw string) string {
	// Trim whitespace
	raw = strings.TrimSpace(raw)
	
	// Check if the output starts with markdown code block
	if strings.HasPrefix(raw, "```") {
		lines := strings.Split(raw, "\n")
		var cleaned []string
		inCode := false
		
		for _, line := range lines {
			// Check for code block markers
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				inCode = !inCode
				continue // Skip the markdown markers
			}
			
			// Only include lines that are inside code blocks
			if inCode {
				cleaned = append(cleaned, line)
			}
		}
		
		// Join the cleaned lines
		result := strings.Join(cleaned, "\n")
		
		// Trim any trailing whitespace
		return strings.TrimSpace(result)
	}
	
	// If no markdown blocks found, check for inline backticks at start/end
	if strings.HasPrefix(raw, "`") && strings.HasSuffix(raw, "`") {
		raw = strings.TrimPrefix(raw, "`")
		raw = strings.TrimSuffix(raw, "`")
	}
	
	return raw
}

// executeTask runs a single code generation task
func (o *Orchestrator) executeTask(ctx context.Context, task Task) error {
	// Update task status to running
	if err := o.storage.UpdateTaskStatus(task.ID, string(StatusRunning)); err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}

	// Load the appropriate prompt template
	prompt, err := o.loadPrompt(task.Type)
	if err != nil {
		return fmt.Errorf("failed to load prompt: %w", err)
	}

	// Call LLM for code generation
	fmt.Printf("  → Generating %s...\n", task.Type)
	response, err := o.llm.Complete(ctx, prompt)
	if err != nil {
		o.storage.UpdateTaskStatus(task.ID, string(StatusFailed))
		return fmt.Errorf("LLM generation failed: %w", err)
	}

	// Clean the LLM output to remove markdown formatting
	cleaned := cleanLLMOutput(response)
	
	// Check output size limit
	if len(cleaned) > o.limits.MaxOutputSize {
		return fmt.Errorf("output exceeds size limit: %d > %d", len(cleaned), o.limits.MaxOutputSize)
	}

	// Show preview of cleaned output
	preview := strings.Split(cleaned, "\n")
	if len(preview) > 3 {
		fmt.Printf("    Preview: %s\n", preview[0])
	}

	// Save the cleaned output
	if err := o.saveOutput(task, cleaned); err != nil {
		return fmt.Errorf("failed to save output: %w", err)
	}

	// Update task as complete
	if err := o.storage.UpdateTaskStatus(task.ID, string(StatusComplete)); err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}

	if err := o.storage.UpdateTaskOutput(task.ID, cleaned); err != nil {
		return fmt.Errorf("failed to save task output: %w", err)
	}

	return nil
}

// loadPrompt reads the prompt template for a given task type
func (o *Orchestrator) loadPrompt(taskType TaskType) (string, error) {
	// Map task types to prompt files
	promptFile := ""
	switch taskType {
	case TaskGenerateModels:
		promptFile = "generate_models.txt"
	case TaskGenerateHandlers:
		promptFile = "generate_handlers.txt"
	case TaskGenerateRepository:
		promptFile = "generate_repository.txt"
	case TaskGenerateTests:
		promptFile = "generate_tests.txt"
	default:
		return "", fmt.Errorf("unknown task type: %s", taskType)
	}

	promptPath := filepath.Join(o.promptsPath, promptFile)
	content, err := os.ReadFile(promptPath)
	if err != nil {
		return "", fmt.Errorf("failed to read prompt file %s: %w", promptPath, err)
	}

	return string(content), nil
}

// saveOutput writes generated code to the appropriate file
func (o *Orchestrator) saveOutput(task Task, content string) error {
	// Determine output file path based on task type
	var outputPath string
	switch task.Type {
	case TaskGenerateModels:
		outputPath = filepath.Join(o.workDir, "internal", "models", "todo.go")
	case TaskGenerateHandlers:
		outputPath = filepath.Join(o.workDir, "internal", "handlers", "todo_handler.go")
	case TaskGenerateRepository:
		outputPath = filepath.Join(o.workDir, "internal", "repository", "todo_repo.go")
	case TaskGenerateTests:
		outputPath = filepath.Join(o.workDir, "tests", "todo_handler_test.go")
	default:
		return fmt.Errorf("unknown task type for output: %s", task.Type)
	}

	// Create directory structure
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Write file
	if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", outputPath, err)
	}

	// Store in database
	if err := o.storage.SaveGeneratedFile(task.ID, outputPath, content); err != nil {
		return fmt.Errorf("failed to save file record: %w", err)
	}

	return nil
}

// generateGoMod creates a go.mod file for the generated project
func (o *Orchestrator) generateGoMod() error {
	content := `module todo-api

go 1.21

require (
	github.com/mattn/go-sqlite3 v1.14.17
)
`
	outputPath := filepath.Join(o.workDir, "go.mod")
	return os.WriteFile(outputPath, []byte(content), 0644)
}

// generateREADME creates a README file for the generated project
func (o *Orchestrator) generateREADME() error {
	content := `# Generated Todo API

This REST API was automatically generated by the Overnight LLM PoC.

## Running the API

` + "```bash" + `
# Install dependencies
go mod download

# Run tests
go test ./...

# Start the server
go run cmd/server/main.go
` + "```" + `

## API Endpoints

- GET    /todos     - List all todos
- GET    /todos/:id - Get a specific todo
- POST   /todos     - Create a new todo
- PUT    /todos/:id - Update a todo
- DELETE /todos/:id - Delete a todo

## Generated Files

- internal/models/todo.go - Data models
- internal/handlers/todo_handler.go - HTTP handlers
- internal/repository/todo_repo.go - Database layer
- tests/todo_handler_test.go - Unit tests

Generated at: ` + time.Now().Format(time.RFC3339) + `
`
	outputPath := filepath.Join(o.workDir, "README.md")
	return os.WriteFile(outputPath, []byte(content), 0644)
}

// generateServerMain creates the main.go entry point for the generated API
func (o *Orchestrator) generateServerMain() error {
	content := `package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	fmt.Println("Starting Todo API server on :8080")

	// Initialize database
	// Setup routes
	// Start server

	log.Fatal(http.ListenAndServe(":8080", nil))
}
`
	outputPath := filepath.Join(o.workDir, "cmd", "server", "main.go")
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(outputPath, []byte(content), 0644)
}

// validateGeneratedCode runs basic validation on generated Go code
func (o *Orchestrator) validateGeneratedCode(ctx context.Context) error {
	// For the PoC, we'll do basic file existence checks
	// In a real implementation, would run go fmt, go vet, go build

	requiredFiles := []string{
		filepath.Join(o.workDir, "internal", "models", "todo.go"),
		filepath.Join(o.workDir, "internal", "handlers", "todo_handler.go"),
		filepath.Join(o.workDir, "internal", "repository", "todo_repo.go"),
		filepath.Join(o.workDir, "go.mod"),
	}

	for _, file := range requiredFiles {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			return fmt.Errorf("required file missing: %s", file)
		}
	}

	return nil
}

// logError records an error for a specific task
func (o *Orchestrator) logError(taskID string, err error) {
	if err := o.storage.UpdateTaskError(taskID, err.Error()); err != nil {
		fmt.Printf("Failed to log error for task %s: %v\n", taskID, err)
	}
}

// writeStatusFile creates a JSON status file for monitoring
func (o *Orchestrator) writeStatusFile() error {
	tasks, err := o.storage.GetAllTasks()
	if err != nil {
		return err
	}

	stats := GenerationStats{
		TotalTasks:    len(tasks),
		TotalDuration: time.Since(o.startTime),
	}

	for _, task := range tasks {
		if task.Status == string(StatusComplete) {
			stats.CompletedTasks++
		} else if task.Status == string(StatusFailed) {
			stats.FailedTasks++
		}
	}

	// Count generated files
	files, _ := filepath.Glob(filepath.Join(o.workDir, "**", "*.go"))
	stats.FilesGenerated = len(files)

	// Marshal to JSON
	statusData := map[string]interface{}{
		"stats":     stats,
		"tasks":     tasks,
		"completed": stats.CompletedTasks == stats.TotalTasks,
		"timestamp": time.Now().Format(time.RFC3339),
		"duration":  stats.TotalDuration.String(),
		"workDir":   o.workDir,
	}

	jsonData, err := json.MarshalIndent(statusData, "", "  ")
	if err != nil {
		return err
	}

	statusPath := filepath.Join(o.workDir, "status.json")
	return os.WriteFile(statusPath, jsonData, 0644)
}

// PrintSummary outputs a summary of the generation session
func (o *Orchestrator) PrintSummary() {
	tasks, err := o.storage.GetAllTasks()
	if err != nil {
		fmt.Printf("Failed to load tasks: %v\n", err)
		return
	}

	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("GENERATION SUMMARY")
	fmt.Println(strings.Repeat("=", 50))

	completed := 0
	failed := 0
	for _, task := range tasks {
		status := "⏳"
		if task.Status == string(StatusComplete) {
			status = "[DONE]"
			completed++
		} else if task.Status == string(StatusFailed) {
			status = "[FAIL]"
			failed++
		}
		fmt.Printf("%s %s - %s\n", status, task.Type, task.Status)
	}

	fmt.Printf("\nTotal Tasks: %d\n", len(tasks))
	fmt.Printf("Completed:   %d\n", completed)
	fmt.Printf("Failed:      %d\n", failed)
	fmt.Printf("Duration:    %v\n", time.Since(o.startTime))
	fmt.Printf("Output Dir:  %s\n", o.workDir)

	if completed == len(tasks) {
		fmt.Println("\nAll tasks completed successfully!")
		fmt.Printf("\nNext steps:\n")
		fmt.Printf("  cd %s\n", o.workDir)
		fmt.Printf("  go mod download\n")
		fmt.Printf("  go test ./...\n")
		fmt.Printf("  go run cmd/server/main.go\n")
	}
}
