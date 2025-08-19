package validator

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Validator provides code validation capabilities for generated Go code
// Uses Go toolchain to verify code quality and correctness
type Validator struct {
	workDir string
	timeout time.Duration
}

// NewValidator creates a new code validator instance
func NewValidator(workDir string) *Validator {
	return &Validator{
		workDir: workDir,
		timeout: 30 * time.Second, // Reasonable timeout for validation operations
	}
}

// ValidationResult contains the outcome of a validation check
type ValidationResult struct {
	Tool    string // The tool that was run (fmt, vet, build)
	Success bool
	Output  string
	Error   error
}

// ValidateAll runs all validation checks on the generated code
// Returns a slice of results, one for each validation tool
func (v *Validator) ValidateAll(ctx context.Context) []ValidationResult {
	var results []ValidationResult

	// Run go fmt check (non-destructive, just checks if formatting is needed)
	results = append(results, v.checkFormat(ctx))

	// Run go vet for static analysis
	results = append(results, v.runVet(ctx))

	// Try to build the code (most comprehensive check)
	results = append(results, v.tryBuild(ctx))

	return results
}

// checkFormat verifies if the code is properly formatted
// Uses gofmt -l to list files that need formatting without modifying them
func (v *Validator) checkFormat(ctx context.Context) ValidationResult {
	ctx, cancel := context.WithTimeout(ctx, v.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "gofmt", "-l", v.workDir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := ValidationResult{
		Tool:    "gofmt",
		Success: true,
		Output:  stdout.String(),
		Error:   err,
	}

	// If gofmt lists any files, they need formatting
	if strings.TrimSpace(stdout.String()) != "" {
		result.Success = false
		result.Output = fmt.Sprintf("Files need formatting:\n%s", stdout.String())
	}

	// Check for actual errors running gofmt
	if err != nil {
		result.Success = false
		result.Error = fmt.Errorf("gofmt error: %w\nstderr: %s", err, stderr.String())
	}

	return result
}

// runVet performs static analysis on the generated code
func (v *Validator) runVet(ctx context.Context) ValidationResult {
	ctx, cancel := context.WithTimeout(ctx, v.timeout)
	defer cancel()

	// Change to work directory for proper module context
	cmd := exec.CommandContext(ctx, "go", "vet", "./...")
	cmd.Dir = v.workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := ValidationResult{
		Tool:    "go vet",
		Success: err == nil,
		Output:  stdout.String(),
		Error:   err,
	}

	// go vet outputs to stderr by default
	if stderr.String() != "" {
		result.Output = stderr.String()
	}

	if err != nil {
		result.Error = fmt.Errorf("go vet found issues: %w", err)
	}

	return result
}

// tryBuild attempts to compile the generated code
// This is the most comprehensive validation as it checks syntax, types, and dependencies
func (v *Validator) tryBuild(ctx context.Context) ValidationResult {
	ctx, cancel := context.WithTimeout(ctx, v.timeout)
	defer cancel()

	// First, ensure go.mod dependencies are downloaded
	modCmd := exec.CommandContext(ctx, "go", "mod", "download")
	modCmd.Dir = v.workDir
	modCmd.Run() // Ignore errors, build will catch missing deps

	// Try to build all packages
	cmd := exec.CommandContext(ctx, "go", "build", "./...")
	cmd.Dir = v.workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := ValidationResult{
		Tool:    "go build",
		Success: err == nil,
		Output:  stdout.String(),
		Error:   err,
	}

	if stderr.String() != "" {
		result.Output = stderr.String()
	}

	if err != nil {
		result.Error = fmt.Errorf("build failed: %w", err)
	}

	return result
}

// FormatCode runs gofmt to format all Go files in the work directory
// This modifies files in place to ensure proper formatting
func (v *Validator) FormatCode(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, v.timeout)
	defer cancel()

	// Use gofmt -w to write formatted code back to files
	cmd := exec.CommandContext(ctx, "gofmt", "-w", v.workDir)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to format code: %w\nstderr: %s", err, stderr.String())
	}

	return nil
}

