package server

import (
	"github.com/altenwald/backlog/pkg/client"
	"github.com/altenwald/backlog/pkg/model"
	"github.com/altenwald/backlog/pkg/store"
)

type ProjectSummaryItem struct {
	Slug       string `json:"slug"`
	Name       string `json:"name"`
	Active     bool   `json:"active"`
	OpenTasks  int    `json:"open_tasks"`
	TotalTasks int    `json:"total_tasks"`
}

type Backend interface {
	ListProjects() ([]ProjectSummaryItem, error)
	GetActiveProject() (map[string]any, error)
	GetActiveProjectSlug() string
	SetActiveProject(slug string) error
	CreateProject(slug, name, desc string) (*model.Project, error)
	GetProject(slug string) (*model.Project, error)
	GetSummary(slug string) (*model.Summary, error)
	GetTopPriorities(slug string, limit int) ([]model.Task, error)
	ListTasks(slug string, filter model.TaskFilter) ([]model.Task, error)
	AddTask(slug string, task model.Task) (*model.Task, error)
	UpdateTask(slug string, task model.Task) (*model.Task, error)
	CompleteTask(slug string, taskID string, done bool, resolution string) (*model.Task, error)
	AssignTask(slug string, taskID string, assignee string) (*model.Task, error)
	DeleteTask(slug string, taskID string) error
	DeleteProject(slug string) error
}

// storeBackend adapts *store.Store to Backend
type storeBackend struct {
	st *store.Store
}

func NewStoreBackend(st *store.Store) Backend {
	return &storeBackend{st: st}
}

func (b *storeBackend) ListProjects() ([]ProjectSummaryItem, error) {
	projects := b.st.ListProjects()
	active := b.st.GetActiveProjectSlug()
	var items []ProjectSummaryItem
	for _, p := range projects {
		sum, _ := b.st.GetSummary(p.Slug)
		openTasks, totalTasks := 0, 0
		if sum != nil {
			openTasks = sum.OpenTasks
			totalTasks = sum.TotalTasks
		}
		items = append(items, ProjectSummaryItem{
			Slug:       p.Slug,
			Name:       p.Name,
			Active:     p.Slug == active,
			OpenTasks:  openTasks,
			TotalTasks: totalTasks,
		})
	}
	return items, nil
}

func (b *storeBackend) GetActiveProjectSlug() string {
	return b.st.GetActiveProjectSlug()
}

func (b *storeBackend) GetActiveProject() (map[string]any, error) {
	active := b.st.GetActiveProjectSlug()
	p, err := b.st.GetProject(active)
	if err != nil {
		return nil, err
	}
	sum, _ := b.st.GetSummary(active)
	return map[string]any{
		"active_project": active,
		"name":           p.Name,
		"description":    p.Description,
		"summary":        sum,
	}, nil
}

func (b *storeBackend) SetActiveProject(slug string) error {
	return b.st.SetActiveProject(slug)
}

func (b *storeBackend) CreateProject(slug, name, desc string) (*model.Project, error) {
	return b.st.CreateProject(slug, name, desc)
}

func (b *storeBackend) GetProject(slug string) (*model.Project, error) {
	return b.st.GetProject(slug)
}

func (b *storeBackend) GetSummary(slug string) (*model.Summary, error) {
	return b.st.GetSummary(slug)
}

func (b *storeBackend) GetTopPriorities(slug string, limit int) ([]model.Task, error) {
	return b.st.GetTopPriorities(slug, limit)
}

func (b *storeBackend) ListTasks(slug string, filter model.TaskFilter) ([]model.Task, error) {
	return b.st.ListTasks(slug, filter)
}

func (b *storeBackend) AddTask(slug string, task model.Task) (*model.Task, error) {
	return b.st.AddTask(slug, task)
}

func (b *storeBackend) UpdateTask(slug string, task model.Task) (*model.Task, error) {
	return b.st.UpdateTask(slug, task)
}

func (b *storeBackend) CompleteTask(slug string, taskID string, done bool, resolution string) (*model.Task, error) {
	return b.st.CompleteTask(slug, taskID, done, resolution)
}

func (b *storeBackend) AssignTask(slug string, taskID string, assignee string) (*model.Task, error) {
	return b.st.AssignTask(slug, taskID, assignee)
}

func (b *storeBackend) DeleteTask(slug string, taskID string) error {
	return b.st.DeleteTask(slug, taskID)
}

func (b *storeBackend) DeleteProject(slug string) error {
	return b.st.DeleteProject(slug)
}

// clientBackend adapts *client.Client to Backend
type clientBackend struct {
	c *client.Client
}

func NewClientBackend(c *client.Client) Backend {
	return &clientBackend{c: c}
}

func (b *clientBackend) ListProjects() ([]ProjectSummaryItem, error) {
	projects, err := b.c.ListProjects()
	if err != nil {
		return nil, err
	}
	var items []ProjectSummaryItem
	for _, p := range projects {
		openTasks, totalTasks := 0, 0
		if p.Summary != nil {
			openTasks = p.Summary.OpenTasks
			totalTasks = p.Summary.TotalTasks
		}
		items = append(items, ProjectSummaryItem{
			Slug:       p.Slug,
			Name:       p.Name,
			Active:     p.Active,
			OpenTasks:  openTasks,
			TotalTasks: totalTasks,
		})
	}
	return items, nil
}

func (b *clientBackend) GetActiveProjectSlug() string {
	act, err := b.c.GetActiveProject()
	if err != nil || act == nil {
		return ""
	}
	return act.Slug
}

func (b *clientBackend) GetActiveProject() (map[string]any, error) {
	act, err := b.c.GetActiveProject()
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"active_project": act.Slug,
		"name":           act.Name,
		"description":    act.Description,
		"summary":        act.Summary,
	}, nil
}

func (b *clientBackend) SetActiveProject(slug string) error {
	return b.c.SetActiveProject(slug)
}

func (b *clientBackend) CreateProject(slug, name, desc string) (*model.Project, error) {
	return b.c.CreateProject(slug, name, desc)
}

func (b *clientBackend) GetProject(slug string) (*model.Project, error) {
	return b.c.GetProject(slug)
}

func (b *clientBackend) GetSummary(slug string) (*model.Summary, error) {
	return b.c.GetSummary(slug)
}

func (b *clientBackend) GetTopPriorities(slug string, limit int) ([]model.Task, error) {
	return b.c.GetTopPriorities(slug, limit)
}

func (b *clientBackend) ListTasks(slug string, filter model.TaskFilter) ([]model.Task, error) {
	return b.c.ListTasks(slug, filter)
}

func (b *clientBackend) AddTask(slug string, task model.Task) (*model.Task, error) {
	return b.c.AddTask(slug, task)
}

func (b *clientBackend) UpdateTask(slug string, task model.Task) (*model.Task, error) {
	return b.c.UpdateTask(slug, task)
}

func (b *clientBackend) CompleteTask(slug string, taskID string, done bool, resolution string) (*model.Task, error) {
	return b.c.CompleteTask(slug, taskID, done, resolution)
}

func (b *clientBackend) AssignTask(slug string, taskID string, assignee string) (*model.Task, error) {
	return b.c.AssignTask(slug, taskID, assignee)
}

func (b *clientBackend) DeleteTask(slug string, taskID string) error {
	return b.c.DeleteTask(slug, taskID)
}

func (b *clientBackend) DeleteProject(slug string) error {
	return b.c.DeleteProject(slug)
}
