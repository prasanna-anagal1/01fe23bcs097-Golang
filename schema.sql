-- Event Registration & Ticketing System -- Database Schema

CREATE TABLE IF NOT EXISTS users (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT    NOT NULL,
    email      TEXT    NOT NULL UNIQUE,
    password   TEXT    NOT NULL,
    role       TEXT    NOT NULL CHECK(role IN ('organizer', 'attendee')),
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
    id         INTEGER  PRIMARY KEY AUTOINCREMENT,
    event_id   INTEGER  NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    user_id    INTEGER  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    ticket_no  TEXT     NOT NULL UNIQUE,
    registered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(event_id, user_id)  -- prevent double-booking same user for same event
);

-- Index for fast lookups
CREATE INDEX IF NOT EXISTS idx_events_organizer ON events(organizer_id);
CREATE INDEX IF NOT EXISTS idx_registrations_event ON registrations(event_id);
CREATE INDEX IF NOT EXISTS idx_registrations_user ON registrations(user_id);
