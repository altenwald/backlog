package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/altenwald/backlog/pkg/model"
	"github.com/altenwald/backlog/pkg/store"
	"github.com/go-chi/chi/v5"
)

type APIHandler struct {
	store *store.Store
}

func NewAPIHandler(st *store.Store) *APIHandler {
	return &APIHandler{store: st}
}

func (h *APIHandler) RegisterRoutes(r chi.Router) {
	r.Route("/api", func(r chi.Router) {
		r.Get("/projects", h.listProjects)
		r.Post("/projects", h.createProject)
		r.Get("/projects/active", h.getActiveProject)
		r.Post("/projects/active", h.setActiveProject)

		r.Route("/projects/{slug}", func(r chi.Router) {
			r.Get("/summary", h.getSummary)
			r.Get("/top", h.getTopPriorities)
			r.Get("/tasks", h.listTasks)
			r.Post("/tasks", h.createTask)
			r.Get("/tasks/{id}", h.getTask)
			r.Put("/tasks/{id}", h.updateTask)
			r.Post("/tasks/{id}/done", h.completeTask)
			r.Delete("/tasks/{id}", h.deleteTask)
		})
	})
}

func jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func errorResponse(w http.ResponseWriter, status int, message string) {
	jsonResponse(w, status, map[string]string{"error": message})
}

func (h *APIHandler) listProjects(w http.ResponseWriter, r *http.Request) {
	projects := h.store.ListProjects()
	active := h.store.GetActiveProjectSlug()

	type ProjectResp struct {
		Slug        string         `json:"slug"`
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Active      bool           `json:"active"`
		Summary     *model.Summary `json:"summary"`
	}

	var list []ProjectResp
	for _, p := range projects {
		sum, _ := h.store.GetSummary(p.Slug)
		list = append(list, ProjectResp{
			Slug:        p.Slug,
			Name:        p.Name,
			Description: p.Description,
			Active:      p.Slug == active,
			Summary:     sum,
		})
	}
	jsonResponse(w, http.StatusOK, list)
}

func (h *APIHandler) createProject(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Slug        string `json:"slug"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid json body")
		return
	}

	p, err := h.store.CreateProject(body.Slug, body.Name, body.Description)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	jsonResponse(w, http.StatusCreated, p)
}

func (h *APIHandler) getActiveProject(w http.ResponseWriter, r *http.Request) {
	active := h.store.GetActiveProjectSlug()
	p, err := h.store.GetProject(active)
	if err != nil {
		errorResponse(w, http.StatusNotFound, err.Error())
		return
	}
	sum, _ := h.store.GetSummary(active)
	jsonResponse(w, http.StatusOK, map[string]any{
		"slug":        p.Slug,
		"name":        p.Name,
		"description": p.Description,
		"summary":     sum,
	})
}

func (h *APIHandler) setActiveProject(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Slug string `json:"slug"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if err := h.store.SetActiveProject(body.Slug); err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"message": "active project updated", "active_project": body.Slug})
}

func (h *APIHandler) getSummary(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	sum, err := h.store.GetSummary(slug)
	if err != nil {
		errorResponse(w, http.StatusNotFound, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, sum)
}

func (h *APIHandler) getTopPriorities(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	limit := 5
	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			limit = val
		}
	}
	tasks, err := h.store.GetTopPriorities(slug, limit)
	if err != nil {
		errorResponse(w, http.StatusNotFound, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, tasks)
}

func (h *APIHandler) listTasks(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	q := r.URL.Query()

	filter := model.TaskFilter{
		Search: q.Get("search"),
	}

	if tStr := q.Get("tier"); tStr != "" {
		if val, err := strconv.Atoi(tStr); err == nil && val >= 1 && val <= 5 {
			tier := model.Tier(val)
			filter.Tier = &tier
		}
	}
	if gStr := q.Get("group"); gStr != "" {
		filter.Group = &gStr
	}
	if dStr := q.Get("done"); dStr != "" {
		done := strings.EqualFold(dStr, "true") || dStr == "1"
		filter.Done = &done
	}

	tasks, err := h.store.ListTasks(slug, filter)
	if err != nil {
		errorResponse(w, http.StatusNotFound, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, tasks)
}

func (h *APIHandler) createTask(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	var task model.Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid json body")
		return
	}

	created, err := h.store.AddTask(slug, task)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	jsonResponse(w, http.StatusCreated, created)
}

func (h *APIHandler) getTask(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	id := chi.URLParam(r, "id")

	tasks, err := h.store.ListTasks(slug, model.TaskFilter{})
	if err != nil {
		errorResponse(w, http.StatusNotFound, err.Error())
		return
	}

	for _, t := range tasks {
		if t.ID == id {
			jsonResponse(w, http.StatusOK, t)
			return
		}
	}
	errorResponse(w, http.StatusNotFound, "task not found")
}

func (h *APIHandler) updateTask(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	id := chi.URLParam(r, "id")

	var task model.Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid json body")
		return
	}
	task.ID = id

	updated, err := h.store.UpdateTask(slug, task)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, updated)
}

func (h *APIHandler) completeTask(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	id := chi.URLParam(r, "id")

	var body struct {
		Done *bool `json:"done"`
	}
	done := true
	if err := json.NewDecoder(r.Body).Decode(&body); err == nil && body.Done != nil {
		done = *body.Done
	}

	updated, err := h.store.CompleteTask(slug, id, done)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, updated)
}

func (h *APIHandler) deleteTask(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	id := chi.URLParam(r, "id")

	if err := h.store.DeleteTask(slug, id); err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"message": "task deleted"})
}
