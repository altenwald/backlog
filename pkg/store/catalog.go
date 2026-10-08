package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/altenwald/backlog/pkg/model"
	"github.com/google/uuid"
)

// Store owns the local catalog. Project handles live for one operation, keeping
// descriptors bounded even when a catalog contains thousands of projects.
type Store struct {
	db        *sql.DB
	dataDir   string
	mu        sync.Mutex
	listeners map[chan Event]struct{}
}

func NewStore(dir string) (*Store, error) {
	if dir == "" {
		h, e := os.UserHomeDir()
		if e != nil {
			return nil, e
		}
		dir = filepath.Join(h, ".config", "backlog")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	db, err := openDB(filepath.Join(dir, "catalog.db"))
	if err != nil {
		return nil, err
	}
	s := &Store{db: db, dataDir: dir, listeners: make(map[chan Event]struct{})}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS project_files (slug TEXT PRIMARY KEY, path TEXT NOT NULL UNIQUE);`)
	if err == nil {
		err = s.migrateLayout()
	}
	if err != nil {
		db.Close()
		return nil, err
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		_ = os.Chmod(filepath.Join(dir, "catalog.db")+suffix, 0600)
	}
	return s, nil
}
func (s *Store) Close() error {
	s.mu.Lock()
	for ch := range s.listeners {
		delete(s.listeners, ch)
		close(ch)
	}
	s.mu.Unlock()
	return s.db.Close()
}
func (s *Store) GetDataDir() string { return s.dataDir }
func (s *Store) Watch() (<-chan Event, func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch := make(chan Event, 32)
	s.listeners[ch] = struct{}{}
	var once sync.Once
	return ch, func() {
		once.Do(func() {
			s.mu.Lock()
			if _, ok := s.listeners[ch]; ok {
				delete(s.listeners, ch)
				close(ch)
			}
			s.mu.Unlock()
		})
	}
}
func (s *Store) Subscribe() <-chan Event { ch, _ := s.Watch(); return ch }
func (s *Store) Publish(ev Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.listeners {
		select {
		case ch <- ev:
		default:
			// Coalesce into a full refresh rather than silently losing invalidations.
			source := ev.Source
		drain:
			for {
				select {
				case old := <-ch:
					if old.Source == "" {
						source = ""
					}
				default:
					break drain
				}
			}
			ch <- Event{Type: "resync", Source: source}
		}
	}
}
func (s *Store) openProject(slug string) (*projectDB, error) {
	if slug == "" {
		slug = s.GetActiveProjectSlug()
	}
	slug = strings.ToLower(slug)
	var rel string
	if err := s.db.QueryRow(`SELECT path FROM project_files WHERE slug=?`, slug).Scan(&rel); err != nil {
		return nil, fmt.Errorf("project %q not found: %w", slug, err)
	}
	path := filepath.Join(s.dataDir, rel)
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	db, err := openDB(path)
	if err != nil {
		return nil, err
	}
	return &projectDB{db: db, onEvent: s.Publish}, nil
}
func (s *Store) ListProjects() []*model.Project {
	ps, err := s.ListProjectsChecked()
	if err != nil {
		log.Printf("[Backlog] Cannot read project catalog: %v", err)
	}
	return ps
}
func (s *Store) GetActiveProjectSlug() string {
	v, _ := getSetting(s.db, settingActiveProject)
	var one int
	if s.db.QueryRow(`SELECT 1 FROM project_files WHERE slug=?`, v).Scan(&one) == nil {
		return v
	}
	v = ""
	_ = s.db.QueryRow(`SELECT slug FROM project_files ORDER BY slug LIMIT 1`).Scan(&v)
	return v
}
func (s *Store) SetActiveProject(slug string) error {
	d, err := s.openProject(slug)
	if err != nil {
		return err
	}
	d.Close()
	if err = setSetting(s.db, settingActiveProject, strings.ToLower(slug)); err == nil {
		s.Publish(Event{Type: EventProjectSelected, ProjectSlug: slug})
	}
	return err
}
func (s *Store) GetMCPUserInstructions() string {
	v, _ := getSetting(s.db, settingMCPUserInstructions)
	return v
}
func (s *Store) SaveMCPUserInstructions(v string) error {
	return setSetting(s.db, settingMCPUserInstructions, v)
}
func (s *Store) CreateProject(slug, name, description string) (*model.Project, error) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if slug == "" || strings.Contains(slug, "::") {
		return nil, errors.New("project slug cannot be empty or contain ::")
	}
	if name == "" {
		name = strings.Title(slug)
	}
	now := time.Now()
	return s.installProject(&model.Project{Slug: slug, Name: name, Description: description, InsertedAt: now, UpdatedAt: now}, 0)
}
func (s *Store) installProject(p *model.Project, next int) (*model.Project, error) {
	rel := filepath.Join("projects", uuid.NewString(), "project.db")
	path := filepath.Join(s.dataDir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := openDB(path)
	if err != nil {
		return nil, err
	}
	d := &projectDB{db: db}
	err = d.withTx(func(tx *sql.Tx) error {
		if e := importProject(tx, p); e != nil {
			return e
		}
		if next > 0 {
			_, e := tx.Exec(`UPDATE projects SET next_task_id=?`, next)
			return e
		}
		return nil
	})
	if err == nil {
		_, err = db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	}
	db.Close()
	if err != nil {
		os.RemoveAll(filepath.Dir(path))
		return nil, err
	}
	if _, err = s.db.Exec(`INSERT INTO project_files(slug,path) VALUES(?,?)`, p.Slug, rel); err != nil {
		return nil, err
	}

	s.Publish(Event{Type: EventProjectCreated, ProjectSlug: p.Slug})
	return p, nil
}
func (s *Store) DeleteProject(slug string) error {
	if strings.TrimSpace(slug) == "" {
		return errors.New("project slug cannot be empty")
	}
	// Remove from the catalog first. Keep the independent file at its existing path for recovery.
	var rel string
	if err := s.db.QueryRow(`SELECT path FROM project_files WHERE slug=?`, slug).Scan(&rel); err != nil {
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM project_files WHERE slug=?`, slug); err != nil {
		return err
	}
	s.Publish(Event{Type: EventProjectDeleted, ProjectSlug: slug})
	return nil
}
func (s *Store) ImportProjectJSON(data []byte) (*model.Project, error) {
	var p model.Project
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	p.Slug = strings.ToLower(strings.TrimSpace(p.Slug))
	if p.Slug == "" {
		return nil, errors.New("missing project slug")
	}
	p.Open = false
	return s.installProject(&p, 0)
}
func (s *Store) SetProjectOpen(slug string, open bool) error {
	d, err := s.openProject(slug)
	if err != nil {
		return err
	}
	defer d.Close()
	err = d.withTx(func(tx *sql.Tx) error {
		_, e := tx.Exec(`UPDATE projects SET open=? WHERE slug=?`, open, slug)
		if e != nil {
			return e
		}
		return touchProject(tx, slug, time.Now())
	})
	if err == nil {
		s.Publish(Event{Type: EventProjectUpdated, ProjectSlug: slug})
	}
	return err
}

