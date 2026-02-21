package models

import (
	"database/sql"
	"errors"
	"time"

	"event-ticketing/db"
)

// Event represents a ticketed event created by an organizer.
type Event struct {
	ID             int64     `json:"id"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	Location       string    `json:"location"`
	Date           time.Time `json:"date"`
	Capacity       int       `json:"capacity"`
	AvailableSpots int       `json:"available_spots"`
	OrganizerID    int64     `json:"organizer_id"`
	CreatedAt      time.Time `json:"created_at"`
}

// CreateEvent inserts a new event. AvailableSpots is initialised to Capacity.
func CreateEvent(title, description, location string, date time.Time, capacity int, organizerID int64) (*Event, error) {
	res, err := db.DB.Exec(
		`INSERT INTO events (title, description, location, date, capacity, available_spots, organizer_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		title, description, location, date, capacity, capacity, organizerID,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &Event{
		ID:             id,
		Title:          title,
		Description:    description,
		Location:       location,
		Date:           date,
		Capacity:       capacity,
		AvailableSpots: capacity,
		OrganizerID:    organizerID,
		CreatedAt:      time.Now(),
	}, nil
}

// ListEvents returns all events ordered by date ascending.
func ListEvents() ([]Event, error) {
	rows, err := db.DB.Query(
		`SELECT id, title, description, location, date, capacity, available_spots, organizer_id, created_at
		 FROM events ORDER BY date ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.Title, &e.Description, &e.Location,
			&e.Date, &e.Capacity, &e.AvailableSpots, &e.OrganizerID, &e.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// GetEventByID fetches a single event.
func GetEventByID(id int64) (*Event, error) {
	row := db.DB.QueryRow(
		`SELECT id, title, description, location, date, capacity, available_spots, organizer_id, created_at
		 FROM events WHERE id = ?`, id,
	)
	e := &Event{}
	err := row.Scan(&e.ID, &e.Title, &e.Description, &e.Location,
		&e.Date, &e.Capacity, &e.AvailableSpots, &e.OrganizerID, &e.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("event not found")
	}
	return e, err
}

// UpdateEvent modifies event details. Only the organizer should call this.
func UpdateEvent(id int64, title, description, location string, date time.Time, capacity int) error {
	_, err := db.DB.Exec(
		`UPDATE events SET title=?, description=?, location=?, date=?, capacity=?
		 WHERE id=?`,
		title, description, location, date, capacity, id,
	)
	return err
}

// DeleteEvent removes an event and cascades to its registrations.
func DeleteEvent(id int64) error {
	_, err := db.DB.Exec(`DELETE FROM events WHERE id = ?`, id)
	return err
}
