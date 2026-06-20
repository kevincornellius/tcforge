package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

// Database wraps *sql.DB and auto-rebinds ? placeholders to $N for Postgres.
type Database struct {
	*sql.DB
	postgres bool
}

var DB *Database

func (d *Database) rebind(q string) string {
	if !d.postgres {
		return q
	}
	n := 0
	var b strings.Builder
	for _, r := range q {
		if r == '?' {
			n++
			fmt.Fprintf(&b, "$%d", n)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (d *Database) Exec(query string, args ...any) (sql.Result, error) {
	return d.DB.Exec(d.rebind(query), args...)
}

func (d *Database) QueryRow(query string, args ...any) *sql.Row {
	return d.DB.QueryRow(d.rebind(query), args...)
}

func (d *Database) Query(query string, args ...any) (*sql.Rows, error) {
	return d.DB.Query(d.rebind(query), args...)
}

func Init(contestDir string) error {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL != "" {
		return initPostgres(dbURL)
	}
	return initSQLite(contestDir)
}

func initPostgres(url string) error {
	sqldb, err := sql.Open("postgres", url)
	if err != nil {
		return err
	}
	sqldb.SetMaxOpenConns(10)
	DB = &Database{DB: sqldb, postgres: true}
	log.Println("db: using postgres")
	return migratePostgres()
}

func initSQLite(contestDir string) error {
	dbPath := os.Getenv("TCFORGE_DB_PATH")
	if dbPath == "" {
		dbPath = filepath.Join(contestDir, ".tcforge", "db.sqlite")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return err
	}
	sqldb, err := sql.Open("sqlite", dbPath+"?_busy_timeout=5000")
	if err != nil {
		return err
	}
	sqldb.SetMaxOpenConns(1)
	DB = &Database{DB: sqldb, postgres: false}
	log.Println("db: using sqlite at", dbPath)
	return migrateSQLite()
}

func migrateSQLite() error {
	stmts := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		`CREATE TABLE IF NOT EXISTS users (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			username      TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			display_name  TEXT NOT NULL,
			is_admin      INTEGER NOT NULL DEFAULT 0,
			created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			token      TEXT PRIMARY KEY,
			user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS problems (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			slug         TEXT NOT NULL UNIQUE,
			path         TEXT NOT NULL,
			title        TEXT NOT NULL,
			time_limit   INTEGER NOT NULL DEFAULT 1,
			memory_limit INTEGER NOT NULL DEFAULT 256,
			position     INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS submissions (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id      INTEGER NOT NULL REFERENCES users(id),
			problem_id   INTEGER NOT NULL REFERENCES problems(id),
			language     TEXT NOT NULL,
			code         TEXT NOT NULL,
			status       TEXT NOT NULL DEFAULT 'queued',
			verdict      TEXT NOT NULL DEFAULT '',
			score        INTEGER NOT NULL DEFAULT 0,
			time_ms      INTEGER NOT NULL DEFAULT 0,
			memory_kb    INTEGER NOT NULL DEFAULT 0,
			submitted_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			graded_at    DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS verdicts (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			submission_id   INTEGER NOT NULL REFERENCES submissions(id) ON DELETE CASCADE,
			test_case       TEXT NOT NULL,
			verdict         TEXT NOT NULL,
			time_ms         INTEGER NOT NULL DEFAULT 0,
			memory_kb       INTEGER NOT NULL DEFAULT 0,
			group_num       INTEGER NOT NULL DEFAULT 0,
			points_fraction REAL NOT NULL DEFAULT 1.0
		)`,
		`CREATE TABLE IF NOT EXISTS subtask_scores (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			submission_id INTEGER NOT NULL REFERENCES submissions(id) ON DELETE CASCADE,
			subtask_num   INTEGER NOT NULL,
			verdict       TEXT NOT NULL DEFAULT '',
			score         INTEGER NOT NULL DEFAULT 0,
			max_score     INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS contest_state (
			id               INTEGER PRIMARY KEY CHECK (id = 1),
			name             TEXT NOT NULL DEFAULT '',
			duration         TEXT NOT NULL DEFAULT '',
			scoring          TEXT NOT NULL DEFAULT 'ioi',
			always_open      INTEGER NOT NULL DEFAULT 1,
			allow_submission INTEGER NOT NULL DEFAULT 1,
			start_at         TEXT,
			end_at           TEXT
		)`,
		`INSERT OR IGNORE INTO contest_state (id, name, duration, scoring) VALUES (1, '', '', 'ioi')`,
		`CREATE TABLE IF NOT EXISTS announcements (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			message    TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS problem_statements (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			problem_id INTEGER NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
			language   TEXT NOT NULL,
			filename   TEXT NOT NULL,
			format     TEXT NOT NULL,
			UNIQUE (problem_id, language)
		)`,
	}
	for _, s := range stmts {
		if _, err := DB.DB.Exec(s); err != nil {
			return err
		}
	}
	runIgnored(
		"ALTER TABLE submissions ADD COLUMN graded_at DATETIME",
		"ALTER TABLE verdicts ADD COLUMN group_num INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE verdicts ADD COLUMN points_fraction REAL NOT NULL DEFAULT 1.0",
	)
	return nil
}

func migratePostgres() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id            SERIAL PRIMARY KEY,
			username      TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			display_name  TEXT NOT NULL,
			is_admin      INTEGER NOT NULL DEFAULT 0,
			created_at    TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			token      TEXT PRIMARY KEY,
			user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS problems (
			id           SERIAL PRIMARY KEY,
			slug         TEXT NOT NULL UNIQUE,
			path         TEXT NOT NULL,
			title        TEXT NOT NULL,
			time_limit   INTEGER NOT NULL DEFAULT 1,
			memory_limit INTEGER NOT NULL DEFAULT 256,
			position     INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS submissions (
			id           SERIAL PRIMARY KEY,
			user_id      INTEGER NOT NULL REFERENCES users(id),
			problem_id   INTEGER NOT NULL REFERENCES problems(id),
			language     TEXT NOT NULL,
			code         TEXT NOT NULL,
			status       TEXT NOT NULL DEFAULT 'queued',
			verdict      TEXT NOT NULL DEFAULT '',
			score        INTEGER NOT NULL DEFAULT 0,
			time_ms      INTEGER NOT NULL DEFAULT 0,
			memory_kb    INTEGER NOT NULL DEFAULT 0,
			submitted_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			graded_at    TIMESTAMPTZ
		)`,
		`CREATE TABLE IF NOT EXISTS verdicts (
			id              SERIAL PRIMARY KEY,
			submission_id   INTEGER NOT NULL REFERENCES submissions(id) ON DELETE CASCADE,
			test_case       TEXT NOT NULL,
			verdict         TEXT NOT NULL,
			time_ms         INTEGER NOT NULL DEFAULT 0,
			memory_kb       INTEGER NOT NULL DEFAULT 0,
			group_num       INTEGER NOT NULL DEFAULT 0,
			points_fraction REAL NOT NULL DEFAULT 1.0
		)`,
		`CREATE TABLE IF NOT EXISTS subtask_scores (
			id            SERIAL PRIMARY KEY,
			submission_id INTEGER NOT NULL REFERENCES submissions(id) ON DELETE CASCADE,
			subtask_num   INTEGER NOT NULL,
			verdict       TEXT NOT NULL DEFAULT '',
			score         INTEGER NOT NULL DEFAULT 0,
			max_score     INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS contest_state (
			id               INTEGER PRIMARY KEY CHECK (id = 1),
			name             TEXT NOT NULL DEFAULT '',
			duration         TEXT NOT NULL DEFAULT '',
			scoring          TEXT NOT NULL DEFAULT 'ioi',
			always_open      INTEGER NOT NULL DEFAULT 1,
			allow_submission INTEGER NOT NULL DEFAULT 1,
			start_at         TEXT,
			end_at           TEXT
		)`,
		`INSERT INTO contest_state (id, name, duration, scoring) VALUES (1, '', '', 'ioi') ON CONFLICT DO NOTHING`,
		`CREATE TABLE IF NOT EXISTS announcements (
			id         SERIAL PRIMARY KEY,
			message    TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS problem_statements (
			id         SERIAL PRIMARY KEY,
			problem_id INTEGER NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
			language   TEXT NOT NULL,
			filename   TEXT NOT NULL,
			format     TEXT NOT NULL,
			UNIQUE (problem_id, language)
		)`,
	}
	for _, s := range stmts {
		if _, err := DB.DB.Exec(s); err != nil {
			return err
		}
	}
	runIgnored(
		"ALTER TABLE submissions ADD COLUMN IF NOT EXISTS graded_at TIMESTAMPTZ",
		"ALTER TABLE verdicts ADD COLUMN IF NOT EXISTS group_num INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE verdicts ADD COLUMN IF NOT EXISTS points_fraction REAL NOT NULL DEFAULT 1.0",
	)
	return nil
}

func runIgnored(stmts ...string) {
	for _, s := range stmts {
		DB.DB.Exec(s)
	}
}
