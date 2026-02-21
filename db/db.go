package db

import (
	"database/sql"
	"log"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

// Init opens the SQLite database, enables WAL mode + foreign keys,
// and creates all tables from the embedded schema.
func Init(path string) {
	var err error
	DB, err = sql.Open("sqlite", path)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}

	// Enable WAL for better concurrent read performance
	if _, err = DB.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		log.Fatalf("WAL pragma failed: %v", err)
	}
	// Enforce foreign key constraints
	if _, err = DB.Exec(`PRAGMA foreign_keys=ON;`); err != nil {
		log.Fatalf("foreign_keys pragma failed: %v", err)
	}

	// Limit writer concurrency — SQLite supports only one writer at a time
	// Setting max open conns to 1 prevents "database is locked" errors
	DB.SetMaxOpenConns(1)

	migrate()
	log.Println("Database initialised at", path)
}

// migrate creates all tables and indexes if they don't exist yet.
func migrate() {
	schema := `
CREATE TABLE IF NOT EXISTS users (
    id         INTEGER  PRIMARY KEY AUTOINCREMENT,
    name       TEXT     NOT NULL,
    email      TEXT     NOT NULL UNIQUE,
    password   TEXT     NOT NULL,
    role       TEXT     NOT NULL CHECK(role IN ('organizer','attendee')),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS events (
    id              INTEGER  PRIMARY KEY AUTOINCREMENT,
    title           TEXT     NOT NULL,
    description     TEXT     NOT NULL,
    location        TEXT     NOT NULL,
    date            DATETIME NOT NULL,
    capacity        INTEGER  NOT NULL CHECK(capacity > 0),
    available_spots INTEGER  NOT NULL CHECK(available_spots >= 0),
    organizer_id    INTEGER  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS registrations (
    id            INTEGER  PRIMARY KEY AUTOINCREMENT,
    event_id      INTEGER  NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    user_id       INTEGER  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    ticket_no     TEXT     NOT NULL UNIQUE,
    registered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(event_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_events_organizer      ON events(organizer_id);
CREATE INDEX IF NOT EXISTS idx_registrations_event   ON registrations(event_id);
CREATE INDEX IF NOT EXISTS idx_registrations_user    ON registrations(user_id);
`
	if _, err := DB.Exec(schema); err != nil {
		log.Fatalf("migration failed: %v", err)
	}
}
