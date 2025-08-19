package storage

import (
	"database/sql"
	_ "embed"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Embed the SQL schema file at compile time
// This ensures the schema is always available in the binary
//
//go:embed schema.sql
var schemaSQL string

// Task represents a code generation task in the database
type Task struct {
	ID        string
	Type      string
	Input     string
	Output    string
	Status    string
	Error     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// FileGenerated represents a generated file entry in the database
type FileGenerated struct {
	ID        int64
	TaskID    string
	FilePath  string
	Content   string
	CreatedAt time.Time
}

// Storage provides database operations for tasks and generated files
type Storage struct {
	db *sql.DB
}

// InitDB initializes the SQLite database with embedded schema
// Creates tables if they don't exist and returns a database handle
func InitDB(dbPath string) (*sql.DB, error) {
	// Open SQLite database (creates file if it doesn't exist)
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Execute embedded schema to create tables
	if _, err := db.Exec(schemaSQL); err != nil {
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	// Enable foreign keys (disabled by default in SQLite)
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Set reasonable connection pool settings
	db.SetMaxOpenConns(1) // SQLite doesn't benefit from multiple connections
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(time.Hour)

	return db, nil
}

// NewStorage creates a new storage instance with the given database
func NewStorage(db *sql.DB) *Storage {
	return &Storage{db: db}
}

// CreateTask inserts a new task into the database
func (s *Storage) CreateTask(task Task) error {
	query := `
		INSERT INTO tasks (id, type, input, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	now := time.Now()
	_, err := s.db.Exec(query, task.ID, task.Type, task.Input, task.Status, now, now)
	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}
	return nil
}

// UpdateTaskStatus updates the status and timestamp of a task
func (s *Storage) UpdateTaskStatus(taskID string, status string) error {
	query := `
		UPDATE tasks 
		SET status = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := s.db.Exec(query, status, time.Now(), taskID)
	if err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}
	return nil
}

// UpdateTaskOutput updates the output of a completed task
func (s *Storage) UpdateTaskOutput(taskID string, output string) error {
	query := `
		UPDATE tasks 
		SET output = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := s.db.Exec(query, output, time.Now(), taskID)
	if err != nil {
		return fmt.Errorf("failed to update task output: %w", err)
	}
	return nil
}

// UpdateTaskError updates the error message of a failed task
func (s *Storage) UpdateTaskError(taskID string, errorMsg string) error {
	query := `
		UPDATE tasks 
		SET error = ?, status = 'failed', updated_at = ?
		WHERE id = ?
	`
	_, err := s.db.Exec(query, errorMsg, time.Now(), taskID)
	if err != nil {
		return fmt.Errorf("failed to update task error: %w", err)
	}
	return nil
}

// SaveGeneratedFile stores a generated file in the database
func (s *Storage) SaveGeneratedFile(taskID, filePath, content string) error {
	query := `
		INSERT INTO files_generated (task_id, file_path, content, created_at)
		VALUES (?, ?, ?, ?)
	`
	_, err := s.db.Exec(query, taskID, filePath, content, time.Now())
	if err != nil {
		return fmt.Errorf("failed to save generated file: %w", err)
	}
	return nil
}

// GetTask retrieves a task by ID
func (s *Storage) GetTask(taskID string) (*Task, error) {
	query := `
		SELECT id, type, input, output, status, error, created_at, updated_at
		FROM tasks
		WHERE id = ?
	`
	var task Task
	var nullOutput, nullError sql.NullString

	err := s.db.QueryRow(query, taskID).Scan(
		&task.ID, &task.Type, &task.Input, &nullOutput,
		&task.Status, &nullError, &task.CreatedAt, &task.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("task not found: %s", taskID)
		}
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	// Handle nullable fields
	if nullOutput.Valid {
		task.Output = nullOutput.String
	}
	if nullError.Valid {
		task.Error = nullError.String
	}

	return &task, nil
}

// GetAllTasks retrieves all tasks from the database
func (s *Storage) GetAllTasks() ([]Task, error) {
	query := `
		SELECT id, type, input, output, status, error, created_at, updated_at
		FROM tasks
		ORDER BY created_at ASC
	`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query tasks: %w", err)
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var task Task
		var nullOutput, nullError sql.NullString

		err := rows.Scan(
			&task.ID, &task.Type, &task.Input, &nullOutput,
			&task.Status, &nullError, &task.CreatedAt, &task.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}

		// Handle nullable fields
		if nullOutput.Valid {
			task.Output = nullOutput.String
		}
		if nullError.Valid {
			task.Error = nullError.String
		}

		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tasks: %w", err)
	}

	return tasks, nil
}

// GetGeneratedFiles retrieves all files generated for a specific task
func (s *Storage) GetGeneratedFiles(taskID string) ([]FileGenerated, error) {
	query := `
		SELECT id, task_id, file_path, content, created_at
		FROM files_generated
		WHERE task_id = ?
		ORDER BY created_at ASC
	`
	rows, err := s.db.Query(query, taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to query generated files: %w", err)
	}
	defer rows.Close()

	var files []FileGenerated
	for rows.Next() {
		var file FileGenerated
		err := rows.Scan(&file.ID, &file.TaskID, &file.FilePath, &file.Content, &file.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan file: %w", err)
		}
		files = append(files, file)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating files: %w", err)
	}

	return files, nil
}

// CleanAllTasks removes all tasks and generated files from the database
// This is useful for cleaning up before a new run
func (s *Storage) CleanAllTasks() error {
	// Start a transaction for atomic cleanup
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete all generated files first (foreign key constraint)
	if _, err := tx.Exec("DELETE FROM files_generated"); err != nil {
		return fmt.Errorf("failed to delete generated files: %w", err)
	}

	// Delete all tasks
	if _, err := tx.Exec("DELETE FROM tasks"); err != nil {
		return fmt.Errorf("failed to delete tasks: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Close closes the database connection
func (s *Storage) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