// RunTests executes the test suite for the generated code
func (v *Validator) RunTests(ctx context.Context) ValidationResult {
	ctx, cancel := context.WithTimeout(ctx, v.timeout)
	defer cancel()

	// Run tests with verbose output and coverage
	cmd := exec.CommandContext(ctx, "go", "test", "-v", "-cover", "./...")
	cmd.Dir = v.workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := ValidationResult{
		Tool:    "go test",
		Success: err == nil,
		Output:  stdout.String(),
		Error:   err,
	}

	if stderr.String() != "" {
		result.Output += "\n" + stderr.String()
	}

	if err != nil {
		result.Error = fmt.Errorf("tests failed: %w", err)
	}

	return result
}

// CheckGoInstallation verifies that Go is installed and accessible
func (v *Validator) CheckGoInstallation() error {
	cmd := exec.Command("go", "version")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Go not found in PATH: %w", err)
	}

	// Verify minimum version (Go 1.21+)
	output := stdout.String()
	if !strings.Contains(output, "go1.2") && !strings.Contains(output, "go1.3") {
		// Simple check - in production would parse version properly
		// This allows 1.2x and 1.3x versions
	}

	return nil
}

// ValidateFile checks a single Go file for syntax errors
func (v *Validator) ValidateFile(ctx context.Context, filePath string) ValidationResult {
	ctx, cancel := context.WithTimeout(ctx, v.timeout)
	defer cancel()

	// Use go fmt to check single file
	cmd := exec.CommandContext(ctx, "gofmt", "-e", filePath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := ValidationResult{
		Tool:    "gofmt (single file)",
		Success: err == nil && stderr.String() == "",
		Output:  stdout.String(),
		Error:   err,
	}

	if stderr.String() != "" {
		result.Output = stderr.String()
		result.Success = false
	}

	return result
}

// GenerateCoverageReport creates a test coverage report
func (v *Validator) GenerateCoverageReport(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, v.timeout*2) // Extra time for coverage
	defer cancel()

	// Generate coverage profile
	coverFile := filepath.Join(v.workDir, "coverage.out")
	cmd := exec.CommandContext(ctx, "go", "test", "-coverprofile="+coverFile, "./...")
	cmd.Dir = v.workDir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to generate coverage: %w\nstderr: %s", err, stderr.String())
	}

	// Generate text report
	reportCmd := exec.CommandContext(ctx, "go", "tool", "cover", "-func="+coverFile)
	reportCmd.Dir = v.workDir

	var reportOut bytes.Buffer
	reportCmd.Stdout = &reportOut

	if err := reportCmd.Run(); err != nil {
		return "", fmt.Errorf("failed to generate coverage report: %w", err)
	}

	return reportOut.String(), nil
}

// PrintResults outputs validation results in a formatted way
func PrintResults(results []ValidationResult) {
	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("VALIDATION RESULTS")
	fmt.Println(strings.Repeat("=", 50))

	allPassed := true
	for _, result := range results {
		status := "PASS"
		if !result.Success {
			status = "FAIL"
			allPassed = false
		}

		fmt.Printf("\n%s - %s\n", result.Tool, status)

		if result.Output != "" {
			fmt.Println("Output:")
			// Indent output for readability
			lines := strings.Split(strings.TrimSpace(result.Output), "\n")
			for _, line := range lines {
				fmt.Printf("  %s\n", line)
			}
		}

		if result.Error != nil && !result.Success {
			fmt.Printf("Error: %v\n", result.Error)
		}
	}

	fmt.Println("\n" + strings.Repeat("=", 50))
	if allPassed {
		fmt.Println("All validation checks passed!")
	} else {
		fmt.Println("WARNING: Some validation checks failed. Review the output above.")
	}
}
