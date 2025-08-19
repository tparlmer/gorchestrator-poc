# Iteration 1: Post-Mortem and Analysis

## Overview
Used llamacode 7b parameters for first iteration.

First implementation attempt of the Overnight LLM Code Generator PoC. The system successfully orchestrates LLM calls and generates files, but the generated code is non-functional due to critical output processing issues.

## What Went Well

### Architecture & Design
- **Clean separation of concerns**: Orchestrator, LLM client, Storage, and Validator are well-separated
- **Embedded resources**: SQL schema and prompts are embedded in the binary successfully
- **SQLite integration**: Task tracking and file storage works correctly
- **Fixed pipeline approach**: Simple and predictable execution flow
- **Safety limits**: Timeout and output size limits are properly enforced

### Code Quality
- **Comprehensive error handling**: All errors are wrapped with context
- **Good test coverage**: Tests exist for core components
- **Proper Go idioms**: Uses context, defer, and standard library effectively
- **Clear documentation**: README, GUIDE.md, and CLAUDE.md provide good guidance

### Infrastructure
- **Build system**: Makefile provides convenient targets
- **CLI interface**: Clean command-line interface with helpful flags
- **Health checks**: Verifies Ollama is running before starting
- **Status reporting**: Generates status.json for monitoring

## What Failed

### Critical Issues

#### 1. **Markdown Wrapping in Generated Code**
**Problem**: All generated Go files are wrapped in markdown code blocks (````go ... ````)
```go
// File saved as todo.go contains:
```
package models
// ... actual code ...
```
```

**Impact**: 100% of generated files are invalid Go code - they're interpreted as string literals

**Root Cause**:
- LLM naturally outputs code in markdown format (training bias)
- No output cleaning before saving files
- Direct save of raw LLM response

#### 2. **Missing Imports**
**Problem**: Generated code references packages without importing them
```go
func (tl TodoList) SortByCreatedAt() TodoList {
    sort.Slice(tl, func(i, j int) bool {  // 'sort' not imported
```

**Impact**: Even if markdown is removed, code won't compile

#### 3. **Undefined Variables**
**Problem**: Code references undefined error variables
```go
if t.Title == "" {
    return ErrEmptyTitle  // Not defined anywhere
}
```

**Impact**: Compilation failures even with correct syntax

#### 4. **No Entry Point Generated**
**Problem**: The `generateServerMain()` function exists but is never called
- No `cmd/server/main.go` file is created
- Users can't run the generated API

#### 5. **Test Command Failure**
**Issue**: Running `go test ./...` fails with "no Go files"
```bash
catatafish@Thomass-MacBook-Pro my-api % go test ./..
# ./..
no Go files in /Users/catatafish/repos/gorchestrator-poc
FAIL    ./.. [setup failed]
```

### Secondary Issues

#### 6. **Prompt Ineffectiveness**
- Instructions to not use markdown are ignored
- "Output ONLY the Go code" still results in explanatory text
- Prompts lack concrete examples of expected output

#### 7. **No Validation Pipeline**
- No syntax checking before saving
- No attempt to compile generated code
- No retry mechanism for failed generations

#### 8. **Limited Observability**
- Can't see what LLM actually returned during generation
- No indication when output needs cleaning
- No metrics on generation quality

## Root Cause Analysis

### The Core Problem: Output Format Mismatch

