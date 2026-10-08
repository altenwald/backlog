package store

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/altenwald/backlog/pkg/model"
	"github.com/google/uuid"
)

func copySQLite(src, dest string) error {
	u := url.URL{Scheme: "file", Path: src}
	q := u.Query()
	q.Set("mode", "ro")
	u.RawQuery = q.Encode()
	db, e := sql.Open("sqlite", u.String())
	if e != nil {
		return e
	}
	defer db.Close()
	_, e = db.Exec(`VACUUM INTO ?`, dest)
	return e
}
func readFullProject(q querier, slug string) (*model.Project, error) {
	p, e := scanProject(q.QueryRow(`SELECT `+projectColumns+` FROM projects WHERE slug=?`, slug))
	if e != nil {
		return nil, e
	}
	p.Tasks, e = loadTasks(q, slug, "", nil)
	if e != nil {
		return nil, e
	}
	p.Spec, e = loadSections(q, slug)
	return p, e
}

// Activation is one catalog transaction after every project has passed SQLite
// integrity/foreign-key checks. Originals are untouched. An interrupted staging
// directory is harmless and never treated as an activated migration.
func (s *Store) migrateLayout() error {
	tx, e := s.db.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	v, e := getSetting(tx, "project_layout")
	if e != nil {
		return e
	}
	if v == "1" {
		return nil
	}
	unlock, e := migrationLock(s.dataDir)
	if e != nil {
		return e
	}
	defer unlock()
	stage, e := os.MkdirTemp(s.dataDir, ".layout-")
	if e != nil {
		return e
	}
	activated := false
	var installed []string
	defer func() {
		os.RemoveAll(stage)
		if !activated {
			for _, dir := range installed {
				os.RemoveAll(dir)
			}
		}
	}()
	var src *projectDB
	path := filepath.Join(s.dataDir, dbFileName)
	if _, e = os.Stat(path); e == nil {
		clone := filepath.Join(stage, "source.db")
		if e = copySQLite(path, clone); e != nil {
			return e
		}
		db, e := openDB(clone)
		if e != nil {
			return e
		}
		src = &projectDB{db: db}
		defer db.Close()
	} else if !os.IsNotExist(e) {
		return e
	}
	var projects []*model.Project
	cfg := legacyConfig{}
	nextIDs := map[string]int{}
	if src != nil {
		metadata, e := src.listProjectsChecked()
		if e != nil {
			return e
		}
		for _, p := range metadata {
			full, e := src.getFullProject(p.Slug)
			if e != nil {
				return e
			}
			projects = append(projects, full)
			var n int
			if e = src.db.QueryRow(`SELECT next_task_id FROM projects WHERE slug=?`, p.Slug).Scan(&n); e != nil {
				return e
			}
			nextIDs[p.Slug] = n
		}
		cfg.ActiveProject = src.GetActiveProjectSlug()
		cfg.MCPUserInstructions = src.GetMCPUserInstructions()
	} else if hasLegacyData(s.dataDir) {
		cfg, projects, e = readLegacy(s.dataDir)
		if e != nil {
			return e
		}
	}
	for i, p := range projects {
		file := filepath.Join(stage, fmt.Sprintf("project-%d.db", i))
		db, e := openDB(file)
		if e != nil {
			return e
		}
		d := &projectDB{db: db}
		e = d.withTx(func(t *sql.Tx) error {
			if e := importProject(t, p); e != nil {
				return e
			}
			if n := nextIDs[p.Slug]; n > 0 {
				_, e := t.Exec(`UPDATE projects SET next_task_id=?`, n)
				return e
			}
			return nil
		})
		if e == nil {
			var check string
			e = db.QueryRow(`PRAGMA integrity_check`).Scan(&check)
			if e == nil && check != "ok" {
				e = fmt.Errorf("integrity check: %s", check)
			}
		}
		if e == nil {
			rows, err := db.Query(`PRAGMA foreign_key_check`)
			if err != nil {
				e = err
			} else {
				if rows.Next() {
					e = fmt.Errorf("foreign key check failed for %s", p.Slug)
				}
				rows.Close()
			}
		}
		if e == nil {
			_, e = db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
		}
		db.Close()
		if e != nil {
			return e
		}
		rel := filepath.Join("projects", uuid.NewString(), "project.db")
		target := filepath.Join(s.dataDir, rel)
		if e = os.MkdirAll(filepath.Dir(target), 0700); e != nil {
			return e
		}
		installed = append(installed, filepath.Dir(target))
		if e = os.Rename(file, target); e != nil {
			return e
		}
		if fd, err := os.Open(filepath.Dir(target)); err == nil {
			_ = fd.Sync()
			fd.Close()
		}
		if _, e = tx.Exec(`INSERT INTO project_files(slug,path) VALUES(?,?)`, p.Slug, rel); e != nil {
			return e
		}
	}
	if e = setSetting(tx, settingActiveProject, cfg.ActiveProject); e != nil {
		return e
	}
	if e = setSetting(tx, settingMCPUserInstructions, cfg.MCPUserInstructions); e != nil {
		return e
	}
	if e = setSetting(tx, "project_layout", "1"); e != nil {
		return e
	}
	// Keep completed files even if COMMIT reports an ambiguous I/O failure.
	activated = true
	if e = tx.Commit(); e != nil {
		return e
	}
	activated = true
	return nil
}
