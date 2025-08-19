# Iteration 2: Database Constraints and Rerun Issues

## Overview
Second run attempted with same CodeLlama 7B model. Failed immediately due to database unique constraint violation, revealing a critical design flaw in task ID management.

## What Happened

### The Failure
```
ERROR: Generation failed: failed to create task task-001: failed to create task: UNIQUE constraint failed: tasks.id
```

**Duration**: 1.4585ms (immediate failure)
**Result**: No new files generated
**Database State**: Still contains tasks from iteration 1

### Root Cause Analysis

#### 1. **Hardcoded Task IDs**
```go
// From orchestrator.go - Tasks have fixed IDs
tasks := []Task{
    {ID: "task-001", Type: TaskGenerateModels, ...},
    {ID: "task-002", Type: TaskGenerateHandlers, ...},
    {ID: "task-003", Type: TaskGenerateRepository, ...},
    {ID: "task-004", Type: TaskGenerateTests, ...},
}
```

**Problem**: Every run attempts to use the same task IDs
**Impact**: Cannot run the generator twice without manual database cleanup

#### 2. **Misleading Error Recovery**
Despite the fatal error, the output showed:
- "All tasks completed successfully!"
- "Total Tasks: 4, Completed: 4"
- Suggested next steps as if generation succeeded

**Why**: `PrintSummary()` is called in all cases and reads OLD tasks from the database:
```go
// Even after error, this runs:
orch.PrintSummary()  // Shows tasks from iteration 1
```

#### 3. **No Run Isolation**
- Database persists across runs
- No concept of "generation sessions"
- Cannot track multiple attempts separately

## Critical Issues Found

### Priority 1: Task ID Uniqueness
**Current State**: Hardcoded IDs prevent multiple runs
**Required Fix**: Generate unique IDs per run

### Priority 2: Error Handling Flow
**Current State**: Summary shows success even on failure
**Required Fix**: Conditional summary based on actual success

### Priority 3: Database Management
**Current State**: Manual cleanup required between runs
**Required Fix**: Auto-cleanup or session management

## Proposed Fixes for Iteration 3

### Fix 1: Unique Task IDs
```go
func (o *Orchestrator) GenerateTodoAPI(ctx context.Context, projectName string) error {
    // Generate unique run ID
    runID := fmt.Sprintf("run_%d", time.Now().Unix())
    
    // Create tasks with unique IDs
    tasks := []Task{
        {ID: fmt.Sprintf("%s_task_001", runID), Type: TaskGenerateModels, ...},
        {ID: fmt.Sprintf("%s_task_002", runID), Type: TaskGenerateHandlers, ...},
        {ID: fmt.Sprintf("%s_task_003", runID), Type: TaskGenerateRepository, ...},
        {ID: fmt.Sprintf("%s_task_004", runID), Type: TaskGenerateTests, ...},
    }
    // ...
}
```

### Fix 2: Conditional Summary
```go
func main() {
    // ...
    if err := orch.GenerateTodoAPI(ctx, *prompt); err != nil {
        fmt.Printf("\nERROR: Generation failed: %v\n", err)
        
        // Don't show success summary on failure
        orch.PrintFailureSummary()  // New method showing what went wrong
        
        // Troubleshooting tips...
        os.Exit(1)
    }
    
    // Only show success summary if actually successful
    orch.PrintSummary()
}
```

### Fix 3: Database Cleanup Options

#### Option A: Auto-cleanup old runs
```go
func (s *Storage) CleanupOldRuns(hoursOld int) error {
    query := `
        DELETE FROM tasks 
        WHERE created_at < datetime('now', '-' || ? || ' hours')
    `
    _, err := s.db.Exec(query, hoursOld)
    return err
}
```

#### Option B: Session-based isolation
```go
type GenerationSession struct {
    ID        string
    StartTime time.Time
    Status    string
}

// Link tasks to sessions
type Task struct {
    ID        string
    SessionID string  // New field
    // ...
}
```

#### Option C: CLI flag for cleanup
```bash
# Add flag to clean database
./overnight-llm --clean-db -output ./my-api

# Or separate command
./overnight-llm --cleanup-old-runs
```

## Workarounds for Current Version

