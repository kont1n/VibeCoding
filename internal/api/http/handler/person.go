package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/kont1n/face-grouper/internal/database"
	"github.com/kont1n/face-grouper/internal/report"
)

// PersonHandler handles person-related API endpoints.
type PersonHandler struct {
	outputDir string
	db        *database.DB
}

// NewPersonHandler creates a new PersonHandler.
func NewPersonHandler(outputDir string, db *database.DB) *PersonHandler {
	return &PersonHandler{
		outputDir: outputDir,
		db:        db,
	}
}

// List handles GET /api/v1/persons.
func (h *PersonHandler) List(w http.ResponseWriter, r *http.Request) {
	// Try database first.
	if h.db != nil && h.db.Persons != nil {
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit <= 0 || limit > 100 {
			limit = 50
		}

		persons, err := h.db.Persons.List(r.Context(), offset, limit)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		count, _ := h.db.Persons.Count(r.Context())

		writeJSON(w, http.StatusOK, map[string]any{
			"persons": persons,
			"total":   count,
			"offset":  offset,
			"limit":   limit,
		})
		return
	}

	// Fallback to report.json.
	rpt, err := report.Load(h.outputDir)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no data available"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"persons": rpt.Persons,
		"total":   len(rpt.Persons),
	})
}

// Get handles GET /api/v1/persons/{id}.
func (h *PersonHandler) Get(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")

	// Try database.
	if h.db != nil && h.db.Persons != nil {
		id, err := uuid.Parse(idStr)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid person ID"})
			return
		}

		person, err := h.db.Persons.GetByID(r.Context(), id)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if person == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "person not found"})
			return
		}

		writeJSON(w, http.StatusOK, person)
		return
	}

	// Fallback to report.json.
	rpt, err := report.Load(h.outputDir)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no data available"})
		return
	}

	personID, err := strconv.Atoi(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid person ID"})
		return
	}

	for _, p := range rpt.Persons {
		if p.ID == personID {
			writeJSON(w, http.StatusOK, p)
			return
		}
	}

	writeJSON(w, http.StatusNotFound, map[string]string{"error": "person not found"})
}

// Rename handles PUT /api/v1/persons/{id}.
func (h *PersonHandler) Rename(w http.ResponseWriter, r *http.Request) {
	if h.db == nil || h.db.Persons == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "database required for rename operation",
		})
		return
	}

	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid person ID"})
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if decodeErr := json.NewDecoder(r.Body).Decode(&req); decodeErr != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	// Sanitize name (prevent XSS).
	if len(req.Name) > 200 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name too long (max 200 chars)"})
		return
	}

	person, err := h.db.Persons.GetByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if person == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "person not found"})
		return
	}

	person.CustomName = req.Name
	if err := h.db.Persons.Update(r.Context(), person); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, person)
}

// Photos handles GET /api/v1/persons/{id}/photos.
func (h *PersonHandler) Photos(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")

	// Try database.
	if h.db != nil && h.db.Photos != nil {
		id, err := uuid.Parse(idStr)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid person ID"})
			return
		}

		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit <= 0 || limit > 100 {
			limit = 50
		}

		photos, err := h.db.Photos.List(r.Context(), offset, limit)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		count, _ := h.db.Photos.CountByPerson(r.Context(), id)

		writeJSON(w, http.StatusOK, map[string]any{
			"photos": photos,
			"total":  count,
			"offset": offset,
			"limit":  limit,
		})
		return
	}

	// Fallback to report.json.
	rpt, err := report.Load(h.outputDir)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no data available"})
		return
	}

	personID, err := strconv.Atoi(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid person ID"})
		return
	}

	for _, p := range rpt.Persons {
		if p.ID == personID {
			writeJSON(w, http.StatusOK, map[string]any{
				"photos": p.Photos,
				"total":  len(p.Photos),
			})
			return
		}
	}

	writeJSON(w, http.StatusNotFound, map[string]string{"error": "person not found"})
}

// Relations handles GET /api/v1/persons/{id}/relations.
func (h *PersonHandler) Relations(w http.ResponseWriter, r *http.Request) {
	if h.db == nil || h.db.Relations == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "database required for relations",
		})
		return
	}

	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid person ID"})
		return
	}

	minSimilarity := float32(0.0)
	if s := r.URL.Query().Get("min_similarity"); s != "" {
		if v, parseErr := strconv.ParseFloat(s, 32); parseErr == nil {
			minSimilarity = float32(v)
		}
	}

	relations, err := h.db.Relations.GetByPersonID(r.Context(), id, minSimilarity)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Build graph data: get person info for related persons.
	relatedIDs := make([]uuid.UUID, 0, len(relations))
	relatedIDs = append(relatedIDs, id) // Include the person itself.
	for _, rel := range relations {
		if rel.Person1ID == id {
			relatedIDs = append(relatedIDs, rel.Person2ID)
		} else {
			relatedIDs = append(relatedIDs, rel.Person1ID)
		}
	}

	nodes, err := h.db.Relations.GetGraph(r.Context(), relatedIDs, minSimilarity)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"person_id": id,
		"relations": relations,
		"nodes":     nodes,
	})
}
