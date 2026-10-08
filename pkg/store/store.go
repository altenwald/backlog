package store

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/altenwald/backlog/pkg/model"
)

type EventType string

const (
	EventTaskCreated     EventType = "task_created"
	EventTaskUpdated     EventType = "task_updated"
	EventTaskCompleted   EventType = "task_completed"
	EventTaskDeleted     EventType = "task_deleted"
	EventProjectCreated  EventType = "project_created"
	EventProjectUpdated  EventType = "project_updated"
	EventProjectSelected EventType = "project_selected"
	EventProjectDeleted  EventType = "project_deleted"
)

type Event struct {
	Source      string         `json:"source,omitempty"`
	Type        EventType      `json:"type"`
	ProjectSlug string         `json:"project_slug"`
	TaskID      string         `json:"task_id,omitempty"`
	SectionID   string         `json:"section_id,omitempty"`
	Summary     *model.Summary `json:"summary,omitempty"`
}

const (
	settingActiveProject       = "active_project"
	settingMCPUserInstructions = "mcp_user_instructions"
)

// projectDB implements operations on a single project database. Writes are serialized by mu so
// that validation and the write it guards run as one unit; every write is
// also a single SQLite transaction.
type projectDB struct {
	mu       sync.RWMutex
	db       *sql.DB
	onEvent  func(Event)
	expected *Precondition
}

// Close releases the database handle.
func (s *projectDB) Close() error {
	return s.db.Close()
}

func (s *projectDB) notify(ev Event) {
	if s.onEvent != nil {
		s.onEvent(ev)
	}
}

type querier interface {
	QueryRow(query string, args ...any) *sql.Row
	Query(query string, args ...any) (*sql.Rows, error)
	Exec(query string, args ...any) (sql.Result, error)
}

func getSetting(q querier, key string) (string, error) {
	var v string
	err := q.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return v, err
}

