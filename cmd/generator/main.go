package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"gorchestrator-poc/internal/llm"
	"gorchestrator-poc/internal/orchestrator"
	"gorchestrator-poc/internal/storage"
	"gorchestrator-poc/internal/validator"
)

// Version information (set via ldflags during build)
var Version = "dev"

func main() {
	// Define command-line flags for configuration
	var (
		prompt       = flag.String("prompt", "REST API for todo list", "Description of what to generate")
		output       = flag.String("output", "./generated", "Output directory for generated code")
		ollamaHost   = flag.String("ollama", "http://localhost:11434", "Ollama API endpoint")
		model        = flag.String("model", "codellama:7b", "LLM model to use for generation")
		dbPath       = flag.String("db", "./poc.db", "SQLite database path")
		skipValidate = flag.Bool("skip-validation", false, "Skip code validation after generation")
		version      = flag.Bool("version", false, "Show version information")
		help         = flag.Bool("help", false, "Show help message")
	)

	flag.Parse()

	// Handle version flag
	if *version {
		fmt.Printf("overnight-llm-poc version %s\n", Version)
		os.Exit(0)
	}

	// Handle help flag
	if *help {
		printHelp()
		os.Exit(0)
	}

	// Print startup banner
	printBanner()

	// Initialize database with embedded schema
	fmt.Println("📦 Initializing database...")
	db, err := storage.InitDB(*dbPath)
	if err != nil {
		log.Fatal("❌ Failed to initialize database:", err)
	}
	defer db.Close()

	// Create Ollama client for LLM interactions
	fmt.Printf("🤖 Connecting to Ollama at %s...\n", *ollamaHost)
	ollamaClient := llm.NewOllamaClient(*ollamaHost, *model)

	// Perform health check to ensure Ollama is running and model is available
	fmt.Printf("🔍 Checking model availability (%s)...\n", *model)
	ctx := context.Background()
	if err := ollamaClient.HealthCheck(ctx); err != nil {
		fmt.Println("❌ Ollama health check failed:", err)
		fmt.Println("\n📋 Prerequisites:")
		fmt.Println("  1. Start Ollama service:")
		fmt.Printf("     ollama serve\n")
		fmt.Println("  2. Pull the required model:")
		fmt.Printf("     ollama pull %s\n", *model)
		fmt.Println("\n💡 Tip: For faster generation, try smaller models like:")
		fmt.Println("     ollama pull codellama:7b")
		fmt.Println("     ollama pull deepseek-coder:1.3b")
		os.Exit(1)
	}
	fmt.Println("✅ Ollama is ready!")

	// Create orchestrator to manage the generation pipeline
	orch := orchestrator.New(ollamaClient, db, *output)

	// Start the generation process
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Printf("🚀 STARTING CODE GENERATION\n")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("📝 Task:        %s\n", *prompt)
	fmt.Printf("📁 Output:      %s\n", *output)
	fmt.Printf("🤖 Model:       %s\n", *model)
	fmt.Printf("💾 Database:    %s\n", *dbPath)
	fmt.Println(strings.Repeat("=", 60) + "\n")

	// Run the main generation pipeline
	if err := orch.GenerateTodoAPI(ctx, *prompt); err != nil {
		fmt.Printf("\n❌ Generation failed: %v\n", err)

		// Still show summary even on failure
		orch.PrintSummary()

		// Provide troubleshooting tips
		fmt.Println("\n💡 Troubleshooting tips:")
		fmt.Println("  - Check Ollama is running: curl http://localhost:11434/api/tags")
		fmt.Println("  - Verify model is downloaded: ollama list")
		fmt.Println("  - Try a smaller model if running out of memory")
		fmt.Println("  - Check the generated files in:", *output)
		fmt.Println("  - Review poc.db for task details")

		os.Exit(1)
	}

	// Run validation if not skipped
	if !*skipValidate {
		fmt.Println("\n" + strings.Repeat("=", 60))
		fmt.Println("🔍 VALIDATING GENERATED CODE")
		fmt.Println(strings.Repeat("=", 60))

		val := validator.NewValidator(*output)

		// Check Go installation first
		if err := val.CheckGoInstallation(); err != nil {
			fmt.Printf("⚠️  Go not found: %v\n", err)
			fmt.Println("   Skipping validation (Go required for validation)")
		} else {
			// Run all validation checks
			results := val.ValidateAll(ctx)
			validator.PrintResults(results)

			// Optionally format the code
			fmt.Println("\n📝 Auto-formatting generated code...")
			if err := val.FormatCode(ctx); err != nil {
				fmt.Printf("⚠️  Failed to format code: %v\n", err)
			} else {
				fmt.Println("✅ Code formatted successfully")
			}
		}
	}

	// Print final summary
	orch.PrintSummary()

	// Print next steps for the user
	printNextSteps(*output)
}

// printBanner displays the application banner
func printBanner() {
	banner := `
╔══════════════════════════════════════════════════════════╗
║           OVERNIGHT LLM CODE GENERATOR POC               ║
║                                                          ║
║  Autonomous code generation using local LLMs via Ollama  ║
╚══════════════════════════════════════════════════════════╝
`
	fmt.Println(banner)
}

// printHelp displays usage information
func printHelp() {
	fmt.Println("Overnight LLM Code Generator - Proof of Concept")
	fmt.Println("\nUsage:")
	fmt.Println("  overnight-llm [flags]")
	fmt.Println("\nFlags:")
	flag.PrintDefaults()
	fmt.Println("\nExamples:")
	fmt.Println("  # Generate with default settings")
	fmt.Println("  ./overnight-llm")
	fmt.Println()
	fmt.Println("  # Use a different model")
	fmt.Println("  ./overnight-llm -model llama2:13b")
	fmt.Println()
	fmt.Println("  # Custom output directory")
	fmt.Println("  ./overnight-llm -output ./my-api")
	fmt.Println()
	fmt.Println("  # Skip validation for faster generation")
	fmt.Println("  ./overnight-llm -skip-validation")
	fmt.Println("\nPrerequisites:")
	fmt.Println("  1. Install and start Ollama: https://ollama.ai")
	fmt.Println("  2. Pull a code generation model: ollama pull codellama:7b")
	fmt.Println("  3. Ensure Go 1.21+ is installed for validation")
}

// printNextSteps provides guidance on what to do with generated code
func printNextSteps(outputDir string) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("🎯 NEXT STEPS")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("\n1. Navigate to generated code:\n")
	fmt.Printf("   cd %s\n\n", outputDir)

	fmt.Println("2. Install dependencies:")
	fmt.Println("   go mod download")
	fmt.Println()

	fmt.Println("3. Run tests:")
	fmt.Println("   go test -v ./...")
	fmt.Println()

	fmt.Println("4. Start the server:")
	fmt.Println("   go run cmd/server/main.go")
	fmt.Println()

	fmt.Println("5. Test the API:")
	fmt.Println("   curl http://localhost:8080/todos")
	fmt.Println()

	fmt.Println("📚 Documentation:")
	fmt.Printf("   - README: %s/README.md\n", outputDir)
	fmt.Printf("   - Status: %s/status.json\n", outputDir)
	fmt.Println()

	fmt.Println("💡 Tips:")
	fmt.Println("   - Review generated code before deployment")
	fmt.Println("   - The code may need adjustments based on your requirements")
	fmt.Println("   - Check status.json for generation details")
	fmt.Println("   - Database poc.db contains full generation history")
}
