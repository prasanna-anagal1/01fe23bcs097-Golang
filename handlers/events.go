package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"event-ticketing/middleware"
	"event-ticketing/models"
)

// eventRequest is the body for creating/updating an event.
type eventRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Location    string `json:"location"`
	Date        string `json:"date"`     // RFC3339 format
	Capacity    int    `json:"capacity"`
}

// Events routes GET/POST /api/events
func Events(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		listEvents(w, r)
	case http.MethodPost:
		middleware.Authenticate(createEvent)(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// EventByID routes GET/PUT/DELETE /api/events/:id
func EventByID(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getEvent(w, r)
	case http.MethodPut:
		middleware.Authenticate(updateEvent)(w, r)
	case http.MethodDelete:
		middleware.Authenticate(deleteEvent)(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// listEvents handles GET /api/events
func listEvents(w http.ResponseWriter, r *http.Request) {
	events, err := models.ListEvents()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch events")
		return
	}
	if events == nil {
		events = []models.Event{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"events": events,
		"count":  len(events),
	})
}

// getEvent handles GET /api/events/:id
func getEvent(w http.ResponseWriter, r *http.Request) {
	id, ok := middleware.GetPathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid event ID")
		return
	}
	event, err := models.GetEventByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, event)
}

// createEvent handles POST /api/events (organizer only)
func createEvent(w http.ResponseWriter, r *http.Request) {
	if middleware.GetUserRole(r) != "organizer" {
		writeError(w, http.StatusForbidden, "only organizers can create events")
		return
	}

	var req eventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := validateEventRequest(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	date, err := time.Parse(time.RFC3339, req.Date)
	if err != nil {
		writeError(w, http.StatusBadRequest, "date must be in RFC3339 format, e.g. 2026-03-01T10:00:00Z")
		return
	}

	event, err := models.CreateEvent(req.Title, req.Description, req.Location, date, req.Capacity, middleware.GetUserID(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create event: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, event)
}

// updateEvent handles PUT /api/events/:id (organizer who owns the event)
func updateEvent(w http.ResponseWriter, r *http.Request) {
	if middleware.GetUserRole(r) != "organizer" {
		writeError(w, http.StatusForbidden, "only organizers can update events")
		return
	}
	id, ok := middleware.GetPathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid event ID")
		return
	}
	event, err := models.GetEventByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "event not found")
		return
	}
	if event.OrganizerID != middleware.GetUserID(r) {
		writeError(w, http.StatusForbidden, "you are not the organizer of this event")
		return
	}

	var req eventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	date := event.Date
	if req.Date != "" {
		date, err = time.Parse(time.RFC3339, req.Date)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid date format")
			return
		}
	}

	// Merge — keep existing values if not provided
	if req.Title == "" { req.Title = event.Title }
	if req.Description == "" { req.Description = event.Description }
	if req.Location == "" { req.Location = event.Location }
	if req.Capacity == 0 { req.Capacity = event.Capacity }

	if err := models.UpdateEvent(id, req.Title, req.Description, req.Location, date, req.Capacity); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update event")
		return
	}
	updated, _ := models.GetEventByID(id)
	writeJSON(w, http.StatusOK, updated)
}

// deleteEvent handles DELETE /api/events/:id (organizer who owns the event)
func deleteEvent(w http.ResponseWriter, r *http.Request) {
	if middleware.GetUserRole(r) != "organizer" {
		writeError(w, http.StatusForbidden, "only organizers can delete events")
		return
	}
	id, ok := middleware.GetPathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid event ID")
		return
	}
	event, err := models.GetEventByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "event not found")
		return
	}
	if event.OrganizerID != middleware.GetUserID(r) {
		writeError(w, http.StatusForbidden, "you are not the organizer of this event")
		return
	}
	if err := models.DeleteEvent(id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete event")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "event deleted"})
}

func validateEventRequest(req eventRequest) error {
	if strings.TrimSpace(req.Title) == "" {
		return errorf("title is required")
	}
	if strings.TrimSpace(req.Description) == "" {
		return errorf("description is required")
	}
	if strings.TrimSpace(req.Location) == "" {
		return errorf("location is required")
	}
	if req.Capacity <= 0 {
		return errorf("capacity must be greater than 0")
	}
	if req.Date == "" {
		return errorf("date is required (RFC3339 format)")
	}
	return nil
}
