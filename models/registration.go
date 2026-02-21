package models

import (
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"event-ticketing/db"
)

// Registration links a User to an Event and holds the unique ticket number.
type Registration struct {
	ID           int64     `json:"id"`
	EventID      int64     `json:"event_id"`
	UserID       int64     `json:"user_id"`
	TicketNo     string    `json:"ticket_no"`
	RegisteredAt time.Time `json:"registered_at"`
}

// ErrNoSpots is returned when an event is fully booked.
var ErrNoSpots = errors.New("no available spots")

// ErrAlreadyRegistered is returned when a user tries to book the same event twice.
var ErrAlreadyRegistered = errors.New("already registered for this event")

// RegisterForEvent atomically checks capacity and creates a registration.
//
// Concurrency strategy:
//   - Uses "BEGIN IMMEDIATE" so SQLite acquires a write-lock before any reads.
//   - Reads available_spots inside the transaction — no other writer can change
//     it between the read and the update (serialized access).
//   - If available_spots == 0, rolls back and returns ErrNoSpots (HTTP 409).
//   - If the UNIQUE(event_id, user_id) constraint fires, returns ErrAlreadyRegistered.
//   - On success, decrements available_spots and inserts the registration atomically.
//
// This prevents overbooking even when 50+ goroutines race simultaneously.
func RegisterForEvent(eventID, userID int64) (*Registration, error) {
	tx, err := db.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck

	// BEGIN IMMEDIATE is set at pragma level; within this transaction,
	// we lock the rows we need.
	var spots int
	err = tx.QueryRow(`SELECT available_spots FROM events WHERE id = ?`, eventID).Scan(&spots)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("event not found")
	}
	if err != nil {
		return nil, err
	}

	if spots <= 0 {
		return nil, ErrNoSpots
	}

	ticketNo := generateTicketNo(eventID, userID)

	_, err = tx.Exec(
		`INSERT INTO registrations (event_id, user_id, ticket_no) VALUES (?, ?, ?)`,
		eventID, userID, ticketNo,
	)
	if err != nil {
		// SQLite UNIQUE constraint violation message contains "UNIQUE constraint failed"
		if isUniqueConstraintErr(err) {
			return nil, ErrAlreadyRegistered
		}
		return nil, err
	}

	_, err = tx.Exec(
		`UPDATE events SET available_spots = available_spots - 1 WHERE id = ?`, eventID,
	)
	if err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return &Registration{
		EventID:      eventID,
		UserID:       userID,
		TicketNo:     ticketNo,
		RegisteredAt: time.Now(),
	}, nil
}

// ListRegistrationsByEvent returns all registrations for a given event.
func ListRegistrationsByEvent(eventID int64) ([]Registration, error) {
	rows, err := db.DB.Query(
		`SELECT id, event_id, user_id, ticket_no, registered_at
		 FROM registrations WHERE event_id = ? ORDER BY registered_at ASC`, eventID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var regs []Registration
	for rows.Next() {
		var r Registration
		if err := rows.Scan(&r.ID, &r.EventID, &r.UserID, &r.TicketNo, &r.RegisteredAt); err != nil {
			return nil, err
		}
		regs = append(regs, r)
	}
	return regs, rows.Err()
}

// generateTicketNo creates a human-readable unique ticket identifier.
func generateTicketNo(eventID, userID int64) string {
	return fmt.Sprintf("TKT-%d-%d-%04d", eventID, userID, rand.Intn(10000))
}

// isUniqueConstraintErr checks for SQLite's unique constraint error message.
func isUniqueConstraintErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return len(msg) > 0 && (contains(msg, "UNIQUE constraint failed") || contains(msg, "unique constraint"))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
