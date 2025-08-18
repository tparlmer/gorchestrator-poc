-- Schema for tracking code generation tasks and their outputs
-- This schema is embedded in the binary and executed on initialization

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

-- Index for faster task lookups by status
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);

-- Index for faster file lookups by task
CREATE INDEX IF NOT EXISTS idx_files_task_id ON files_generated(task_id);