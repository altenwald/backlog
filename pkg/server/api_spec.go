package server

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// SpecSectionRequest is the body accepted when creating, updating or moving a
// spec section. Nil fields are left unchanged on update.
type SpecSectionRequest struct {
	Title    *string `json:"title,omitempty"`
	Body     *string `json:"body,omitempty"`
	Position *int    `json:"position,omitempty"`
}

// listSpecSections returns the section index, or the full sections named by
// repeated ?id= query parameters.
func (h *APIHandler) listSpecSections(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if ids := r.URL.Query()["id"]; len(ids) > 0 {
		sections, err := h.store.GetSpecSections(slug, ids)
		if err != nil {
			errorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, sections)
		return
	}
	infos, err := h.store.ListSpecSections(slug)
	if err != nil {
		errorResponse(w, http.StatusNotFound, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, infos)
}

func (h *APIHandler) getSpecSection(w http.ResponseWriter, r *http.Request) {
	sections, err := h.store.GetSpecSections(chi.URLParam(r, "slug"), []string{chi.URLParam(r, "id")})
	if err != nil {
		errorResponse(w, http.StatusNotFound, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, sections[0])
}

func decodeSpecSectionRequest(w http.ResponseWriter, r *http.Request) (SpecSectionRequest, bool) {
	var body SpecSectionRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return body, false
	}
	return body, true
}

func (h *APIHandler) addSpecSection(w http.ResponseWriter, r *http.Request) {
	body, ok := decodeSpecSectionRequest(w, r)
	if !ok {
		return
	}
	var title, text string
	if body.Title != nil {
		title = *body.Title
	}
	if body.Body != nil {
		text = *body.Body
	}
	position := -1
	if body.Position != nil {
		position = *body.Position
	}
	sec, err := h.store.AddSpecSection(chi.URLParam(r, "slug"), title, text, position)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	jsonResponse(w, http.StatusCreated, sec)
}

func (h *APIHandler) updateSpecSection(w http.ResponseWriter, r *http.Request) {
	body, ok := decodeSpecSectionRequest(w, r)
	if !ok {
		return
	}
	sec, err := h.store.UpdateSpecSection(chi.URLParam(r, "slug"), chi.URLParam(r, "id"), body.Title, body.Body)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, sec)
}

func (h *APIHandler) deleteSpecSection(w http.ResponseWriter, r *http.Request) {
	if err := h.store.DeleteSpecSection(chi.URLParam(r, "slug"), chi.URLParam(r, "id")); err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *APIHandler) moveSpecSection(w http.ResponseWriter, r *http.Request) {
	body, ok := decodeSpecSectionRequest(w, r)
	if !ok {
		return
	}
	if body.Position == nil {
		errorResponse(w, http.StatusBadRequest, "position is required")
		return
	}
	if err := h.store.MoveSpecSection(chi.URLParam(r, "slug"), chi.URLParam(r, "id"), *body.Position); err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
