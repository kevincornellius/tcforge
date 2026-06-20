package db

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

func Init(contestDir string) error {
	tcforgeDir := filepath.Join(contestDir, ".tcforge")
	if err := os.MkdirAll(tcforgeDir, 0755); err != nil {
		return err
	}

	db, err := sql.Open("sqlite", filepath.Join(tcforgeDir, "db.sqlite"))
	if err != nil {
		return err
	}

	db.SetMaxOpenConns(1) // SQLite is single-writer
	DB = db
	return migrate()
}

func migrate() error {
	_, err := DB.Exec(`
	PRAGMA journal_mode=WAL;
	PRAGMA foreign_keys=ON;

	CREATE TABLE IF NOT EXISTS users (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		username     TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		display_name TEXT NOT NULL,
		is_admin     INTEGER NOT NULL DEFAULT 0,
		created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS sessions (
		token      TEXT PRIMARY KEY,
		user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS problems (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		slug         TEXT NOT NULL UNIQUE,
		path         TEXT NOT NULL,
		title        TEXT NOT NULL,
		time_limit   INTEGER NOT NULL DEFAULT 1,
		memory_limit INTEGER NOT NULL DEFAULT 256,
		position     INTEGER NOT NULL DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS submissions (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id     INTEGER NOT NULL REFERENCES users(id),
		problem_id  INTEGER NOT NULL REFERENCES problems(id),
		language    TEXT NOT NULL,
		code        TEXT NOT NULL,
		status      TEXT NOT NULL DEFAULT 'queued',
		verdict     TEXT NOT NULL DEFAULT '',
		score       INTEGER NOT NULL DEFAULT 0,
		time_ms     INTEGER NOT NULL DEFAULT 0,
		memory_kb   INTEGER NOT NULL DEFAULT 0,
		submitted_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS verdicts (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		submission_id INTEGER NOT NULL REFERENCES submissions(id) ON DELETE CASCADE,
		test_case     TEXT NOT NULL,
		verdict       TEXT NOT NULL,
		time_ms       INTEGER NOT NULL DEFAULT 0,
		memory_kb     INTEGER NOT NULL DEFAULT 0,
		group_num     INTEGER NOT NULL DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS subtask_scores (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		submission_id INTEGER NOT NULL REFERENCES submissions(id) ON DELETE CASCADE,
		subtask_num   INTEGER NOT NULL,
		verdict       TEXT NOT NULL DEFAULT '',
		score         INTEGER NOT NULL DEFAULT 0,
		max_score     INTEGER NOT NULL DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS contest_state (
		id               INTEGER PRIMARY KEY CHECK (id = 1),
		name             TEXT NOT NULL DEFAULT '',
		duration         TEXT NOT NULL DEFAULT '',
		scoring          TEXT NOT NULL DEFAULT 'ioi',
		always_open      INTEGER NOT NULL DEFAULT 1,
		allow_submission INTEGER NOT NULL DEFAULT 1,
		start_at         TEXT,
		end_at           TEXT
	);
	INSERT OR IGNORE INTO contest_state (id, name, duration, scoring) VALUES (1, '', '', 'ioi');

	CREATE TABLE IF NOT EXISTS announcements (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		message    TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS problem_statements (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		problem_id INTEGER NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
		language   TEXT NOT NULL,
		filename   TEXT NOT NULL,
		format     TEXT NOT NULL,
		UNIQUE (problem_id, language)
	);
	`)
	if err != nil {
		return err
	}
	// Migrate existing DBs — ignore errors if columns/tables already exist
	DB.Exec("ALTER TABLE verdicts ADD COLUMN group_num INTEGER NOT NULL DEFAULT 0")
	DB.Exec(`CREATE TABLE IF NOT EXISTS subtask_scores (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		submission_id INTEGER NOT NULL REFERENCES submissions(id) ON DELETE CASCADE,
		subtask_num INTEGER NOT NULL,
		verdict TEXT NOT NULL DEFAULT '',
		score INTEGER NOT NULL DEFAULT 0,
		max_score INTEGER NOT NULL DEFAULT 0
	)`)
	DB.Exec(`CREATE TABLE IF NOT EXISTS contest_state (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		name TEXT NOT NULL DEFAULT '',
		duration TEXT NOT NULL DEFAULT '',
		scoring TEXT NOT NULL DEFAULT 'ioi',
		always_open INTEGER NOT NULL DEFAULT 1,
		start_at TEXT, end_at TEXT
	)`)
	DB.Exec(`ALTER TABLE contest_state ADD COLUMN always_open INTEGER NOT NULL DEFAULT 1`)
	DB.Exec(`ALTER TABLE contest_state ADD COLUMN allow_submission INTEGER NOT NULL DEFAULT 1`)
	DB.Exec(`INSERT OR IGNORE INTO contest_state (id, name, duration, scoring) VALUES (1,'','','ioi')`)
	DB.Exec(`CREATE TABLE IF NOT EXISTS announcements (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		message TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	DB.Exec(`CREATE TABLE IF NOT EXISTS problem_statements (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		problem_id INTEGER NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
		language TEXT NOT NULL, filename TEXT NOT NULL, format TEXT NOT NULL,
		UNIQUE (problem_id, language)
	)`)
	return nil
}
