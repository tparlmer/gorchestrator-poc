# Overnight LLM Code Generator - Proof of Concept

A minimal Go application that autonomously generates complete REST APIs with tests using local LLMs via Ollama. This proof of concept demonstrates how AI can generate working code overnight with zero API costs.

## 🚀 Quick Start

```bash
# 1. Install Ollama (if not already installed)
# Visit: https://ollama.ai

# 2. Start Ollama and pull a model
ollama serve
ollama pull codellama:7b

# 3. Build and run the generator
make build
./overnight-llm -output ./my-api

# 4. Test the generated API
cd my-api
go test ./...
go run cmd/server/main.go
```

## 📋 Prerequisites

- **Go 1.21+** - Required for building and validating generated code
- **Ollama** - Local LLM runtime (https://ollama.ai)
- **SQLite3** - Embedded database (included via Go driver)
- **~2GB RAM** - For running small models
- **~5GB disk space** - For model storage

## 🏗️ Architecture

The system follows a simple, fixed pipeline architecture:

```
┌──────────────┐     ┌─────────────┐     ┌──────────────┐
│   Prompts    │────▶│ Ollama LLM  │────▶│  Generated   │
│  Templates   │     │  (Local)    │     │    Code      │
└──────────────┘     └─────────────┘     └──────────────┘
                            │
                     ┌──────▼──────┐
                     │   SQLite    │
                     │  Task Store │
                     └─────────────┘
```

### Core Components

- **Orchestrator**: Manages the fixed pipeline of code generation tasks
- **LLM Client**: Simple HTTP client for Ollama API (no streaming)
- **Storage**: SQLite database for tracking tasks and outputs
- **Validator**: Uses Go toolchain to validate generated code
- **Prompts**: Fixed templates for each generation phase

## 🎯 Features

- ✅ **Zero API Costs** - Uses only local Ollama models
- ✅ **Single Binary** - Everything compiles to ~30MB executable
- ✅ **Embedded Resources** - SQL schema and prompts included
- ✅ **Code Validation** - Automatic formatting and validation
- ✅ **Progress Tracking** - Real-time status updates
- ✅ **Safety Limits** - Timeouts and output size restrictions

## 📦 Installation

### From Source

```bash
# Clone the repository
git clone <repository-url>
cd gorchestrator-poc

# Install dependencies
make deps

# Build the binary
make build

# Or install to GOPATH/bin
make install
```

### Pre-built Binaries

```bash
# Build for multiple platforms
make build-all

# Binaries will be in dist/
ls dist/
# overnight-llm-mac-arm64
# overnight-llm-mac-amd64  
# overnight-llm-linux-amd64
```

## 🔧 Usage

### Basic Usage

```bash
# Generate with default settings
./overnight-llm

# Specify output directory
./overnight-llm -output ./my-api

# Use a different model
./overnight-llm -model llama2:13b

# Skip validation for faster generation
./overnight-llm -skip-validation
```

### Command-line Options

| Flag | Default | Description |
|------|---------|-------------|
| `-output` | `./generated` | Output directory for generated code |
| `-model` | `codellama:7b` | Ollama model to use |
| `-ollama` | `http://localhost:11434` | Ollama API endpoint |
| `-prompt` | `REST API for todo list` | What to generate |
| `-db` | `./poc.db` | SQLite database path |
| `-skip-validation` | `false` | Skip code validation |
| `-version` | - | Show version information |
| `-help` | - | Show help message |

## 🤖 Supported Models

Recommended models for code generation:

| Model | Size | Speed | Quality |
|-------|------|-------|---------|
| `codellama:7b` | 3.8GB | Fast | Good |
| `deepseek-coder:1.3b` | 776MB | Very Fast | Acceptable |
| `codellama:13b` | 7.4GB | Medium | Better |
| `llama2:13b` | 7.4GB | Medium | Good |

## 📁 Generated Output Structure

The generator creates a complete Go project:

```
generated/
├── cmd/
│   └── server/
│       └── main.go              # API server entry point
├── internal/
│   ├── models/
│   │   └── todo.go             # Data models with validation
│   ├── handlers/
│   │   └── todo_handler.go     # HTTP request handlers
│   └── repository/
│       └── todo_repo.go        # Database operations
├── tests/
│   └── todo_handler_test.go   # Unit tests
├── go.mod                      # Go module file
├── README.md                   # Generated documentation
└── status.json                 # Generation statistics
```

## 🧪 Testing

### Unit Tests

```bash
# Run all tests
make test

# Generate coverage report
make test-coverage
# Opens coverage.html in browser
```

### Integration Testing

```bash
# Test the generated API
cd generated/
go test -v ./...
go run cmd/server/main.go

# In another terminal
curl http://localhost:8080/todos
curl -X POST http://localhost:8080/todos \
  -H "Content-Type: application/json" \
  -d '{"title":"Test Todo","description":"Testing the API"}'
```

## 🛠️ Development

### Project Structure

```
gorchestrator-poc/
├── cmd/generator/          # CLI entry point
├── internal/
│   ├── orchestrator/      # Pipeline management
│   ├── llm/              # Ollama client
│   ├── storage/          # SQLite operations
│   └── validator/        # Code validation
├── prompts/              # Generation templates
├── Makefile             # Build automation
└── README.md            # This file
```

### Development Workflow

```bash
# Auto-rebuild on file changes (requires entr)
make dev

# Format code
make fmt

# Run linter
make lint

# Run go vet
make vet
```

## 📊 Success Metrics

The PoC is considered successful when:

- ✅ Generates a working Todo REST API
- ✅ Generated code compiles without errors
- ✅ Tests achieve >70% coverage
- ✅ Completes in under 30 minutes
- ✅ Uses $0 in API costs
- ✅ Binary size under 50MB

## 🐛 Troubleshooting

### Ollama Not Running

```bash
# Check if Ollama is running
make check-ollama

# Start Ollama
ollama serve

# Pull required models
make setup-models
```

### Model Not Found

```bash
# List available models
ollama list

# Pull a model
ollama pull codellama:7b
```

### Build Errors

```bash
# Clean and rebuild
make clean
make deps
make build
```

### Generation Failures

- Check Ollama is running: `curl http://localhost:11434/api/tags`
- Verify model exists: `ollama list`
- Try a smaller model if out of memory
- Check `poc.db` for detailed error messages
- Review `generated/status.json` for task details

## 📈 Performance

Typical generation times on M1 MacBook Pro:

| Model | Generation Time | Memory Usage |
|-------|----------------|--------------|
| `codellama:7b` | ~5-10 minutes | ~4GB |
| `deepseek-coder:1.3b` | ~2-5 minutes | ~1GB |
| `codellama:13b` | ~10-20 minutes | ~8GB |

## 🔒 Safety & Limitations

- **Fixed Pipeline**: No dynamic task graphs (by design)
- **Local Only**: No cloud API support (cost control)
- **Output Limits**: 10MB max per task (configurable)
- **Timeout**: 30-minute maximum runtime
- **No Retry Logic**: Fails fast on errors (simplicity)

## 🚀 Future Enhancements

After the PoC proves successful, potential v2 features:

- [ ] Multiple agents working in parallel
- [ ] Dynamic task decomposition
- [ ] Cloud LLM fallback for complex tasks
- [ ] Web UI for monitoring progress
- [ ] Git integration for automatic commits
- [ ] Support for multiple programming languages
- [ ] Custom prompt templates
- [ ] Incremental code updates

## 📝 License

This is a proof of concept for demonstration purposes.

## 🤝 Contributing

This is a minimal PoC focused on demonstrating core value. Please keep contributions aligned with the simplicity principle.

## 📞 Support

For issues or questions:
1. Check the troubleshooting section
2. Review `poc.db` for task errors
3. Examine `generated/status.json` for details
4. Try with a smaller model or simpler prompt

---

**Remember**: This is a MINIMAL proof of concept. Every feature beyond autonomous overnight code generation should be deferred to v2.