package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	EventProjectSelected EventType = "project_selected"
	EventProjectDeleted  EventType = "project_deleted"
)

type Event struct {
	Type        EventType      `json:"type"`
	ProjectSlug string         `json:"project_slug"`
	TaskID      string         `json:"task_id,omitempty"`
	Summary     *model.Summary `json:"summary,omitempty"`
}

type Config struct {
	ActiveProject string `json:"active_project"`
}

type Store struct {
	mu          sync.RWMutex
	dataDir     string
	config      Config
	projects    map[string]*model.Project
	subscribers []chan Event
}

func NewStore(dataDir string) (*Store, error) {
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		dataDir = filepath.Join(home, ".config", "backlog")
	}

	projectsDir := filepath.Join(dataDir, "projects")
	if err := os.MkdirAll(projectsDir, 0755); err != nil {
		return nil, fmt.Errorf("cannot create projects dir: %w", err)
	}

	s := &Store{
		dataDir:  dataDir,
		projects: make(map[string]*model.Project),
	}

	if err := s.loadAll(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Store) GetDataDir() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dataDir
}

func (s *Store) loadAll() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Load config
	configPath := filepath.Join(s.dataDir, "config.json")
	if data, err := os.ReadFile(configPath); err == nil {
		_ = json.Unmarshal(data, &s.config)
	}

	// Load projects
	projectsDir := filepath.Join(s.dataDir, "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			filePath := filepath.Join(projectsDir, entry.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}
			var p model.Project
			if err := json.Unmarshal(data, &p); err == nil && p.Slug != "" {
				s.projects[p.Slug] = &p
			}
		}
	}

	if s.config.ActiveProject == "" && len(s.projects) > 0 {
		for slug := range s.projects {
			s.config.ActiveProject = slug
			break
		}
	}

	return nil
}

func (s *Store) saveConfig() error {
	configPath := filepath.Join(s.dataDir, "config.json")
	data, err := json.MarshalIndent(s.config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}

func (s *Store) saveProject(p *model.Project) error {
	projectsDir := filepath.Join(s.dataDir, "projects")
	filePath := filepath.Join(projectsDir, p.Slug+".json")
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0644)
}

func (s *Store) Subscribe() <-chan Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch := make(chan Event, 20)
	s.subscribers = append(s.subscribers, ch)
	return ch
}

func (s *Store) notify(ev Event) {
	for _, ch := range s.subscribers {
		select {
		case ch <- ev:
		default:
		}
	}
}

func (s *Store) GetActiveProjectSlug() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config.ActiveProject
}

func (s *Store) SetActiveProject(slug string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	slug = strings.ToLower(slug)
	if _, ok := s.projects[slug]; !ok {
		return fmt.Errorf("project '%s' not found", slug)
	}

	if s.config.ActiveProject == slug {
		return nil
	}

	s.config.ActiveProject = slug
	if err := s.saveConfig(); err != nil {
		return err
	}

	go s.notify(Event{
		Type:        EventProjectSelected,
		ProjectSlug: slug,
	})
	return nil
}

