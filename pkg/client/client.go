package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/altenwald/backlog/pkg/model"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8484"
	}
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (c *Client) IsServerRunning() bool {
	resp, err := c.httpClient.Get(c.baseURL + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

type ProjectInfo struct {
	Slug        string         `json:"slug"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Active      bool           `json:"active"`
	Summary     *model.Summary `json:"summary"`
}

func (c *Client) ListProjects() ([]ProjectInfo, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/api/projects")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server error: %s", resp.Status)
	}

	var projects []ProjectInfo
	err = json.NewDecoder(resp.Body).Decode(&projects)
	return projects, err
}

func (c *Client) GetActiveProject() (*ProjectInfo, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/api/projects/active")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server error: %s", resp.Status)
	}

	var p ProjectInfo
	err = json.NewDecoder(resp.Body).Decode(&p)
	return &p, err
}

func (c *Client) SetActiveProject(slug string) error {
	body, _ := json.Marshal(map[string]string{"slug": slug})
	resp, err := c.httpClient.Post(c.baseURL+"/api/projects/active", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server error: %s", resp.Status)
	}
	return nil
}

func (c *Client) CreateProject(slug, name, description string) (*model.Project, error) {
	body, _ := json.Marshal(map[string]string{
		"slug":        slug,
		"name":        name,
		"description": description,
	})
	resp, err := c.httpClient.Post(c.baseURL+"/api/projects", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("server error: %s", resp.Status)
	}

	var p model.Project
	err = json.NewDecoder(resp.Body).Decode(&p)
	return &p, err
}

func (c *Client) GetSummary(projectSlug string) (*model.Summary, error) {
	resp, err := c.httpClient.Get(fmt.Sprintf("%s/api/projects/%s/summary", c.baseURL, url.PathEscape(projectSlug)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server error: %s", resp.Status)
	}

	var sum model.Summary
	err = json.NewDecoder(resp.Body).Decode(&sum)
	return &sum, err
}

func (c *Client) GetTopPriorities(projectSlug string, limit int) ([]model.Task, error) {
	u := fmt.Sprintf("%s/api/projects/%s/top?limit=%d", c.baseURL, url.PathEscape(projectSlug), limit)
	resp, err := c.httpClient.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server error: %s", resp.Status)
	}

	var tasks []model.Task
	err = json.NewDecoder(resp.Body).Decode(&tasks)
	return tasks, err
}

func (c *Client) ListTasks(projectSlug string, filter model.TaskFilter) ([]model.Task, error) {
	params := url.Values{}
	if filter.Tier != nil {
		params.Set("tier", strconv.Itoa(int(*filter.Tier)))
	}
	if filter.Group != nil && *filter.Group != "" {
		params.Set("group", *filter.Group)
	}
	if filter.Done != nil {
		params.Set("done", strconv.FormatBool(*filter.Done))
	}
	if filter.Assignee != nil && *filter.Assignee != "" {
		params.Set("assignee", *filter.Assignee)
	}
	if filter.Search != "" {
		params.Set("search", filter.Search)
	}

	u := fmt.Sprintf("%s/api/projects/%s/tasks?%s", c.baseURL, url.PathEscape(projectSlug), params.Encode())
	resp, err := c.httpClient.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server error: %s", resp.Status)
	}

	var tasks []model.Task
	err = json.NewDecoder(resp.Body).Decode(&tasks)
	return tasks, err
}

func (c *Client) AddTask(projectSlug string, task model.Task) (*model.Task, error) {
	body, _ := json.Marshal(task)
	u := fmt.Sprintf("%s/api/projects/%s/tasks", c.baseURL, url.PathEscape(projectSlug))
	resp, err := c.httpClient.Post(u, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("server error: %s", resp.Status)
	}

	var created model.Task
	err = json.NewDecoder(resp.Body).Decode(&created)
	return &created, err
}

func (c *Client) CompleteTask(projectSlug, taskID string, done bool, resolution ...string) (*model.Task, error) {
	reqBody := map[string]any{"done": done}
	if len(resolution) > 0 && resolution[0] != "" {
		reqBody["resolution"] = resolution[0]
	}
	body, _ := json.Marshal(reqBody)
	u := fmt.Sprintf("%s/api/projects/%s/tasks/%s/done", c.baseURL, url.PathEscape(projectSlug), url.PathEscape(taskID))
	resp, err := c.httpClient.Post(u, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server error: %s", resp.Status)
	}

	var updated model.Task
	err = json.NewDecoder(resp.Body).Decode(&updated)
	return &updated, err
}

func (c *Client) AssignTask(projectSlug, taskID string, assignee string) (*model.Task, error) {
	body, _ := json.Marshal(map[string]string{"assignee": assignee})
	u := fmt.Sprintf("%s/api/projects/%s/tasks/%s/assign", c.baseURL, url.PathEscape(projectSlug), url.PathEscape(taskID))
	resp, err := c.httpClient.Post(u, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server error: %s", resp.Status)
	}

	var updated model.Task
	err = json.NewDecoder(resp.Body).Decode(&updated)
	return &updated, err
}

func (c *Client) DeleteTask(projectSlug, taskID string) error {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/projects/%s/tasks/%s", c.baseURL, url.PathEscape(projectSlug), url.PathEscape(taskID)), nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server error: %s", resp.Status)
	}
	return nil
}