func setSetting(q querier, key, value string) error {
	_, err := q.Exec(`INSERT INTO settings(key, value) VALUES(?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// activeProject returns the configured active project, falling back to the
// first project by slug when none is set or the configured one is gone.
func activeProject(q querier) string {
	slug, _ := getSetting(q, settingActiveProject)
	if slug != "" {
		var exists int
		if q.QueryRow(`SELECT 1 FROM projects WHERE slug = ?`, slug).Scan(&exists) == nil {
			return slug
		}
	}
	var first string
	_ = q.QueryRow(`SELECT slug FROM projects ORDER BY slug LIMIT 1`).Scan(&first)
	return first
}

func (s *projectDB) resolveSlug(q querier, slug string) string {
	if slug == "" {
		return activeProject(q)
	}
	return strings.ToLower(slug)
}

func projectExists(q querier, slug string) error {
	var one int
	err := q.QueryRow(`SELECT 1 FROM projects WHERE slug = ?`, slug).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("project '%s' not found", slug)
	}
	return err
}

// withTx runs fn inside a write transaction while holding the write lock.
func (s *projectDB) withTx(fn func(tx *sql.Tx) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if s.expected != nil {
		if s.expected.RequestID != "" {
			var one int
			if e := tx.QueryRow(`SELECT 1 FROM remote_receipts WHERE request_id=?`, s.expected.RequestID).Scan(&one); e == nil {
				tx.Rollback()
				return ErrAlreadyApplied
			} else if !errors.Is(e, sql.ErrNoRows) {
				tx.Rollback()
				return e
			}
		}
		if err := checkPrecondition(tx, *s.expected); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if s.expected != nil && s.expected.RequestID != "" {
		if _, e := tx.Exec(`INSERT INTO remote_receipts(request_id,applied_at) VALUES(?,?)`, s.expected.RequestID, formatTime(time.Now())); e != nil {
			tx.Rollback()
			return e
		}
	}
	return tx.Commit()
}

func (s *projectDB) GetActiveProjectSlug() string {
	return activeProject(s.db)
}

func (s *projectDB) SetActiveProject(slug string) error {
	slug = strings.ToLower(slug)
	changed := false
	err := s.withTx(func(tx *sql.Tx) error {
		if err := projectExists(tx, slug); err != nil {
			return err
		}
		current, err := getSetting(tx, settingActiveProject)
		if err != nil {
			return err
		}
		if current == slug {
			return nil
		}
		changed = true
		return setSetting(tx, settingActiveProject, slug)
	})
	if err != nil || !changed {
		return err
	}

	go s.notify(Event{
		Type:        EventProjectSelected,
		ProjectSlug: slug,
	})
	return nil
}

const projectColumns = `slug, name, description, inserted_at, updated_at, open`

func scanProject(row interface{ Scan(...any) error }) (*model.Project, error) {
	var p model.Project
	var inserted, updated string
	if err := row.Scan(&p.Slug, &p.Name, &p.Description, &inserted, &updated, &p.Open); err != nil {
		return nil, err
	}
	p.InsertedAt = parseTime(inserted)
	p.UpdatedAt = parseTime(updated)
	p.Tasks = []model.Task{}
	return &p, nil
}

// ListProjects returns project metadata sorted by name. Tasks and the
// specification are not loaded; use GetProject for a single full project.
func (s *projectDB) ListProjects() []*model.Project {
	rows, err := s.db.Query(`SELECT ` + projectColumns + ` FROM projects ORDER BY name, slug`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var result []*model.Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return result
		}
		result = append(result, p)
	}
	return result
}

// GetProject returns a project with all its tasks. The specification is not
// loaded; read it with GetProjectSpecification or GetSpecSections.
func (s *projectDB) GetProject(slug string) (*model.Project, error) {
	slug = s.resolveSlug(s.db, slug)

	p, err := scanProject(s.db.QueryRow(`SELECT `+projectColumns+` FROM projects WHERE slug = ?`, slug))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("project '%s' not found", slug)
	}
	if err != nil {
		return nil, err
	}

	tasks, err := loadTasks(s.db, slug, "", nil)
	if err != nil {
		return nil, err
	}
	p.Tasks = tasks
	return p, nil
}

func (s *projectDB) CreateProject(slug, name, description string) (*model.Project, error) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if slug == "" {
		return nil, errors.New("project slug cannot be empty")
	}
	if name == "" {
		name = strings.Title(slug)
	}

	now := time.Now()
	p := &model.Project{
		Slug:        slug,
		Name:        name,
		Description: description,
		Tasks:       []model.Task{},
		InsertedAt:  now,
		UpdatedAt:   now,
	}

	err := s.withTx(func(tx *sql.Tx) error {
		if projectExists(tx, slug) == nil {
			return fmt.Errorf("project '%s' already exists", slug)
		}
		if err := insertProject(tx, p, 1); err != nil {
			return err
		}
		current, err := getSetting(tx, settingActiveProject)
		if err != nil {
			return err
		}
		if current == "" {
			return setSetting(tx, settingActiveProject, slug)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	go s.notify(Event{
		Type:        EventProjectCreated,
		ProjectSlug: slug,
	})
	return p, nil
}

func insertProject(q querier, p *model.Project, nextTaskID int) error {
	_, err := q.Exec(`INSERT INTO projects(slug, name, description, next_task_id, inserted_at, updated_at, open)
		VALUES(?, ?, ?, ?, ?, ?, ?)`,
		p.Slug, p.Name, p.Description, nextTaskID, formatTime(p.InsertedAt), formatTime(p.UpdatedAt), p.Open)
	return err
}

func (s *projectDB) DeleteProject(slug string) error {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if slug == "" {
		return errors.New("project slug cannot be empty")
	}

	err := s.withTx(func(tx *sql.Tx) error {
		if err := projectExists(tx, slug); err != nil {
			return err
		}
		// Children are deleted by cascade.
		if _, err := tx.Exec(`DELETE FROM projects WHERE slug = ?`, slug); err != nil {
			return err
		}
		current, err := getSetting(tx, settingActiveProject)
		if err != nil {
			return err
		}
		if current == slug {
			var next string
			_ = tx.QueryRow(`SELECT slug FROM projects ORDER BY slug LIMIT 1`).Scan(&next)
			return setSetting(tx, settingActiveProject, next)
		}
		return nil
	})
	if err != nil {
		return err
	}

	go s.notify(Event{
		Type:        EventProjectDeleted,
		ProjectSlug: slug,
	})
	return nil
}

const taskColumns = `id, parent_id, title, description, size, tier, done, deprecated,
	assignee, resolution, inserted_at, updated_at, terminated_at`

func scanTask(row interface{ Scan(...any) error }) (model.Task, error) {
	var t model.Task
	var parent, terminated sql.NullString
	var inserted, updated, size string
	var tier int
	var done, deprecated int
	if err := row.Scan(&t.ID, &parent, &t.Title, &t.Description, &size, &tier, &done, &deprecated,
		&t.Assignee, &t.Resolution, &inserted, &updated, &terminated); err != nil {
		return t, err
	}
	t.ParentID = parent.String
	t.Size = model.Size(size)
	t.Tier = model.Tier(tier)
	t.Done = done != 0
	t.Deprecated = deprecated != 0
	t.InsertedAt = parseTime(inserted)
	t.UpdatedAt = parseTime(updated)
	if terminated.Valid {
		ts := parseTime(terminated.String)
		t.TerminatedAt = &ts
	}
	return t, nil
}

// loadTasks returns the project's tasks in insertion order, optionally
// narrowed by an extra SQL condition on the tasks table (aliased t).
func loadTasks(q querier, slug, where string, args []any) ([]model.Task, error) {
	query := `SELECT ` + taskColumns + ` FROM tasks t WHERE t.project_slug = ?`
	if where != "" {
		query += ` AND ` + where
	}
	query += ` ORDER BY t.rowid`

	rows, err := q.Query(query, append([]any{slug}, args...)...)
	if err != nil {
		return nil, err
	}
	var tasks []model.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		tasks = append(tasks, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	deps, err := loadDeps(q, slug)
	if err != nil {
		return nil, err
	}
	for i := range tasks {
		tasks[i].DependsOn = deps[tasks[i].ID]
	}
	return tasks, nil
}

func loadDeps(q querier, slug string) (map[string][]string, error) {
	rows, err := q.Query(`SELECT task_id, depends_on FROM task_deps WHERE project_slug = ? ORDER BY rowid`, slug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	deps := make(map[string][]string)
	for rows.Next() {
		var id, dep string
		if err := rows.Scan(&id, &dep); err != nil {
			return nil, err
		}
		deps[id] = append(deps[id], dep)
	}
	return deps, rows.Err()
}

func getTask(q querier, slug, id string) (model.Task, error) {
	t, err := scanTask(q.QueryRow(`SELECT `+taskColumns+` FROM tasks WHERE project_slug = ? AND id = ?`, slug, id))
	if errors.Is(err, sql.ErrNoRows) {
		return t, fmt.Errorf("task ID '%s' not found in project '%s'", id, slug)
	}
	if err != nil {
		return t, err
	}
	rows, err := q.Query(`SELECT depends_on FROM task_deps WHERE project_slug = ? AND task_id = ? ORDER BY rowid`, slug, id)
	if err != nil {
		return t, err
	}
	defer rows.Close()
	for rows.Next() {
		var dep string
		if err := rows.Scan(&dep); err != nil {
			return t, err
		}
		t.DependsOn = append(t.DependsOn, dep)
	}
	return t, rows.Err()
}

func taskExists(q querier, slug, id string) bool {
	var one int
	return q.QueryRow(`SELECT 1 FROM tasks WHERE project_slug = ? AND id = ?`, slug, id).Scan(&one) == nil
}

func insertTask(q querier, slug string, t model.Task) error {
	var terminated any
	if t.TerminatedAt != nil {
		terminated = formatTime(*t.TerminatedAt)
	}
	_, err := q.Exec(`INSERT INTO tasks(project_slug, id, parent_id, title, description, size, tier, done,
		deprecated, assignee, resolution, inserted_at, updated_at, terminated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		slug, t.ID, nullString(t.ParentID), t.Title, t.Description, string(t.Size), int(t.Tier),
		boolInt(t.Done), boolInt(t.Deprecated), t.Assignee, t.Resolution,
		formatTime(t.InsertedAt), formatTime(t.UpdatedAt), terminated)
	if err != nil {
		return err
	}
	return replaceDeps(q, slug, t.ID, t.DependsOn)
}

func updateTaskRow(q querier, slug string, t model.Task) error {
	var terminated any
	if t.TerminatedAt != nil {
		terminated = formatTime(*t.TerminatedAt)
	}
	_, err := q.Exec(`UPDATE tasks SET parent_id = ?, title = ?, description = ?, size = ?, tier = ?,
		done = ?, deprecated = ?, assignee = ?, resolution = ?, updated_at = ?, terminated_at = ?
		WHERE project_slug = ? AND id = ?`,
		nullString(t.ParentID), t.Title, t.Description, string(t.Size), int(t.Tier),
		boolInt(t.Done), boolInt(t.Deprecated), t.Assignee, t.Resolution,
		formatTime(t.UpdatedAt), terminated, slug, t.ID)
	return err
}

func replaceDeps(q querier, slug, id string, deps []string) error {
	if _, err := q.Exec(`DELETE FROM task_deps WHERE project_slug = ? AND task_id = ?`, slug, id); err != nil {
		return err
	}
	for _, d := range deps {
		if _, err := q.Exec(`INSERT INTO task_deps(project_slug, task_id, depends_on) VALUES(?, ?, ?)`, slug, id, d); err != nil {
			return err
		}
	}
	return nil
}

func touchProject(q querier, slug string, now time.Time) error {
	return touchMonotonic(q, slug, now)
}

func (s *projectDB) ListTasks(projectSlug string, filter model.TaskFilter) ([]model.Task, error) {
	projectSlug = s.resolveSlug(s.db, projectSlug)
	if err := projectExists(s.db, projectSlug); err != nil {
		return nil, err
	}

	var conds []string
	var args []any
	if filter.Tier != nil {
		conds = append(conds, `t.tier = ?`)
		args = append(args, int(*filter.Tier))
	}
	if filter.ParentID != nil {
		conds = append(conds, `COALESCE(t.parent_id, '') = ?`)
		args = append(args, *filter.ParentID)
	}
	if filter.DependsOn != nil && *filter.DependsOn != "" {
		conds = append(conds, `EXISTS (SELECT 1 FROM task_deps d
			WHERE d.project_slug = t.project_slug AND d.task_id = t.id AND d.depends_on = ?)`)
		args = append(args, *filter.DependsOn)
	}
	if filter.Blocked != nil {
		conds = append(conds, `EXISTS (SELECT 1 FROM task_deps d
			JOIN tasks x ON x.project_slug = d.project_slug AND x.id = d.depends_on
			WHERE d.project_slug = t.project_slug AND d.task_id = t.id AND x.done = 0) = ?`)
		args = append(args, boolInt(*filter.Blocked))
	}
	if filter.Size != nil {
		conds = append(conds, `t.size = ?`)
		args = append(args, string(*filter.Size))
	}
	if filter.Done != nil {
		conds = append(conds, `t.done = ?`)
		args = append(args, boolInt(*filter.Done))
	}
	if filter.Deprecated != nil {
		conds = append(conds, `t.deprecated = ?`)
		args = append(args, boolInt(*filter.Deprecated))
	}

	tasks, err := loadTasks(s.db, projectSlug, strings.Join(conds, ` AND `), args)
	if err != nil {
		return nil, err
	}

	// Assignee and free-text matching stay in Go: SQLite's lower() and LIKE
	// only fold ASCII, and descriptions are often written in Spanish.
	var reqAssignee string
	if filter.Assignee != nil {
		reqAssignee = strings.ToLower(strings.TrimPrefix(*filter.Assignee, "@"))
	}
	searchLower := strings.ToLower(strings.TrimSpace(filter.Search))

	var results []model.Task
	for _, task := range tasks {
		if reqAssignee != "" {
			taskAssignee := strings.ToLower(strings.TrimPrefix(task.Assignee, "@"))
			if reqAssignee == "unassigned" {
				if task.Assignee != "" {
					continue
				}
			} else if taskAssignee != reqAssignee {
				continue
			}
		}
		if searchLower != "" {
			inTitle := strings.Contains(strings.ToLower(task.Title), searchLower)
			inDesc := strings.Contains(strings.ToLower(task.Description), searchLower)
			inAssignee := strings.Contains(strings.ToLower(task.Assignee), searchLower)
			inID := task.ID == searchLower || task.ParentID == searchLower
			if !inTitle && !inDesc && !inAssignee && !inID {
				continue
			}
		}
		results = append(results, task)
	}

	return results, nil
}

func (s *projectDB) GetTopPriorities(projectSlug string, limit int) ([]model.Task, error) {
	if limit <= 0 {
		limit = 5
	}
	doneFalse := false
	tasks, err := s.ListTasks(projectSlug, model.TaskFilter{Done: &doneFalse})
	if err != nil {
		return nil, err
	}

	taskMap := make(map[string]model.Task, len(tasks))
	for _, t := range tasks {
		taskMap[t.ID] = t
	}

	// Sort by: 1) Unblocked first (actionable), 2) Tier (ascending: T1 first), 3) Size weight (descending)
	sort.SliceStable(tasks, func(i, j int) bool {
		blockedI := tasks[i].IsBlocked(taskMap)
		blockedJ := tasks[j].IsBlocked(taskMap)
		if blockedI != blockedJ {
			return !blockedI
		}
		if tasks[i].Tier != tasks[j].Tier {
			return tasks[i].Tier < tasks[j].Tier
		}
		return tasks[i].Size.Weight() > tasks[j].Size.Weight()
	})

	if len(tasks) > limit {
		tasks = tasks[:limit]
	}
	return tasks, nil
}

func (s *projectDB) GetSummary(projectSlug string) (*model.Summary, error) {
	projectSlug = s.resolveSlug(s.db, projectSlug)

	var name string
	err := s.db.QueryRow(`SELECT name FROM projects WHERE slug = ?`, projectSlug).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("project '%s' not found", projectSlug)
	}
	if err != nil {
		return nil, err
	}

	summary := &model.Summary{
		ProjectSlug:     projectSlug,
		ProjectName:     name,
		SizeCounts:      make(map[model.Size]int),
		OpenSizeCounts:  make(map[model.Size]int),
		TierCounts:      make(map[model.Tier]int),
		TotalTierCounts: make(map[model.Tier]int),
	}

	rows, err := s.db.Query(`SELECT size, tier, done, COUNT(*) FROM tasks
		WHERE project_slug = ? GROUP BY size, tier, done`, projectSlug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var size string
		var tier, done, n int
		if err := rows.Scan(&size, &tier, &done, &n); err != nil {
			return nil, err
		}
		sz, tr := model.Size(size), model.Tier(tier)
		summary.TotalTasks += n
		summary.SizeCounts[sz] += n
		summary.TotalTierCounts[tr] += n
		if done != 0 {
			summary.CompletedTasks += n
		} else {
			summary.OpenTasks += n
			summary.OpenSizeCounts[sz] += n
			summary.TierCounts[tr] += n
		}
	}
	return summary, rows.Err()
}