func (s *Store) ListProjects() []*model.Project {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*model.Project
	for _, p := range s.projects {
		copyProj := *p
		result = append(result, &copyProj)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

func (s *Store) GetProject(slug string) (*model.Project, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if slug == "" {
		slug = s.config.ActiveProject
	}
	slug = strings.ToLower(slug)

	p, ok := s.projects[slug]
	if !ok {
		return nil, fmt.Errorf("project '%s' not found", slug)
	}
	copyProj := *p
	return &copyProj, nil
}

func (s *Store) CreateProject(slug, name, description string) (*model.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	slug = strings.ToLower(strings.TrimSpace(slug))
	if slug == "" {
		return nil, errors.New("project slug cannot be empty")
	}
	if _, exists := s.projects[slug]; exists {
		return nil, fmt.Errorf("project '%s' already exists", slug)
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
	s.projects[slug] = p
	if s.config.ActiveProject == "" {
		s.config.ActiveProject = slug
		_ = s.saveConfig()
	}

	if err := s.saveProject(p); err != nil {
		return nil, err
	}

	go s.notify(Event{
		Type:        EventProjectCreated,
		ProjectSlug: slug,
	})
	return p, nil
}

func (s *Store) DeleteProject(slug string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	slug = strings.ToLower(strings.TrimSpace(slug))
	if slug == "" {
		return errors.New("project slug cannot be empty")
	}

	if _, exists := s.projects[slug]; !exists {
		return fmt.Errorf("project '%s' not found", slug)
	}

	delete(s.projects, slug)

	// Remove JSON file on disk
	filePath := filepath.Join(s.dataDir, "projects", slug+".json")
	_ = os.Remove(filePath)

	// If the deleted project was active, pick another project or clear
	if s.config.ActiveProject == slug {
		newActive := ""
		for otherSlug := range s.projects {
			newActive = otherSlug
			break
		}
		s.config.ActiveProject = newActive
		_ = s.saveConfig()
	}

	go s.notify(Event{
		Type:        EventProjectDeleted,
		ProjectSlug: slug,
	})
	return nil
}

func (s *Store) ListTasks(projectSlug string, filter model.TaskFilter) ([]model.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if projectSlug == "" {
		projectSlug = s.config.ActiveProject
	}
	projectSlug = strings.ToLower(projectSlug)

	p, ok := s.projects[projectSlug]
	if !ok {
		return nil, fmt.Errorf("project '%s' not found", projectSlug)
	}

	var results []model.Task
	searchLower := strings.ToLower(strings.TrimSpace(filter.Search))

	for _, task := range p.Tasks {
		if filter.Tier != nil && task.Tier != *filter.Tier {
			continue
		}
		if filter.ParentID != nil && task.ParentID != *filter.ParentID {
			continue
		}
		if filter.Size != nil && task.Size != *filter.Size {
			continue
		}
		if filter.Done != nil && task.Done != *filter.Done {
			continue
		}
		if filter.Assignee != nil && *filter.Assignee != "" {
			reqAssignee := strings.ToLower(strings.TrimPrefix(*filter.Assignee, "@"))
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
			inTag := strings.Contains(strings.ToLower(task.Tag), searchLower)
			inAssignee := strings.Contains(strings.ToLower(task.Assignee), searchLower)
			inID := task.ID == searchLower || task.ParentID == searchLower
			if !inTitle && !inDesc && !inTag && !inAssignee && !inID {
				continue
			}
		}
		results = append(results, task)
	}

	return results, nil
}

func (s *Store) GetTopPriorities(projectSlug string, limit int) ([]model.Task, error) {
	if limit <= 0 {
		limit = 5
	}
	doneFalse := false
	tasks, err := s.ListTasks(projectSlug, model.TaskFilter{Done: &doneFalse})
	if err != nil {
		return nil, err
	}

	// Sort by Tier (ascending: T1 first), then by size weight (descending)
	sort.Slice(tasks, func(i, j int) bool {
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

func (s *Store) GetSummary(projectSlug string) (*model.Summary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if projectSlug == "" {
		projectSlug = s.config.ActiveProject
	}
	projectSlug = strings.ToLower(projectSlug)

	p, ok := s.projects[projectSlug]
	if !ok {
		return nil, fmt.Errorf("project '%s' not found", projectSlug)
	}

	summary := &model.Summary{
		ProjectSlug:     p.Slug,
		ProjectName:     p.Name,
		SizeCounts:      make(map[model.Size]int),
		OpenSizeCounts:  make(map[model.Size]int),
		TierCounts:      make(map[model.Tier]int),
		TotalTierCounts: make(map[model.Tier]int),
	}

	for _, t := range p.Tasks {
		summary.TotalTasks++
		summary.SizeCounts[t.Size]++
		summary.TotalTierCounts[t.Tier]++

		if t.Done {
			summary.CompletedTasks++
		} else {
			summary.OpenTasks++
			summary.OpenSizeCounts[t.Size]++
			summary.TierCounts[t.Tier]++
		}
	}

	return summary, nil
}

func (s *Store) AddTask(projectSlug string, task model.Task) (*model.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if projectSlug == "" {
		projectSlug = s.config.ActiveProject
	}
	projectSlug = strings.ToLower(projectSlug)

	p, ok := s.projects[projectSlug]
	if !ok {
		return nil, fmt.Errorf("project '%s' not found", projectSlug)
	}

	if task.Title == "" {
		return nil, errors.New("task title is required")
	}
	if task.Size == "" {
		task.Size = model.SizeM
	}
	if task.Tier <= 0 || task.Tier > 5 {
		task.Tier = model.Tier3
	}

	if task.ParentID != "" {
		parentFound := false
		for _, existing := range p.Tasks {
			if existing.ID == task.ParentID {
				parentFound = true
				break
			}
		}
		if !parentFound {
			return nil, fmt.Errorf("parent task #%s not found in project '%s'", task.ParentID, projectSlug)
		}
	}

	now := time.Now()
	task.InsertedAt = now
	task.UpdatedAt = now

	// Auto-generate numeric ID if empty
	if task.ID == "" {
		maxID := 0
		for _, existing := range p.Tasks {
			if idNum, err := strconv.Atoi(existing.ID); err == nil && idNum > maxID {
				maxID = idNum
			}
		}
		task.ID = strconv.Itoa(maxID + 1)
	}

	p.Tasks = append(p.Tasks, task)
	p.UpdatedAt = now

	if err := s.saveProject(p); err != nil {
		return nil, err
	}

	go s.notify(Event{
		Type:        EventTaskCreated,
		ProjectSlug: projectSlug,
		TaskID:      task.ID,
	})
	return &task, nil
}

func (s *Store) CompleteTask(projectSlug string, taskID string, done bool, resolution ...string) (*model.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if projectSlug == "" {
		projectSlug = s.config.ActiveProject
	}
	projectSlug = strings.ToLower(projectSlug)

	p, ok := s.projects[projectSlug]
	if !ok {
		return nil, fmt.Errorf("project '%s' not found", projectSlug)
	}

	for i := range p.Tasks {
		if p.Tasks[i].ID == taskID {
			p.Tasks[i].Done = done
			now := time.Now()
			p.Tasks[i].UpdatedAt = now
			if done {
				p.Tasks[i].TerminatedAt = &now
				if len(resolution) > 0 && resolution[0] != "" {
					p.Tasks[i].Resolution = resolution[0]
				}
			} else {
				p.Tasks[i].TerminatedAt = nil
			}
			p.UpdatedAt = now

			if err := s.saveProject(p); err != nil {
				return nil, err
			}

			go s.notify(Event{
				Type:        EventTaskCompleted,
				ProjectSlug: projectSlug,
				TaskID:      taskID,
			})
			return &p.Tasks[i], nil
		}
	}

	return nil, fmt.Errorf("task ID '%s' not found in project '%s'", taskID, projectSlug)
}

func (s *Store) UpdateTask(projectSlug string, task model.Task) (*model.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if projectSlug == "" {
		projectSlug = s.config.ActiveProject
	}
	projectSlug = strings.ToLower(projectSlug)

	p, ok := s.projects[projectSlug]
	if !ok {
		return nil, fmt.Errorf("project '%s' not found", projectSlug)
	}

	for i := range p.Tasks {
		if p.Tasks[i].ID == task.ID {
			now := time.Now()
			if task.Title != "" {
				p.Tasks[i].Title = task.Title
			}
			p.Tasks[i].Description = task.Description
			if task.ParentID != "" {
				if task.ParentID == task.ID {
					return nil, errors.New("a task cannot be its own parent")
				}
				if task.ParentID == "none" || task.ParentID == "0" {
					p.Tasks[i].ParentID = ""
				} else {
					parentFound := false
					for _, existing := range p.Tasks {
						if existing.ID == task.ParentID {
							parentFound = true
							break
						}
					}
					if !parentFound {
						return nil, fmt.Errorf("parent task #%s not found in project '%s'", task.ParentID, projectSlug)
					}
					// Cycle check
					curr := task.ParentID
					for curr != "" {
						if curr == task.ID {
							return nil, errors.New("cannot set parent: circular dependency detected")
						}
						next := ""
						for _, existing := range p.Tasks {
							if existing.ID == curr {
								next = existing.ParentID
								break
							}
						}
						curr = next
					}
					p.Tasks[i].ParentID = task.ParentID
				}
			}
			if task.Size != "" {
				p.Tasks[i].Size = task.Size
			}
			if task.Tier > 0 {
				p.Tasks[i].Tier = task.Tier
			}
			if task.Resolution != "" {
				p.Tasks[i].Resolution = task.Resolution
			}
			if task.Assignee != "" {
				p.Tasks[i].Assignee = task.Assignee
			}
			p.Tasks[i].Tag = task.Tag
			p.Tasks[i].UpdatedAt = now
			p.UpdatedAt = now

			if err := s.saveProject(p); err != nil {
				return nil, err
			}

			go s.notify(Event{
				Type:        EventTaskUpdated,
				ProjectSlug: projectSlug,
				TaskID:      task.ID,
			})
			return &p.Tasks[i], nil
		}
	}

	return nil, fmt.Errorf("task ID '%s' not found in project '%s'", task.ID, projectSlug)
}

func (s *Store) AssignTask(projectSlug string, taskID string, assignee string) (*model.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if projectSlug == "" {
		projectSlug = s.config.ActiveProject
	}
	projectSlug = strings.ToLower(projectSlug)

	p, ok := s.projects[projectSlug]
	if !ok {
		return nil, fmt.Errorf("project '%s' not found", projectSlug)
	}

	for i := range p.Tasks {
		if p.Tasks[i].ID == taskID {
			p.Tasks[i].Assignee = strings.TrimSpace(assignee)
			now := time.Now()
			p.Tasks[i].UpdatedAt = now
			p.UpdatedAt = now

			if err := s.saveProject(p); err != nil {
				return nil, err
			}

			go s.notify(Event{
				Type:        EventTaskUpdated,
				ProjectSlug: projectSlug,
				TaskID:      taskID,
			})
			return &p.Tasks[i], nil
		}
	}

	return nil, fmt.Errorf("task ID '%s' not found in project '%s'", taskID, projectSlug)
}

func (s *Store) DeleteTask(projectSlug string, taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if projectSlug == "" {
		projectSlug = s.config.ActiveProject
	}
	projectSlug = strings.ToLower(projectSlug)

	p, ok := s.projects[projectSlug]
	if !ok {
		return fmt.Errorf("project '%s' not found", projectSlug)
	}

	// Verify taskID exists
	found := false
	for _, t := range p.Tasks {
		if t.ID == taskID {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("task ID '%s' not found in project '%s'", taskID, projectSlug)
	}

	// Find all descendant tasks to delete recursively
	toDelete := map[string]bool{taskID: true}
	added := true
	for added {
		added = false
		for _, t := range p.Tasks {
			if !toDelete[t.ID] && toDelete[t.ParentID] {
				toDelete[t.ID] = true
				added = true
			}
		}
	}

	var remaining []model.Task
	for _, t := range p.Tasks {
		if !toDelete[t.ID] {
			remaining = append(remaining, t)
		}
	}
	p.Tasks = remaining
	p.UpdatedAt = time.Now()

	if err := s.saveProject(p); err != nil {
		return err
	}

	go s.notify(Event{
		Type:        EventTaskDeleted,
		ProjectSlug: projectSlug,
		TaskID:      taskID,
	})
	return nil
}
