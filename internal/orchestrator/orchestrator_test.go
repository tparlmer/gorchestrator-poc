package orchestrator

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gorchestrator-poc/internal/storage"
)

// mockLLMProvider implements LLMProvider for testing
type mockLLMProvider struct {
	completeFunc    func(ctx context.Context, prompt string) (string, error)
	healthCheckFunc func(ctx context.Context) error
	callCount       int
}

func (m *mockLLMProvider) Complete(ctx context.Context, prompt string) (string, error) {
	m.callCount++
	if m.completeFunc != nil {
		return m.completeFunc(ctx, prompt)
	}
	return "mock generated code", nil
}

func (m *mockLLMProvider) HealthCheck(ctx context.Context) error {
	if m.healthCheckFunc != nil {
		return m.healthCheckFunc(ctx)
	}
	return nil
}

// TestNew verifies orchestrator creation
func TestNew(t *testing.T) {
	// Create temp database
	db, cleanup := createTestDB(t)
	defer cleanup()

	mockLLM := &mockLLMProvider{}
	workDir := t.TempDir()

	orch := New(mockLLM, db, workDir)

	if orch == nil {
		t.Fatal("Expected orchestrator, got nil")
	}

	if orch.llm != mockLLM {
		t.Error("LLM provider not set correctly")
	}

	if orch.workDir != workDir {
		t.Errorf("Expected workDir %s, got %s", workDir, orch.workDir)
	}

	// Check default limits
	if orch.limits.MaxRuntime != 30*time.Minute {
		t.Errorf("Expected MaxRuntime 30m, got %v", orch.limits.MaxRuntime)
	}

	if orch.limits.MaxOutputSize != 10*1024*1024 {
		t.Errorf("Expected MaxOutputSize 10MB, got %d", orch.limits.MaxOutputSize)
	}
}

// TestLoadPrompt verifies prompt loading for different task types
func TestLoadPrompt(t *testing.T) {
	// Create temp directory with prompt files
	tempDir := t.TempDir()
	promptsDir := filepath.Join(tempDir, "prompts")
	os.MkdirAll(promptsDir, 0755)

	// Create test prompt files
	prompts := map[TaskType]string{
		TaskGenerateModels:     "generate_models.txt",
		TaskGenerateHandlers:   "generate_handlers.txt",
		TaskGenerateRepository: "generate_repository.txt",
		TaskGenerateTests:      "generate_tests.txt",
	}

	for taskType, filename := range prompts {
		content := "Test prompt for " + string(taskType)
		err := os.WriteFile(filepath.Join(promptsDir, filename), []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create prompt file: %v", err)
		}
	}

	// Create orchestrator with custom prompts path
	orch := &Orchestrator{
		promptsPath: promptsDir,
	}

	// Test loading each prompt
	for taskType := range prompts {
		prompt, err := orch.loadPrompt(taskType)
		if err != nil {
			t.Errorf("Failed to load prompt for %s: %v", taskType, err)
		}

		expected := "Test prompt for " + string(taskType)
		if prompt != expected {
			t.Errorf("Expected prompt '%s', got '%s'", expected, prompt)
		}
	}

	// Test unknown task type
	_, err := orch.loadPrompt(TaskType("unknown"))
	if err == nil {
		t.Error("Expected error for unknown task type, got nil")
	}
}

// TestSaveOutput verifies file saving functionality
func TestSaveOutput(t *testing.T) {
	db, cleanup := createTestDB(t)
	defer cleanup()

	workDir := t.TempDir()
	orch := &Orchestrator{
		storage: storage.NewStorage(db),
		workDir: workDir,
	}

	tests := []struct {
		name     string
		task     Task
		content  string
		wantPath string
	}{
		{
			name:     "models output",
			task:     Task{ID: "1", Type: TaskGenerateModels},
			content:  "package models\n\ntype Todo struct{}",
			wantPath: filepath.Join(workDir, "internal", "models", "todo.go"),
		},
		{
			name:     "handlers output",
			task:     Task{ID: "2", Type: TaskGenerateHandlers},
			content:  "package handlers\n\nfunc ListTodos() {}",
			wantPath: filepath.Join(workDir, "internal", "handlers", "todo_handler.go"),
		},
		{
			name:     "repository output",
			task:     Task{ID: "3", Type: TaskGenerateRepository},
			content:  "package repository\n\ntype TodoRepo struct{}",
			wantPath: filepath.Join(workDir, "internal", "repository", "todo_repo.go"),
		},
		{
			name:     "tests output",
			task:     Task{ID: "4", Type: TaskGenerateTests},
			content:  "package tests\n\nfunc TestTodo(t *testing.T) {}",
			wantPath: filepath.Join(workDir, "tests", "todo_handler_test.go"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// First create the task in the database (required for foreign key)
			err := orch.storage.CreateTask(storage.Task{
				ID:     tt.task.ID,
				Type:   string(tt.task.Type),
				Status: "pending",
			})
			if err != nil {
				t.Fatalf("Failed to create task: %v", err)
			}

			// Save output
			err = orch.saveOutput(tt.task, tt.content)
			if err != nil {
				t.Fatalf("Failed to save output: %v", err)
			}

			// Verify file exists and contains correct content
			data, err := os.ReadFile(tt.wantPath)
			if err != nil {
				t.Fatalf("Failed to read saved file: %v", err)
			}

			if string(data) != tt.content {
				t.Errorf("Expected content '%s', got '%s'", tt.content, string(data))
			}
		})
	}
}