// ProjectBackup is a standalone, consistent SQLite file, safe to transfer while the app runs.
func (s *Store) ProjectBackup(slug, dest string) error {
	d, e := s.openProject(slug)
	if e != nil {
		return e
	}
	defer d.Close()
	return d.Backup(dest)
}
func (s *Store) ImportProjectDB(path string) (*model.Project, error) {
	// Work on a snapshot: never migrate or write to the user's source file.
	tmp, e := os.MkdirTemp(s.dataDir, ".import-")
	if e != nil {
		return nil, e
	}
	defer os.RemoveAll(tmp)
	clone := filepath.Join(tmp, "project.db")
	if e = copySQLite(path, clone); e != nil {
		return nil, e
	}
	db, e := openDB(clone)
	if e != nil {
		return nil, e
	}
	defer db.Close()
	d := &projectDB{db: db}
	ps, e := d.listProjectsChecked()
	if e != nil {
		return nil, e
	}
	if len(ps) != 1 {
		return nil, errors.New("expected exactly one project in the SQLite file")
	}
	p, e := d.getFullProject(ps[0].Slug)
	if e != nil {
		return nil, e
	}
	p.Open = false
	var next int
	if e = db.QueryRow(`SELECT next_task_id FROM projects`).Scan(&next); e != nil {
		return nil, e
	}
	return s.installProject(p, next)
}