### Manual Database Cleanup
```bash
# Remove database before each run
rm poc.db
./overnight-llm -output ./my-api

# Or use different database per run
./overnight-llm -db poc_run2.db -output ./my-api-run2
```

### SQLite Direct Cleanup
```bash
# Clean specific tasks
sqlite3 poc.db "DELETE FROM tasks WHERE id LIKE 'task-%';"
sqlite3 poc.db "DELETE FROM files_generated;"

# Or reset everything
sqlite3 poc.db "DELETE FROM tasks; DELETE FROM files_generated;"
```

## Testing Strategy for Iteration 3

### Test Multiple Runs
```bash
# First run
./overnight-llm -output ./test1

# Second run (should work without manual cleanup)
./overnight-llm -output ./test2

# Verify both runs tracked separately
sqlite3 poc.db "SELECT id, status FROM tasks ORDER BY created_at;"
```

### Test Error Recovery
```bash
# Simulate failure (kill Ollama mid-run)
./overnight-llm -output ./test-fail

# Verify error summary (not success summary)
# Should see failure details, not "All tasks completed"
```

## Additional Improvements Needed

### 1. Run Metadata
Track each generation attempt:
```go
type GenerationRun struct {
    ID          string
    StartTime   time.Time
    EndTime     time.Time
    Model       string
    OutputDir   string
    Success     bool
    TaskCount   int
    ErrorMsg    string
}
```

### 2. Progress Persistence
Save progress to allow resumption:
```go
func (o *Orchestrator) ResumeGeneration(runID string) error {
    // Find incomplete tasks from previous run
    // Continue from last successful task
}
```

### 3. Cleanup Command
Add maintenance commands:
```go
case "cleanup":
    if err := storage.CleanupOldRuns(24); err != nil {
        log.Fatal("Cleanup failed:", err)
    }
    fmt.Println(" Cleaned old runs")
```

## Comparison with Iteration 1

| Issue | Iteration 1 | Iteration 2 | Priority |
|-------|-------------|-------------|----------|
| Markdown wrapping | Present (100% failure) | Not tested (didn't run) | High |
| Task ID conflicts | Not discovered | Fatal error | Critical |
| Error reporting | Not tested | Misleading success message | High |
| Database persistence | Worked | Prevents reruns | Critical |
| Missing main.go | Present | Not tested | Medium |

## Pre-Iteration 3 Checklist

### Must Fix Before Running Again
- [ ] Implement unique task IDs (Fix 1)
- [ ] Fix error reporting (Fix 2)
- [ ] Add database cleanup mechanism (Fix 3)
- [ ] Keep markdown cleaning from iteration 1 fixes

### Should Test
- [ ] Multiple consecutive runs
- [ ] Error recovery behavior
- [ ] Database state after failures
- [ ] Different output directories

### Consider Adding
- [ ] `-clean` flag for database reset
- [ ] Run ID in output for tracking
- [ ] Better error messages with solutions
- [ ] Session management for isolation

## Key Learnings

1. **Persistence Requires Planning**: Database persistence needs session management
2. **Error Paths Matter**: Must test failure scenarios, not just success
3. **Idempotency**: System should handle multiple runs gracefully
4. **Clear Failure Reporting**: Never show success messages on failure

## Next Steps

1. **Implement Critical Fixes**: Task IDs and error handling
2. **Add Database Management**: Cleanup or sessions
3. **Retest with Fixes**: Ensure multiple runs work
4. **Then Address Iteration 1 Issues**: Markdown wrapping, missing files
5. **Consider Model Change**: Try DeepSeek Coder for better output

## Success Criteria for Iteration 3

- [ ] Can run generator multiple times without manual cleanup
- [ ] Clear error messages on failure (no false success)
- [ ] Generated code actually compiles (from iteration 1 fixes)
- [ ] All 5 files generated (including main.go)
- [ ] Tests can run (even if they fail)

## Commands for Next Test

```bash
# After implementing fixes
make build

# Test multiple runs
./overnight-llm -output ./run1
./overnight-llm -output ./run2  # Should work!

# Verify both runs completed
ls -la run1/ run2/

# Check database has both runs
sqlite3 poc.db "SELECT DISTINCT substr(id, 1, 15) as run FROM tasks;"
```

---

**Critical Insight**: The system was designed for single-run proof of concept, not production use. Iteration 3 must add basic multi-run support to be useful for development and testing.