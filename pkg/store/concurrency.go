package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/altenwald/backlog/pkg/model"
	"reflect"
	"strings"
	"time"
)

var ErrAlreadyApplied = errors.New("request already applied; reload the current state")
var ErrStale = errors.New("stale: read the current state and apply your change again")
var ErrClosed = errors.New("project is closed to remote access")

type Precondition struct {
	RequestID string    `json:"-"`
	Slug      string    `json:"slug"`
	Kind      string    `json:"kind"`
	ID        string    `json:"id"`
	UpdatedAt time.Time `json:"updated_at"`
	Remote    bool      `json:"-"`
}

func checkPrecondition(q querier, p Precondition) error {
	if p.Remote {
		var open bool
		if e := q.QueryRow(`SELECT open FROM projects WHERE slug=?`, p.Slug).Scan(&open); e != nil {
			return e
		}
		if !open {
			return ErrClosed
		}
	}
	if p.UpdatedAt.IsZero() && !(p.Kind == "spec" && p.ID == model.SpecMainID) {
		return ErrStale
	}
	var v string
	var e error
	switch p.Kind {
	case "task":
		e = q.QueryRow(`SELECT updated_at FROM tasks WHERE project_slug=? AND id=?`, p.Slug, p.ID).Scan(&v)
	case "spec":
		e = q.QueryRow(`SELECT updated_at FROM spec_sections WHERE project_slug=? AND id=?`, p.Slug, p.ID).Scan(&v)
	default:
		e = q.QueryRow(`SELECT updated_at FROM projects WHERE slug=?`, p.Slug).Scan(&v)
	}
	if errors.Is(e, sql.ErrNoRows) && p.Kind == "spec" && p.ID == model.SpecMainID && p.UpdatedAt.IsZero() {
		return nil
	}
	if e != nil || !parseTime(v).Equal(p.UpdatedAt) {
		return ErrStale
	}
	return nil
}

// RemoteCall exposes only project operations, never settings, catalog writes or
// local files. Preconditions are checked inside the SQL write transaction.
func (s *Store) RemoteCall(method string, args []json.RawMessage, pre Precondition) (any, error) {
	reads := map[string]bool{"GetProject": true, "ListTasks": true, "GetSummary": true, "GetTopPriorities": true, "ListSpecSections": true, "GetSpecSections": true, "GetProjectSpecification": true, "ExportProjectJSON": true, "ExportProjectMarkdown": true}
	writes := map[string]string{"AddTask": "project", "UpdateTask": "task", "CompleteTask": "task", "AssignTask": "task", "DeleteTask": "task", "DeprecateTask": "task", "UpdateProjectSpecification": "project", "AddSpecSection": "project", "UpdateSpecSection": "spec", "DeleteSpecSection": "spec", "MoveSpecSection": "project"}
	kind, write := writes[method]
	if !reads[method] && !write {
		return nil, errors.New("operation not available remotely")
	}
	if len(args) == 0 {
		return nil, errors.New("missing project")
	}
	var slug string
	if e := json.Unmarshal(args[0], &slug); e != nil || slug == "" {
		return nil, errors.New("missing project")
	}
	d, e := s.openProject(slug)
	if e != nil {
		return nil, e
	}
	defer d.Close()
	var open bool
	if e = d.db.QueryRow(`SELECT open FROM projects WHERE slug=?`, slug).Scan(&open); e != nil {
		return nil, e
	}
	if !open {
		return nil, ErrClosed
	}
	if write {
		pre.Slug = slug
		pre.Remote = true
		pre.Kind = kind
		if kind == "task" || kind == "spec" {
			if len(args) < 2 {
				return nil, errors.New("missing entity")
			}
			if method == "UpdateTask" {
				var u struct {
					ID string `json:"id"`
				}
				if e = json.Unmarshal(args[1], &u); e != nil {
					return nil, e
				}
				pre.ID = u.ID
			} else {
				if e = json.Unmarshal(args[1], &pre.ID); e != nil {
					return nil, e
				}
			}
		}
		d.expected = &pre
	}
	fn := reflect.ValueOf(d).MethodByName(method)
	typ := fn.Type()
	if len(args) != typ.NumIn() {
		return nil, fmt.Errorf("invalid arguments for %s", method)
	}
	values := make([]reflect.Value, len(args))
	for i, arg := range args {
		v := reflect.New(typ.In(i))
		if e = json.Unmarshal(arg, v.Interface()); e != nil {
			return nil, e
		}
		values[i] = v.Elem()
	}
	var result []reflect.Value
	if typ.IsVariadic() {
		result = fn.CallSlice(values)
	} else {
		result = fn.Call(values)
	}
	last := result[len(result)-1]
	if !last.IsNil() {
		return nil, last.Interface().(error)
	}
	if len(result) > 1 {
		return result[0].Interface(), nil
	}
	return nil, nil
}
func monotonicTime(previous time.Time) time.Time {
	n := time.Now()
	if !n.After(previous) {
		return previous.Add(time.Nanosecond)
	}
	return n
}
func touchMonotonic(q querier, slug string, now time.Time) error {
	var old string
	if e := q.QueryRow(`SELECT updated_at FROM projects WHERE slug=?`, slug).Scan(&old); e != nil {
		return e
	}
	if prev := parseTime(old); !now.After(prev) {
		now = prev.Add(time.Nanosecond)
	}
	_, e := q.Exec(`UPDATE projects SET updated_at=? WHERE slug=?`, formatTime(now), slug)
	return e
}

// SetLocalSetting stores machine preferences and pairing material in the catalog.
func (s *Store) SetLocalSetting(key, value string) error {
	if !strings.HasPrefix(key, "network.") {
		return errors.New("invalid settings key")
	}
	return setSetting(s.db, key, value)
}
func (s *Store) LocalSetting(key string) string { v, _ := getSetting(s.db, key); return v }

// RemoteSnapshot reads permission and content from one consistent snapshot.
func (s *Store) RemoteSnapshot(slug string) (*model.Project, error) {
	d, e := s.openProject(slug)
	if e != nil {
		return nil, e
	}
	defer d.Close()
	tx, e := d.db.Begin()
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	p, e := readFullProject(tx, slug)
	if e != nil {
		return nil, e
	}
	if !p.Open {
		return nil, ErrClosed
	}
	return p, nil
}