// Backup retains the portable single-file backup format. Each project is read in
// a transaction; projects have independent timelines and need no cross-project transaction.
func (s *Store) Backup(dest string) error {
	if _, e := os.Stat(dest); e == nil {
		return fmt.Errorf("%s already exists", dest)
	}
	if e := os.MkdirAll(filepath.Dir(dest), 0700); e != nil {
		return e
	}
	tmp := dest + "." + uuid.NewString() + ".tmp"
	defer os.Remove(tmp)
	db, e := openDB(tmp)
	if e != nil {
		return e
	}
	defer db.Close()
	out := &projectDB{db: db}
	projects, e := s.ListProjectsChecked()
	if e != nil {
		return e
	}
	for _, p := range projects {
		d, e := s.openProject(p.Slug)
		if e != nil {
			return e
		}
		tx, e := d.db.Begin()
		if e != nil {
			d.Close()
			return e
		}
		p, e = readFullProject(tx, p.Slug)
		var next int
		if e == nil {
			e = tx.QueryRow(`SELECT next_task_id FROM projects WHERE slug=?`, p.Slug).Scan(&next)
		}
		tx.Rollback()
		d.Close()
		if e != nil {
			return e
		}
		if e = out.withTx(func(t *sql.Tx) error {
			if e := importProject(t, p); e != nil {
				return e
			}
			_, e := t.Exec(`UPDATE projects SET next_task_id=? WHERE slug=?`, next, p.Slug)
			return e
		}); e != nil {
			return e
		}
	}
	for _, key := range []string{settingActiveProject, settingMCPUserInstructions} {
		v, _ := getSetting(s.db, key)
		if e = setSetting(db, key, v); e != nil {
			return e
		}
	}
	if _, e = db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); e != nil {
		return e
	}
	if e = db.Close(); e != nil {
		return e
	}
	return os.Link(tmp, dest)
}
func (s *Store) AutoBackup(keep int) (string, error) {
	dir := filepath.Join(s.dataDir, backupsDirName)
	dest := filepath.Join(dir, "backlog-"+time.Now().Format("2006-01-02")+".db")
	if _, e := os.Stat(dest); e == nil {
		return "", nil
	}
	if e := s.Backup(dest); e != nil {
		return "", e
	}
	files, _ := filepath.Glob(filepath.Join(dir, "backlog-*.db"))
	sort.Strings(files)
	for keep > 0 && len(files) > keep {
		os.Remove(files[0])
		files = files[1:]
	}
	return dest, nil
}

// ListProjectsChecked is used for backups/migrations/network catalogs, where
// omitting an unreadable project must be an error, never a partial success.
func (s *Store) ListProjectsChecked() ([]*model.Project, error) {
	rows, e := s.db.Query(`SELECT slug FROM project_files ORDER BY slug`)
	if e != nil {
		return nil, e
	}
	var slugs []string
	for rows.Next() {
		var slug string
		if e = rows.Scan(&slug); e != nil {
			rows.Close()
			return nil, e
		}
		slugs = append(slugs, slug)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	var out []*model.Project
	for _, slug := range slugs {
		d, e := s.openProject(slug)
		if e != nil {
			return nil, e
		}
		p, e := scanProject(d.db.QueryRow(`SELECT `+projectColumns+` FROM projects WHERE slug=?`, slug))
		d.Close()
		if e != nil {
			return nil, e
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
func (d *projectDB) listProjectsChecked() ([]*model.Project, error) {
	rows, e := d.db.Query(`SELECT ` + projectColumns + ` FROM projects ORDER BY name,slug`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []*model.Project
	for rows.Next() {
		p, e := scanProject(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
