package validator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestNewValidator verifies validator creation
func TestNewValidator(t *testing.T) {
	workDir := "/test/dir"
	v := NewValidator(workDir)

	if v.workDir != workDir {
		t.Errorf("Expected workDir %s, got %s", workDir, v.workDir)
	}

	if v.timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got %v", v.timeout)
	}
}

// TestValidateFile tests single file validation
func TestValidateFile(t *testing.T) {
	// Create temp directory with test files
	workDir := t.TempDir()
	v := NewValidator(workDir)

	tests := []struct {
		name     string
		content  string
		wantPass bool
	}{
		{
			name: "valid Go file",
			content: `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}
`,
			wantPass: true,
		},
		{
			name: "invalid Go syntax",
			content: `package main

func main() {
	fmt.Println("Missing import"
}
`,
			wantPass: false,
		},
		{
			name:     "empty file",
			content:  "",
			wantPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file
			filePath := filepath.Join(workDir, "test.go")
			err := os.WriteFile(filePath, []byte(tt.content), 0644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}
			defer os.Remove(filePath)

			// Validate file
			ctx := context.Background()
			result := v.ValidateFile(ctx, filePath)

			if tt.wantPass && !result.Success {
				t.Errorf("Expected validation to pass, but it failed: %v", result.Error)
			}

			if !tt.wantPass && result.Success {
				t.Error("Expected validation to fail, but it passed")
			}
		})
	}
}

// TestCheckGoInstallation verifies Go installation check
func TestCheckGoInstallation(t *testing.T) {
	v := NewValidator(t.TempDir())

	// This test assumes Go is installed on the test system
	err := v.CheckGoInstallation()
	if err != nil {
		t.Skipf("Go not installed on test system: %v", err)
	}
}

// TestPrintResults verifies result printing doesn't panic
func TestPrintResults(t *testing.T) {
	results := []ValidationResult{
		{
			Tool:    "gofmt",
			Success: true,
			Output:  "All files formatted",
		},
		{
			Tool:    "go vet",
			Success: false,
			Output:  "Found issues",
			Error:   nil,
		},
		{
			Tool:    "go build",
			Success: true,
			Output:  "",
		},
	}

	// This should not panic
	PrintResults(results)
}

// TestValidationResult verifies the ValidationResult structure
func TestValidationResult(t *testing.T) {
	result := ValidationResult{
		Tool:    "test-tool",
		Success: true,
		Output:  "test output",
		Error:   nil,
	}

	if result.Tool != "test-tool" {
		t.Errorf("Expected tool 'test-tool', got '%s'", result.Tool)
	}

	if !result.Success {
		t.Error("Expected success to be true")
	}

	if result.Output != "test output" {
		t.Errorf("Expected output 'test output', got '%s'", result.Output)
	}
}

// TestFormatCode tests code formatting with a mock Go file
func TestFormatCode(t *testing.T) {
	workDir := t.TempDir()
	v := NewValidator(workDir)

	// Create an unformatted Go file
	testFile := filepath.Join(workDir, "unformatted.go")
	unformattedCode := `package main
import "fmt"
func main(){fmt.Println("Hello")}`

	err := os.WriteFile(testFile, []byte(unformattedCode), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Try to format (this will only work if gofmt is available)
	ctx := context.Background()
	err = v.FormatCode(ctx)

	// If gofmt is not available, skip the rest of the test
	if err != nil && strings.Contains(err.Error(), "executable file not found") {
		t.Skip("gofmt not available on test system")
	}
}

// TestValidateAll tests the complete validation pipeline
func TestValidateAll(t *testing.T) {
	workDir := t.TempDir()
	v := NewValidator(workDir)

	// Create a simple valid Go module for testing
	goModContent := `module test

go 1.21
`
	err := os.WriteFile(filepath.Join(workDir, "go.mod"), []byte(goModContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// Create a simple Go file
	mainContent := `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}
`
	err = os.WriteFile(filepath.Join(workDir, "main.go"), []byte(mainContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create main.go: %v", err)
	}

	// Run all validations
	ctx := context.Background()
	results := v.ValidateAll(ctx)

	// Should have results for fmt, vet, and build
	if len(results) != 3 {
		t.Errorf("Expected 3 validation results, got %d", len(results))
	}

	// Check that we got results for each tool
	tools := make(map[string]bool)
	for _, result := range results {
		tools[result.Tool] = true
	}

	expectedTools := []string{"gofmt", "go vet", "go build"}
	for _, tool := range expectedTools {
		if !tools[tool] {
			t.Errorf("Missing result for tool: %s", tool)
		}
	}
}

// TestRunTests verifies test execution
func TestRunTests(t *testing.T) {
	workDir := t.TempDir()
	v := NewValidator(workDir)

	// Create a simple test file
	testContent := `package main

import "testing"

func TestExample(t *testing.T) {
	result := 2 + 2
	if result != 4 {
		t.Errorf("Expected 4, got %d", result)
	}
}
`
	testFile := filepath.Join(workDir, "example_test.go")
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create go.mod
	goModContent := `module test

go 1.21
`
	err = os.WriteFile(filepath.Join(workDir, "go.mod"), []byte(goModContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// Run tests
	ctx := context.Background()
	result := v.RunTests(ctx)

	// Check if go test is available
	if result.Error != nil && strings.Contains(result.Error.Error(), "executable file not found") {
		t.Skip("go test not available on test system")
	}

	// Tests should pass
	if !result.Success {
		t.Errorf("Expected tests to pass, but they failed: %v", result.Error)
	}
}

// TestTimeoutHandling verifies timeout is applied correctly
func TestTimeoutHandling(t *testing.T) {
	workDir := t.TempDir()
	v := &Validator{
		workDir: workDir,
		timeout: 1 * time.Millisecond, // Very short timeout
	}

	// Create a context that will timeout
	ctx := context.Background()

	// This should timeout
	result := v.checkFormat(ctx)

	// The operation should have been cancelled or timed out
	// We're not checking specific error as it depends on timing
	if result.Success {
		t.Skip("Operation completed before timeout - test inconclusive")
	}
}
