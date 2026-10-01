package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/altenwald/backlog/pkg/model"
)

type specSectionPayload struct {
	Title    *string `json:"title,omitempty"`
	Body     *string `json:"body,omitempty"`
	Position *int    `json:"position,omitempty"`
}

func (c *Client) sectionsURL(projectSlug string, parts ...string) string {
	u := fmt.Sprintf("%s/api/projects/%s/spec/sections", c.baseURL, url.PathEscape(projectSlug))
	for _, p := range parts {
		u += "/" + url.PathEscape(p)
	}
	return u
}

// doSpec sends a request with an optional JSON body and decodes the response
// into out when it is not nil. Server error messages are surfaced.
func (c *Client) doSpec(method, u string, in, out any) error {
	var body *bytes.Reader
	if in != nil {
		data, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	} else {
		body = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, u, body)
	if err != nil {
		return err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var e struct {
			Error string `json:"error"`
		}
		if json.NewDecoder(resp.Body).Decode(&e) == nil && e.Error != "" {
			return fmt.Errorf("%s", e.Error)
		}
		return fmt.Errorf("server error: %s", resp.Status)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// ListSpecSections returns the spec section index (no bodies).
func (c *Client) ListSpecSections(projectSlug string) ([]model.SpecSectionInfo, error) {
	var infos []model.SpecSectionInfo
	err := c.doSpec(http.MethodGet, c.sectionsURL(projectSlug), nil, &infos)
	return infos, err
}

// GetSpecSections returns the requested sections, or all when ids is empty.
func (c *Client) GetSpecSections(projectSlug string, ids []string) ([]model.SpecSection, error) {
	if len(ids) == 0 {
		infos, err := c.ListSpecSections(projectSlug)
		if err != nil {
			return nil, err
		}
		for _, info := range infos {
			ids = append(ids, info.ID)
		}
		if len(ids) == 0 {
			return nil, nil
		}
	}
	q := url.Values{"id": ids}
	var sections []model.SpecSection
	err := c.doSpec(http.MethodGet, c.sectionsURL(projectSlug)+"?"+q.Encode(), nil, &sections)
	return sections, err
}

// AddSpecSection creates a section at position (negative appends it).
func (c *Client) AddSpecSection(projectSlug, title, body string, position int) (*model.SpecSection, error) {
	var sec model.SpecSection
	err := c.doSpec(http.MethodPost, c.sectionsURL(projectSlug),
		specSectionPayload{Title: &title, Body: &body, Position: &position}, &sec)
	return &sec, err
}

// UpdateSpecSection changes the title and/or body of a section.
func (c *Client) UpdateSpecSection(projectSlug, id string, title, body *string) (*model.SpecSection, error) {
	var sec model.SpecSection
	err := c.doSpec(http.MethodPatch, c.sectionsURL(projectSlug, id),
		specSectionPayload{Title: title, Body: body}, &sec)
	return &sec, err
}

// DeleteSpecSection removes a section.
func (c *Client) DeleteSpecSection(projectSlug, id string) error {
	return c.doSpec(http.MethodDelete, c.sectionsURL(projectSlug, id), nil, nil)
}

// MoveSpecSection moves a section to position (0-based).
func (c *Client) MoveSpecSection(projectSlug, id string, position int) error {
	return c.doSpec(http.MethodPost, c.sectionsURL(projectSlug, id, "move"),
		specSectionPayload{Position: &position}, nil)
}
