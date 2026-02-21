package handlers

import (
	"errors"
	"net/http"

	"event-ticketing/middleware"
	"event-ticketing/models"
)

// RegisterForEvent handles POST /api/events/:id/register
func RegisterForEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	eventID, ok := middleware.GetEventIDFromPath(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid event ID in path")
		return
	}

	userID := middleware.GetUserID(r)
	if userID == 0 {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Verify event exists
	event, err := models.GetEventByID(eventID)
	if err != nil {
		writeError(w, http.StatusNotFound, "event not found")
		return
	}

	// Perform atomic registration
	reg, err := models.RegisterForEvent(eventID, userID)
	if err != nil {
		switch {
		case errors.Is(err, models.ErrNoSpots):
			writeError(w, http.StatusConflict, "event is fully booked — no available spots")
		case errors.Is(err, models.ErrAlreadyRegistered):
			writeError(w, http.StatusConflict, "you are already registered for this event")
		default:
			writeError(w, http.StatusInternalServerError, "registration failed: "+err.Error())
		}
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message":      "registration successful",
		"ticket_no":    reg.TicketNo,
		"event_id":     event.ID,
		"event_title":  event.Title,
		"registered_at": reg.RegisteredAt,
	})
}

// ListRegistrations handles GET /api/events/:id/registrations (organizer only)
func ListRegistrations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if middleware.GetUserRole(r) != "organizer" {
		writeError(w, http.StatusForbidden, "only organizers can view registrations")
		return
	}

	eventID, ok := middleware.GetEventIDFromPath(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid event ID in path")
		return
	}

	// Verify the event belongs to this organizer
	event, err := models.GetEventByID(eventID)
	if err != nil {
		writeError(w, http.StatusNotFound, "event not found")
		return
	}
	if event.OrganizerID != middleware.GetUserID(r) {
		writeError(w, http.StatusForbidden, "you are not the organizer of this event")
		return
	}

	regs, err := models.ListRegistrationsByEvent(eventID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list registrations")
		return
	}
	if regs == nil {
		regs = []models.Registration{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"event_id":      event.ID,
		"event_title":   event.Title,
		"capacity":      event.Capacity,
		"available_spots": event.AvailableSpots,
		"registrations": regs,
		"total":         len(regs),
	})
}