func cleanDependsOn(q querier, slug, selfID string, deps []string) ([]string, error) {
	var clean []string
	seen := make(map[string]bool)
	for _, depID := range deps {
		depID = strings.TrimSpace(depID)
		if depID == "" || seen[depID] {
			continue
		}
		if selfID != "" && depID == selfID {
			return nil, errors.New("a task cannot depend on itself")
		}
		seen[depID] = true
		if !taskExists(q, slug, depID) {
			return nil, fmt.Errorf("dependency task #%s not found in project '%s'", depID, slug)
		}
		clean = append(clean, depID)
	}
	return clean, nil
}

func (s *projectDB) AddTask(projectSlug string, task model.Task) (*model.Task, error) {
	if task.Title == "" {
		return nil, errors.New("task title is required")
	}
	if task.Size == "" {
		task.Size = model.SizeM
	}
	if task.Tier <= 0 || task.Tier > 5 {
		task.Tier = model.Tier3
	}

	err := s.withTx(func(tx *sql.Tx) error {
		projectSlug = s.resolveSlug(tx, projectSlug)
		var nextID int
		err := tx.QueryRow(`SELECT next_task_id FROM projects WHERE slug = ?`, projectSlug).Scan(&nextID)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("project '%s' not found", projectSlug)
		}
		if err != nil {
			return err
		}

		if task.ParentID != "" && !taskExists(tx, projectSlug, task.ParentID) {
			return fmt.Errorf("parent task #%s not found in project '%s'", task.ParentID, projectSlug)
		}

		deps, err := cleanDependsOn(tx, projectSlug, "", task.DependsOn)
		if err != nil {
			return err
		}
		task.DependsOn = deps

		now := time.Now()
		task.InsertedAt = now
		task.UpdatedAt = now

		// IDs are never reused, even after the highest task is deleted.
		if task.ID == "" {
			for taskExists(tx, projectSlug, strconv.Itoa(nextID)) {
				nextID++
			}
			task.ID = strconv.Itoa(nextID)
		} else if taskExists(tx, projectSlug, task.ID) {
			return fmt.Errorf("task ID '%s' already exists in project '%s'", task.ID, projectSlug)
		}
		if n, err := strconv.Atoi(task.ID); err == nil && n >= nextID {
			nextID = n + 1
		}

		if err := insertTask(tx, projectSlug, task); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE projects SET next_task_id = ?, updated_at = ? WHERE slug = ?`,
			nextID, formatTime(now), projectSlug); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	go s.notify(Event{
		Type:        EventTaskCreated,
		ProjectSlug: projectSlug,
		TaskID:      task.ID,
	})
	return &task, nil
}

// mutateTask loads a task, applies fn and writes the task back, all in one
// transaction. fn may return an error to abort without changes.
func (s *projectDB) mutateTask(projectSlug, taskID string, fn func(tx *sql.Tx, slug string, t *model.Task) error) (string, *model.Task, error) {
	var result model.Task
	err := s.withTx(func(tx *sql.Tx) error {
		projectSlug = s.resolveSlug(tx, projectSlug)
		if err := projectExists(tx, projectSlug); err != nil {
			return err
		}
		t, err := getTask(tx, projectSlug, taskID)
		if err != nil {
			return err
		}
		previous := t.UpdatedAt
		if err := fn(tx, projectSlug, &t); err != nil {
			return err
		}
		if !t.UpdatedAt.After(previous) {
			t.UpdatedAt = previous.Add(time.Nanosecond)
		}
		if err := updateTaskRow(tx, projectSlug, t); err != nil {
			return err
		}
		if err := touchProject(tx, projectSlug, t.UpdatedAt); err != nil {
			return err
		}
		result = t
		return nil
	})
	if err != nil {
		return projectSlug, nil, err
	}
	return projectSlug, &result, nil
}

func (s *projectDB) CompleteTask(projectSlug string, taskID string, done bool, resolution ...string) (*model.Task, error) {
	slug, task, err := s.mutateTask(projectSlug, taskID, func(_ *sql.Tx, _ string, t *model.Task) error {
		t.Done = done
		now := time.Now()
		t.UpdatedAt = now
		if done {
			t.TerminatedAt = &now
			if len(resolution) > 0 && resolution[0] != "" {
				t.Resolution = resolution[0]
			}
		} else {
			t.TerminatedAt = nil
			t.Deprecated = false
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	go s.notify(Event{
		Type:        EventTaskCompleted,
		ProjectSlug: slug,
		TaskID:      taskID,
	})
	return task, nil
}

func (s *projectDB) UpdateTask(projectSlug string, upd model.TaskUpdate) (*model.Task, error) {
	slug, task, err := s.mutateTask(projectSlug, upd.ID, func(tx *sql.Tx, slug string, t *model.Task) error {
		if upd.ExpectedUpdatedAt != nil && !t.UpdatedAt.Equal(*upd.ExpectedUpdatedAt) {
			return ErrStale
		}
		if upd.Done != nil {
			t.Done = *upd.Done
			if t.Done {
				now := time.Now()
				t.TerminatedAt = &now
			} else {
				t.TerminatedAt = nil
			}
		}
		if upd.Deprecated != nil {
			t.Deprecated = *upd.Deprecated
		}

		if upd.Title != "" {
			t.Title = upd.Title
		}
		if upd.Description != nil {
			t.Description = *upd.Description
		}
		if upd.ParentID != "" {
			if upd.ParentID == upd.ID {
				return errors.New("a task cannot be its own parent")
			}
			if upd.ParentID == "none" || upd.ParentID == "0" {
				t.ParentID = ""
			} else {
				if !taskExists(tx, slug, upd.ParentID) {
					return fmt.Errorf("parent task #%s not found in project '%s'", upd.ParentID, slug)
				}
				// Cycle check: walk up from the new parent.
				curr := upd.ParentID
				for curr != "" {
					if curr == upd.ID {
						return errors.New("cannot set parent: circular dependency detected")
					}
					var next sql.NullString
					if err := tx.QueryRow(`SELECT parent_id FROM tasks WHERE project_slug = ? AND id = ?`,
						slug, curr).Scan(&next); err != nil && !errors.Is(err, sql.ErrNoRows) {
						return err
					}
					curr = next.String
				}
				t.ParentID = upd.ParentID
			}
		}
		if upd.Size != "" {
			t.Size = upd.Size
		}
		if upd.DependsOn != nil {
			deps, err := cleanDependsOn(tx, slug, upd.ID, upd.DependsOn)
			if err != nil {
				return err
			}

			depMap, err := loadDeps(tx, slug)
			if err != nil {
				return err
			}
			depMap[upd.ID] = deps

			visited := make(map[string]bool)
			recStack := make(map[string]bool)
			var hasCycle func(curr string) bool
			hasCycle = func(curr string) bool {
				visited[curr] = true
				recStack[curr] = true
				for _, neighbor := range depMap[curr] {
					if !visited[neighbor] {
						if hasCycle(neighbor) {
							return true
						}
					} else if recStack[neighbor] {
						return true
					}
				}
				recStack[curr] = false
				return false
			}
			if hasCycle(upd.ID) {
				return errors.New("cannot set dependency: circular dependency detected")
			}

			t.DependsOn = deps
			if err := replaceDeps(tx, slug, upd.ID, deps); err != nil {
				return err
			}
		}
		if upd.Tier > 0 {
			t.Tier = upd.Tier
		}
		if upd.Resolution != "" {
			t.Resolution = upd.Resolution
		}
		if upd.Assignee != "" {
			t.Assignee = upd.Assignee
		}
		t.UpdatedAt = time.Now()
		return nil
	})
	if err != nil {
		return nil, err
	}

	go s.notify(Event{
		Type:        EventTaskUpdated,
		ProjectSlug: slug,
		TaskID:      upd.ID,
	})
	return task, nil
}

func (s *projectDB) AssignTask(projectSlug string, taskID string, assignee string) (*model.Task, error) {
	slug, task, err := s.mutateTask(projectSlug, taskID, func(_ *sql.Tx, _ string, t *model.Task) error {
		t.Assignee = strings.TrimSpace(assignee)
		t.UpdatedAt = time.Now()
		return nil
	})
	if err != nil {
		return nil, err
	}

	go s.notify(Event{
		Type:        EventTaskUpdated,
		ProjectSlug: slug,
		TaskID:      taskID,
	})
	return task, nil
}

func (s *projectDB) DeleteTask(projectSlug string, taskID string, expected ...time.Time) error {
	err := s.withTx(func(tx *sql.Tx) error {
		projectSlug = s.resolveSlug(tx, projectSlug)
		if len(expected) > 0 {
			if e := checkPrecondition(tx, Precondition{Slug: projectSlug, Kind: "task", ID: taskID, UpdatedAt: expected[0]}); e != nil {
				return e
			}
		}
		if err := projectExists(tx, projectSlug); err != nil {
			return err
		}
		if !taskExists(tx, projectSlug, taskID) {
			return fmt.Errorf("task ID '%s' not found in project '%s'", taskID, projectSlug)
		}
		// Cascading edge removals also change the surviving dependent tasks.
		tasks, e := loadTasks(tx, projectSlug, "", nil)
		if e != nil {
			return e
		}
		doomed := map[string]bool{taskID: true}
		changed := true
		for changed {
			changed = false
			for _, t := range tasks {
				if !doomed[t.ID] && doomed[t.ParentID] {
					doomed[t.ID] = true
					changed = true
				}
			}
		}
		for _, t := range tasks {
			if doomed[t.ID] {
				continue
			}
			deps := []string{}
			for _, id := range t.DependsOn {
				if !doomed[id] {
					deps = append(deps, id)
				}
			}
			if len(deps) != len(t.DependsOn) {
				t.DependsOn = deps
				t.UpdatedAt = monotonicTime(t.UpdatedAt)
				if e := updateTaskRow(tx, projectSlug, t); e != nil {
					return e
				}
			}
		}
		// Subtasks and dependency edges go by cascade.
		if _, err := tx.Exec(`DELETE FROM tasks WHERE project_slug = ? AND id = ?`, projectSlug, taskID); err != nil {
			return err
		}
		return touchProject(tx, projectSlug, time.Now())
	})
	if err != nil {
		return err
	}

	go s.notify(Event{
		Type:        EventTaskDeleted,
		ProjectSlug: projectSlug,
		TaskID:      taskID,
	})
	return nil
}

// GetMCPUserInstructions returns the user-defined portion of MCP instructions.
// Returns an empty string when the user has not set custom instructions yet.
func (s *projectDB) GetMCPUserInstructions() string {
	v, _ := getSetting(s.db, settingMCPUserInstructions)
	return v
}

// SaveMCPUserInstructions persists the user-defined portion of MCP instructions.
func (s *projectDB) SaveMCPUserInstructions(instructions string) error {
	return s.withTx(func(tx *sql.Tx) error {
		return setSetting(tx, settingMCPUserInstructions, instructions)
	})
}

// DeprecateTask marks a task as deprecated and completed (or undeprecates it).
func (s *projectDB) DeprecateTask(projectSlug string, taskID string, deprecated bool) (*model.Task, error) {
	slug, task, err := s.mutateTask(projectSlug, taskID, func(_ *sql.Tx, _ string, t *model.Task) error {
		t.Deprecated = deprecated
		now := time.Now()
		t.UpdatedAt = now
		if deprecated {
			t.Done = true
			if t.TerminatedAt == nil {
				t.TerminatedAt = &now
			}
			if t.Resolution == "" {
				t.Resolution = "Deprecated: no longer applicable according to project specification."
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	go s.notify(Event{
		Type:        EventTaskUpdated,
		ProjectSlug: slug,
		TaskID:      taskID,
	})
	return task, nil
}
