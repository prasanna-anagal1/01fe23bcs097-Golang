package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"event-ticketing/db"
	"event-ticketing/handlers"
	"event-ticketing/middleware"
)

func main() {
	// Initialise the database (creates events.db if it doesn't exist)
	db.Init("events.db")

	mux := http.NewServeMux()

	// ── Auth (public) ────────────────────────────────────────────────
	mux.HandleFunc("/api/register", handlers.Register)
	mux.HandleFunc("/api/login", handlers.Login)

	// ── Events ───────────────────────────────────────────────────────
	// GET  /api/events          → list all events (public)
	// POST /api/events          → create event   (organizer, auth required)
	mux.HandleFunc("/api/events", handlers.Events)

	// GET    /api/events/:id    → get single event (public)
	// PUT    /api/events/:id    → update event     (organizer)
	// DELETE /api/events/:id    → delete event     (organizer)
	mux.HandleFunc("/api/events/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// POST /api/events/:id/register → register for event (attendee)
		if strings.HasSuffix(path, "/register") {
			middleware.Authenticate(handlers.RegisterForEvent)(w, r)
			return
		}

		// GET /api/events/:id/registrations → list registrations (organizer)
		if strings.HasSuffix(path, "/registrations") {
			middleware.Authenticate(handlers.ListRegistrations)(w, r)
			return
		}

		// Everything else → single event CRUD
		handlers.EventByID(w, r)
	})

	// ── Health check ─────────────────────────────────────────────────
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"status":"ok","service":"event-ticketing"}`)
	})

	// ── Root info ────────────────────────────────────────────────────
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{
  "service": "Event Registration & Ticketing System",
  "version": "1.0.0",
  "endpoints": {
    "POST /api/register":                    "Register a new user",
    "POST /api/login":                       "Login and get token",
    "GET  /api/events":                      "List all events",
    "POST /api/events":                      "Create event (organizer)",
    "GET  /api/events/:id":                  "Get event details",
    "PUT  /api/events/:id":                  "Update event (organizer)",
    "DELETE /api/events/:id":               "Delete event (organizer)",
    "POST /api/events/:id/register":         "Register for event (attendee)",
    "GET  /api/events/:id/registrations":    "List registrations (organizer)"
  }
}`)
	})

	port := "8080"
	log.Printf("🎟  Event Ticketing API running on http://localhost:%s", port)
	log.Printf("📄  API docs: http://localhost:%s/", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