// TestGenerateGoMod verifies go.mod generation
func TestGenerateGoMod(t *testing.T) {
	workDir := t.TempDir()
	orch := &Orchestrator{workDir: workDir}

	err := orch.generateGoMod()
	if err != nil {
		t.Fatalf("Failed to generate go.mod: %v", err)
	}

	// Check file exists
	modPath := filepath.Join(workDir, "go.mod")
	data, err := os.ReadFile(modPath)
	if err != nil {
		t.Fatalf("Failed to read go.mod: %v", err)
	}

	content := string(data)

	// Verify content
	if !strings.Contains(content, "module todo-api") {
		t.Error("go.mod should contain module declaration")
	}

	if !strings.Contains(content, "go 1.21") {
		t.Error("go.mod should specify Go version")
	}

	if !strings.Contains(content, "github.com/mattn/go-sqlite3") {
		t.Error("go.mod should include sqlite3 dependency")
	}
}

// TestValidateGeneratedCode verifies validation logic
func TestValidateGeneratedCode(t *testing.T) {
	workDir := t.TempDir()
	orch := &Orchestrator{workDir: workDir}

	// Test with missing files
	ctx := context.Background()
	err := orch.validateGeneratedCode(ctx)
	if err == nil {
		t.Error("Expected error for missing files, got nil")
	}

	// Create required files
	requiredFiles := []string{
		filepath.Join(workDir, "internal", "models", "todo.go"),
		filepath.Join(workDir, "internal", "handlers", "todo_handler.go"),
		filepath.Join(workDir, "internal", "repository", "todo_repo.go"),
		filepath.Join(workDir, "go.mod"),
	}

	for _, file := range requiredFiles {
		dir := filepath.Dir(file)
		os.MkdirAll(dir, 0755)
		os.WriteFile(file, []byte("test content"), 0644)
	}

	// Now validation should pass
	err = orch.validateGeneratedCode(ctx)
	if err != nil {
		t.Errorf("Unexpected validation error: %v", err)
	}
}

// TestExecuteTask verifies task execution flow
func TestExecuteTask(t *testing.T) {
	// Set up test environment
	db, cleanup := createTestDB(t)
	defer cleanup()

	workDir := t.TempDir()
	promptsDir := filepath.Join(workDir, "prompts")
	os.MkdirAll(promptsDir, 0755)

	// Create a test prompt file
	promptFile := filepath.Join(promptsDir, "generate_models.txt")
	os.WriteFile(promptFile, []byte("test prompt"), 0644)

	// Create mock LLM
	mockLLM := &mockLLMProvider{
		completeFunc: func(ctx context.Context, prompt string) (string, error) {
			return "generated code", nil
		},
	}

	// Create orchestrator
	orch := &Orchestrator{
		llm:         mockLLM,
		storage:     storage.NewStorage(db),
		workDir:     workDir,
		promptsPath: promptsDir,
		limits: SafetyLimits{
			MaxOutputSize: 1024 * 1024,
		},
	}

	// Create and store task
	task := Task{
		ID:     "test-001",
		Type:   TaskGenerateModels,
		Input:  "test input",
		Status: StatusPending,
	}

	err := orch.storage.CreateTask(storage.Task{
		ID:     task.ID,
		Type:   string(task.Type),
		Input:  task.Input,
		Status: string(task.Status),
	})
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	// Execute task
	ctx := context.Background()
	err = orch.executeTask(ctx, task)
	if err != nil {
		t.Fatalf("Failed to execute task: %v", err)
	}

	// Verify task was updated
	storedTask, err := orch.storage.GetTask(task.ID)
	if err != nil {
		t.Fatalf("Failed to get task: %v", err)
	}

	if storedTask.Status != string(StatusComplete) {
		t.Errorf("Expected status %s, got %s", StatusComplete, storedTask.Status)
	}

	if storedTask.Output != "generated code" {
		t.Errorf("Expected output 'generated code', got '%s'", storedTask.Output)
	}

	// Verify file was created
	expectedPath := filepath.Join(workDir, "internal", "models", "todo.go")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Error("Expected file was not created")
	}
}

// TestSafetyLimits verifies safety limit enforcement
func TestSafetyLimits(t *testing.T) {
	db, cleanup := createTestDB(t)
	defer cleanup()

	workDir := t.TempDir()
	promptsDir := filepath.Join(workDir, "prompts")
	os.MkdirAll(promptsDir, 0755)

	// Create test prompt
	os.WriteFile(filepath.Join(promptsDir, "generate_models.txt"), []byte("test"), 0644)

	// Test output size limit
	largeOutput := make([]byte, 100) // Small limit for testing
	for i := range largeOutput {
		largeOutput[i] = 'a'
	}

	mockLLM := &mockLLMProvider{
		completeFunc: func(ctx context.Context, prompt string) (string, error) {
			return string(largeOutput), nil
		},
	}

	orch := &Orchestrator{
		llm:         mockLLM,
		storage:     storage.NewStorage(db),
		workDir:     workDir,
		promptsPath: promptsDir,
		limits: SafetyLimits{
			MaxOutputSize: 50, // Smaller than output
		},
	}

	task := Task{
		ID:   "test-001",
		Type: TaskGenerateModels,
	}

	orch.storage.CreateTask(storage.Task{
		ID:     task.ID,
		Type:   string(task.Type),
		Status: string(StatusPending),
	})

	ctx := context.Background()
	err := orch.executeTask(ctx, task)
	if err == nil {
		t.Error("Expected error for output size limit, got nil")
	}

	if !strings.Contains(err.Error(), "exceeds size limit") {
		t.Errorf("Expected size limit error, got: %v", err)
	}
}

// Helper function to create test database
func createTestDB(t *testing.T) (*sql.DB, func()) {
	tempFile := filepath.Join(t.TempDir(), "test.db")
	db, err := storage.InitDB(tempFile)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.Remove(tempFile)
	}

	return db, cleanup
}
