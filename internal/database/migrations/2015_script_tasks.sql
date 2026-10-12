CREATE TABLE script_definitions (
    id TEXT PRIMARY KEY,
    document TEXT NOT NULL
);
CREATE TABLE script_tasks (
    id TEXT PRIMARY KEY,
    script_id TEXT NOT NULL REFERENCES script_definitions(id),
    document TEXT NOT NULL,
    next_run BIGINT NOT NULL DEFAULT 0
);
CREATE INDEX idx_script_tasks_script ON script_tasks(script_id);
CREATE TABLE script_task_state (
    task_id TEXT NOT NULL REFERENCES script_tasks(id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    value TEXT NOT NULL,
    PRIMARY KEY (task_id, key)
);
CREATE TABLE script_task_runs (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL REFERENCES script_tasks(id) ON DELETE CASCADE,
    started_at BIGINT NOT NULL,
    status TEXT NOT NULL,
    document TEXT NOT NULL
);
CREATE INDEX idx_script_task_runs_task ON script_task_runs(task_id, started_at);
CREATE INDEX idx_script_task_runs_started ON script_task_runs(started_at);
CREATE INDEX idx_script_task_runs_status ON script_task_runs(status);
