package store

import (
	"database/sql"
	"fmt"
	"net/url"
	"time"

	_ "modernc.org/sqlite"
)

const dbFileName = "backlog.db"

type migration struct {
	sql string
	// after runs in the same transaction, for data changes SQL cannot express.
	after func(tx *sql.Tx) error
}

// migrations are applied in order; PRAGMA user_version records how many ran.
// Never edit a released migration: append a new one instead.
var migrations = []migration{
	{sql: `
CREATE TABLE settings (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

CREATE TABLE projects (
  slug         TEXT PRIMARY KEY,
  name         TEXT NOT NULL,
  description  TEXT NOT NULL DEFAULT '',
  next_task_id INTEGER NOT NULL DEFAULT 1,
  inserted_at  TEXT NOT NULL,
  updated_at   TEXT NOT NULL
);

CREATE TABLE spec_sections (
  project_slug TEXT NOT NULL REFERENCES projects(slug) ON DELETE CASCADE,
  id           TEXT NOT NULL,
  position     INTEGER NOT NULL,
  title        TEXT NOT NULL DEFAULT '',
  body         TEXT NOT NULL DEFAULT '',
  updated_at   TEXT NOT NULL,
  PRIMARY KEY (project_slug, id)
);

CREATE TABLE tasks (
  project_slug  TEXT NOT NULL REFERENCES projects(slug) ON DELETE CASCADE,
  id            TEXT NOT NULL,
  parent_id     TEXT,
  title         TEXT NOT NULL,
  description   TEXT NOT NULL DEFAULT '',
  size          TEXT NOT NULL CHECK (size IN ('XS','S','M','L','XL')),
  tier          INTEGER NOT NULL CHECK (tier BETWEEN 1 AND 5),
  done          INTEGER NOT NULL DEFAULT 0,
  deprecated    INTEGER NOT NULL DEFAULT 0,
  assignee      TEXT NOT NULL DEFAULT '',
  resolution    TEXT NOT NULL DEFAULT '',
  inserted_at   TEXT NOT NULL,
  updated_at    TEXT NOT NULL,
  terminated_at TEXT,
  PRIMARY KEY (project_slug, id),
  FOREIGN KEY (project_slug, parent_id) REFERENCES tasks(project_slug, id)
    ON DELETE CASCADE DEFERRABLE INITIALLY DEFERRED
);
CREATE INDEX tasks_open     ON tasks(project_slug, done, tier);
CREATE INDEX tasks_assignee ON tasks(project_slug, assignee, done);
CREATE INDEX tasks_parent   ON tasks(project_slug, parent_id);

CREATE TABLE task_deps (
  project_slug TEXT NOT NULL,
  task_id      TEXT NOT NULL,
  depends_on   TEXT NOT NULL,
  PRIMARY KEY (project_slug, task_id, depends_on),
  FOREIGN KEY (project_slug, task_id) REFERENCES tasks(project_slug, id)
    ON DELETE CASCADE DEFERRABLE INITIALLY DEFERRED,
  FOREIGN KEY (project_slug, depends_on) REFERENCES tasks(project_slug, id)
    ON DELETE CASCADE DEFERRABLE INITIALLY DEFERRED
);
CREATE INDEX task_deps_reverse ON task_deps(project_slug, depends_on);
`},
	{
		// Spec pages form a wiki: links between pages, main page first.
		sql: `
CREATE TABLE spec_links (
  project_slug TEXT NOT NULL,
  from_id      TEXT NOT NULL,
  to_id        TEXT NOT NULL,
  position     INTEGER NOT NULL,
  PRIMARY KEY (project_slug, from_id, to_id),
  FOREIGN KEY (project_slug, from_id) REFERENCES spec_sections(project_slug, id) ON DELETE CASCADE
);
CREATE INDEX spec_links_to ON spec_links(project_slug, to_id);
`,
		after: normalizeStoredSpecs,
	},
}

func openDB(path string) (*sql.DB, error) {
	q := url.Values{}
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "synchronous(NORMAL)")
	q.Add("_pragma", "busy_timeout(5000)")
	q.Set("_txlock", "immediate")
	db, err := sql.Open("sqlite", "file:"+path+"?"+q.Encode())
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("cannot open %s: %w", path, err)
	}
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("cannot migrate %s: %w", path, err)
	}
	return db, nil
}

func migrate(db *sql.DB) error {
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return err
	}
	if version > len(migrations) {
		return fmt.Errorf("database schema version %d is newer than this build supports (%d)", version, len(migrations))
	}
	for i := version; i < len(migrations); i++ {
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(migrations[i].sql); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
		if after := migrations[i].after; after != nil {
			if err := after(tx); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("migration %d: %w", i+1, err)
			}
		}
		if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, i+1)); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func formatTime(t time.Time) string {
	return t.Format(time.RFC3339Nano)
}

func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, s)
	return t
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