| Component | Expects | Receives | Result |
|-----------|---------|----------|--------|
| Orchestrator | Raw Go code | Markdown-wrapped code | Saves invalid files |
| Go Compiler | Valid syntax | String literals (```) | Compilation failure |
| Test Runner | .go files | Comments | "No Go files" |

### Why LLMs Generate Markdown

1. **Training Data**: LLMs are trained on documentation, tutorials, and Q&A sites where code is always in markdown
2. **Instruction Following**: LLMs interpret "generate code" as "show code in readable format"
3. **Pattern Recognition**: Even with explicit instructions, the markdown pattern is deeply ingrained

## Model Selection Considerations

### Impact of Model Size on Markdown Wrapping

#### Why Larger Models Might Still Have the Issue
1. **Universal Training Bias**
   - All LLaMA/CodeLlama models (7B, 13B, 34B) trained on similar datasets
   - Code in training data is overwhelmingly in markdown format
   - Larger models may actually be BETTER at following this pattern

2. **Instruction Following Paradox**
   - Larger models have stronger pattern recognition
   - More parameters = more commitment to training patterns
   - May reinforce markdown output rather than prevent it

#### Potential Benefits of Larger Models
- **Better instruction comprehension** - May understand "raw code only" more literally
- **Fewer syntax errors** - Better understanding of language semantics
- **Complete implementations** - Less likely to have missing imports/functions
- **Response to correction** - More responsive to refined prompts

**Prediction**: 70% chance larger models still wrap in markdown due to training bias

### Code-Specific Model Alternatives

#### Models Designed for Code Generation

| Model | Size | Key Advantage | Ollama Available |
|-------|------|---------------|------------------|
| **DeepSeek Coder** | 1.3B-33B | Trained on raw code files, minimal markdown | ✅ Yes |
| **StarCoder2** | 3B-15B | Fill-in-the-middle training | ❌ No |
| **WizardCoder** | Various | Better instruction following | ❌ No |
| **Phind-CodeLlama** | 34B | Pure code focus, less explanatory | ❌ No |
| **Mistral** | 7B | Good code mode | ✅ Yes |

#### Why Code-Specific Models Perform Better

**Training Data Differences:**
- **General Models**: Documentation, tutorials, Q&A sites (code in markdown)
- **Code Models**: Raw repository files, commit diffs (code as files)

**Output Behavior:**
```bash
# General Model (CodeLlama) Output:
```python
def factorial(n):
    return 1 if n <= 1 else n * factorial(n-1)
```
Here's how this function works...

# Code Model (DeepSeek) Output:
def factorial(n):
    return 1 if n <= 1 else n * factorial(n-1)
```

### Recommended Testing Strategy

1. **Try DeepSeek Coder First**
   ```bash
   ollama pull deepseek-coder:6.7b
   ./overnight-llm -model deepseek-coder:6.7b -output ./my-api
   ```

2. **Compare Different Sizes**
   ```bash
   # Small but fast
   ./overnight-llm -model deepseek-coder:1.3b -output ./my-api-small
   
   # Larger for quality
   ./overnight-llm -model codellama:13b -output ./my-api-large
   ```

3. **Test Code vs Instruct Variants**
   ```bash
   # Code completion variant (less markdown)
   ollama pull codellama:7b-code
   
   # Instruction variant (more markdown)
   ollama pull codellama:7b-instruct
   ```

### Key Insight

The markdown wrapping issue is **model-agnostic but training-dependent**. Models trained primarily on raw code repositories (like DeepSeek Coder) are significantly less likely to wrap output in markdown. The solution isn't just a bigger model, but a model trained on the right type of data.

## Proposed Solutions

### Immediate Fixes (Priority 1)

#### 1. Output Cleaning Function
```go
func cleanLLMOutput(raw string) string {
    // Strip markdown code blocks
    raw = strings.TrimSpace(raw)

    // Handle ```go or ``` prefix
    if strings.HasPrefix(raw, "```") {
        lines := strings.Split(raw, "\n")
        var cleaned []string
        inCode := false

        for _, line := range lines {
            if strings.HasPrefix(strings.TrimSpace(line), "```") {
                inCode = !inCode
                continue
            }
            if inCode {
                cleaned = append(cleaned, line)
            }
        }

        result := strings.Join(cleaned, "\n")

        // Remove any trailing explanation after last ```
        if idx := strings.LastIndex(result, "```"); idx > 0 {
            result = result[:idx]
        }

        return strings.TrimSpace(result)
    }

    return raw
}
```

#### 2. Call generateServerMain()
Add to orchestrator pipeline:
```go
// After generating handlers
if err := o.generateServerMain(); err != nil {
    return fmt.Errorf("failed to generate server main: %w", err)
}
```

#### 3. Import Detection and Addition
```go
func addMissingImports(code string) (string, error) {
    fset := token.NewFileSet()
    file, err := parser.ParseFile(fset, "", code, parser.ParseComments)
    if err != nil {
        return "", err
    }

    // Add missing imports based on undefined identifiers
    imports := detectRequiredImports(file)
    return addImports(code, imports), nil
}
```

### Prompt Engineering Improvements (Priority 2)

#### 1. Stronger Instructions with Examples
```
CRITICAL FORMATTING RULES:
1. First character MUST be 'p' from 'package'
2. NO markdown formatting (no ```)
3. NO explanatory text before or after code

EXAMPLE OF WRONG OUTPUT:
```go
package main
```

EXAMPLE OF CORRECT OUTPUT:
package main

import "fmt"

func main() {
    fmt.Println("Hello")
}
```

#### 2. Few-Shot Prompting
Include examples of correctly formatted code in the prompt:
```
Here are examples of correctly formatted Go code:

EXAMPLE 1:
package models

import "time"

type User struct {
    ID int64
}

Now generate similar code for: [task description]
```

### Validation Pipeline (Priority 3)

#### 1. Syntax Validation
```go
func validateGeneratedCode(content string) error {
    // Check for markdown
    if strings.Contains(content, "```") {
        return fmt.Errorf("contains markdown formatting")
    }

    // Parse as Go code
    _, err := parser.ParseFile(token.NewFileSet(), "", content, 0)
    if err != nil {
        return fmt.Errorf("invalid Go syntax: %w", err)
    }

    // Check package declaration exists
    if !strings.HasPrefix(strings.TrimSpace(content), "package") {
        return fmt.Errorf("missing package declaration")
    }

    return nil
}
```

#### 2. Retry Mechanism
```go
func (o *Orchestrator) executeTaskWithRetry(ctx context.Context, task Task) error {
    maxRetries := 3

    for attempt := 1; attempt <= maxRetries; attempt++ {
        output, err := o.llm.Complete(ctx, prompt)
        if err != nil {
            return err
        }

        // Clean output
        cleaned := cleanLLMOutput(output)

        // Validate
        if err := validateGeneratedCode(cleaned); err != nil {
            if attempt < maxRetries {
                fmt.Printf("  WARNING: Validation failed (attempt %d/%d): %v\n", attempt, maxRetries, err)
                // Modify prompt for retry
                prompt = makeStrongerPrompt(prompt, err)
                continue
            }
            return err
        }

        // Success
        return o.saveOutput(task, cleaned)
    }

    return fmt.Errorf("failed after %d retries", maxRetries)
}
```

### Enhanced Observability (Priority 4)

#### 1. Output Preview
```go
func (o *Orchestrator) saveOutput(task Task, content string) error {
    // Show preview of generated code
    preview := strings.Split(content, "\n")
    if len(preview) > 5 {
        preview = preview[:5]
    }
    fmt.Printf("  Generated preview:\n")
    for _, line := range preview {
        fmt.Printf("     %s\n", line)
    }

    // Continue with saving...
}
```

#### 2. Debugging Mode
Add a `-debug` flag that:
- Saves raw LLM outputs to `.raw` files
- Shows token counts
- Displays validation results
- Logs retry attempts

## Metrics & Impact

### Current State
- **Success Rate**: 0% (no compilable code)
- **Files Generated**: 4/5 (missing main.go)
- **Tests Passing**: 0% (can't run)
- **Time Taken**: ~5-10 minutes
- **Manual Fixes Required**: 100% of files

### Expected After Fixes
- **Success Rate**: >80% (compilable code)
- **Files Generated**: 5/5 (includes main.go)
- **Tests Passing**: >70% (may need minor fixes)
- **Time Taken**: ~5-15 minutes (with retries)
- **Manual Fixes Required**: <20% of files

## Lessons Learned

1. **Never Trust LLM Output Format**: Always clean and validate
2. **Prompts Need Examples**: Abstract instructions aren't enough
3. **Validation is Critical**: Catch issues before saving
4. **Observability Matters**: Can't fix what you can't see
5. **Start Simple**: Even basic cleaning would make huge difference

## Next Steps

### Must Fix Now
1. Implement `cleanLLMOutput()` function
2. Add call to `generateServerMain()`
3. Validate before saving files

### Should Fix Soon
4. Add retry mechanism with better prompts
5. Implement import detection
6. Add output preview for debugging

### Consider for V2
7. Multi-agent validation system
8. Learning from failures
9. Automatic error recovery

## Conclusion

The first iteration successfully demonstrated the orchestration concept but failed on output processing. The primary issue - markdown wrapping - is straightforward to fix and would immediately improve success rates from 0% to functional. The architecture is sound; we just need better LLM output handling.

**Key Insight**: The gap between what LLMs naturally output and what compilers expect is the critical challenge in code generation systems. Bridging this gap with proper cleaning and validation is essential for success.
