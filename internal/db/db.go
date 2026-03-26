package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

var (
	instance *sql.DB
	once     sync.Once
)

// Init opens (or creates) the SQLite database and applies the schema.
func Init(dbPath string) (*sql.DB, error) {
	var initErr error
	once.Do(func() {
		if dbPath == "" {
			home, _ := os.UserHomeDir()
			dbPath = filepath.Join(home, ".tot-mcp", "tot.db")
		}
		dir := filepath.Dir(dbPath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			initErr = fmt.Errorf("create db dir: %w", err)
			return
		}

		d, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
		if err != nil {
			initErr = fmt.Errorf("open db: %w", err)
			return
		}
		d.SetMaxOpenConns(1) // SQLite is single-writer
		if err := migrate(d); err != nil {
			initErr = fmt.Errorf("migrate: %w", err)
			return
		}
		instance = d
	})
	return instance, initErr
}

// Get returns the singleton database. Panics if Init was not called.
func Get() *sql.DB {
	if instance == nil {
		panic("db.Init() must be called first")
	}
	return instance
}

func migrate(d *sql.DB) error {
	_, err := d.Exec(schema)
	return err
}

const schema = `
CREATE TABLE IF NOT EXISTS trees (
	id               TEXT PRIMARY KEY,
	problem          TEXT NOT NULL,
	root_id          TEXT NOT NULL,
	search_strategy  TEXT NOT NULL DEFAULT 'bfs',
	max_depth        INTEGER NOT NULL DEFAULT 5,
	branching_factor INTEGER NOT NULL DEFAULT 3,
	status           TEXT NOT NULL DEFAULT 'active',
	created_at       TEXT NOT NULL,
	updated_at       TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS nodes (
	id          TEXT PRIMARY KEY,
	tree_id     TEXT NOT NULL REFERENCES trees(id) ON DELETE CASCADE,
	parent_id   TEXT REFERENCES nodes(id),
	thought     TEXT NOT NULL,
	evaluation  TEXT,
	score       REAL NOT NULL DEFAULT 0.0,
	depth       INTEGER NOT NULL DEFAULT 0,
	is_terminal INTEGER NOT NULL DEFAULT 0,
	metadata    TEXT NOT NULL DEFAULT '{}',
	created_at  TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_nodes_tree   ON nodes(tree_id);
CREATE INDEX IF NOT EXISTS idx_nodes_parent ON nodes(parent_id);

CREATE TABLE IF NOT EXISTS frontier (
	tree_id  TEXT NOT NULL REFERENCES trees(id) ON DELETE CASCADE,
	node_id  TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
	priority REAL NOT NULL DEFAULT 0.0,
	PRIMARY KEY (tree_id, node_id)
);

CREATE TABLE IF NOT EXISTS solutions (
	id         TEXT PRIMARY KEY,
	tree_id    TEXT,
	problem    TEXT NOT NULL,
	solution   TEXT NOT NULL,
	thoughts   TEXT NOT NULL DEFAULT '[]',
	path_ids   TEXT NOT NULL DEFAULT '[]',
	score      REAL NOT NULL DEFAULT 0.0,
	tags       TEXT NOT NULL DEFAULT '[]',
	embedding  BLOB,
	created_at TEXT NOT NULL
);

CREATE VIRTUAL TABLE IF NOT EXISTS solutions_fts USING fts5(
	problem, solution, thoughts,
	content='solutions', content_rowid='rowid'
);

CREATE TRIGGER IF NOT EXISTS solutions_ai AFTER INSERT ON solutions BEGIN
	INSERT INTO solutions_fts(rowid, problem, solution, thoughts)
	VALUES (new.rowid, new.problem, new.solution, new.thoughts);
END;
CREATE TRIGGER IF NOT EXISTS solutions_ad AFTER DELETE ON solutions BEGIN
	INSERT INTO solutions_fts(solutions_fts, rowid, problem, solution, thoughts)
	VALUES ('delete', old.rowid, old.problem, old.solution, old.thoughts);
END;
CREATE TRIGGER IF NOT EXISTS solutions_au AFTER UPDATE ON solutions BEGIN
	INSERT INTO solutions_fts(solutions_fts, rowid, problem, solution, thoughts)
	VALUES ('delete', old.rowid, old.problem, old.solution, old.thoughts);
	INSERT INTO solutions_fts(rowid, problem, solution, thoughts)
	VALUES (new.rowid, new.problem, new.solution, new.thoughts);
END;

CREATE TABLE IF NOT EXISTS experiment_configs (
	tree_id TEXT PRIMARY KEY REFERENCES trees(id) ON DELETE CASCADE,
	config  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS experiment_results (
	id               INTEGER PRIMARY KEY AUTOINCREMENT,
	tree_id          TEXT NOT NULL REFERENCES trees(id) ON DELETE CASCADE,
	node_id          TEXT NOT NULL,
	status           TEXT NOT NULL,
	metric           REAL,
	memory_mb        REAL,
	duration_seconds REAL,
	commit_hash      TEXT,
	kept             INTEGER NOT NULL DEFAULT 0,
	log_tail         TEXT,
	created_at       TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_expr_tree ON experiment_results(tree_id);
`
