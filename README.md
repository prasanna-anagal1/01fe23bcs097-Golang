# 🎟 Event Registration & Ticketing System

A production-quality REST API built in Go for event registration and ticketing — similar to Eventbrite. Users can browse events, register for events with limited capacity, and organizers can create events and manage registrations.

The **critical engineering challenge** — handling concurrent registrations to prevent overbooking when multiple users race to book the last few spots — is solved using atomic database transactions.

---

## 🚀 Quick Start

### Prerequisites
- Go 1.21+

### Run the Server
```bash
go mod tidy
go run ./...
```

The server starts at **http://localhost:8080**

### Run Tests (including concurrent booking test)
```bash
go test -v -race ./...
```

---

## 📁 Project Structure

```
event-ticketing/
├── main.go                   # Server entry point & route registration
├── go.mod / go.sum           # Module dependencies
├── schema.sql                # Database schema (reference)
├── db/
│   └── db.go                 # SQLite connection, WAL mode, schema migration
├── models/
│   ├── user.go               # User struct + bcrypt password hashing
│   ├── event.go              # Event CRUD operations
│   └── registration.go       # Concurrent-safe booking logic (key file!)
├── handlers/
│   ├── auth.go               # Register & Login endpoints
│   ├── events.go             # Event CRUD endpoints
│   ├── registrations.go      # Register for event & list registrations
│   └── helpers.go            # Shared JSON response utilities
├── middleware/
│   └── auth.go               # HMAC-SHA256 token generation & validation
├── concurrent_test.go        # Concurrent booking simulation (50 goroutines)
├── DESIGN.md                 # Race condition prevention deep-dive
└── prompts.md                # AI prompts used (transparency)
```

---

## 🔌 API Endpoints

### Authentication

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| `POST` | `/api/register` | None | Register a new user |
| `POST` | `/api/login` | None | Login and receive a token |

### Events

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| `GET` | `/api/events` | None | List all events |
| `POST` | `/api/events` | Organizer | Create a new event |
| `GET` | `/api/events/:id` | None | Get event details |
| `PUT` | `/api/events/:id` | Organizer (owner) | Update event |
| `DELETE` | `/api/events/:id` | Organizer (owner) | Delete event |

### Registrations

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| `POST` | `/api/events/:id/register` | Any user | Register for an event |
| `GET` | `/api/events/:id/registrations` | Organizer (owner) | List all registrations |

---

## 🔐 Authentication

All protected endpoints require an `Authorization` header:
```
Authorization: Bearer <token>
```

The token is returned in the response body after `POST /api/register` or `POST /api/login`.

---

## 📋 Example curl Commands

### 1. Register an Organizer
```bash
curl -X POST http://localhost:8080/api/register \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Alice",
    "email": "alice@example.com",
    "password": "mypassword",
    "role": "organizer"
  }'
```

**Response:**
```json
{
  "token": "eyJhbGci...",
  "user": { "id": 1, "name": "Alice", "email": "alice@example.com", "role": "organizer" }
}
```

### 2. Create an Event
```bash
curl -X POST http://localhost:8080/api/events \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{
    "title": "Go Summit Bangalore 2026",
    "description": "Annual Go conference in Bangalore",
    "location": "Nimhans Convention Centre, Bangalore",
    "date": "2026-03-15T09:00:00Z",
    "capacity": 100
  }'
```

### 3. Browse Events
```bash
curl http://localhost:8080/api/events
```

### 4. Register for an Event
```bash
curl -X POST http://localhost:8080/api/events/1/register \
  -H "Authorization: Bearer <token>"
```

**Response (success):**
```json
{
  "message": "registration successful",
  "ticket_no": "TKT-1-2-3847",
  "event_id": 1,
  "event_title": "Go Summit Bangalore 2026"
}
```

**Response (fully booked — HTTP 409):**
```json
{ "error": "event is fully booked — no available spots" }
```

### 5. Login
```bash
curl -X POST http://localhost:8080/api/login \
  -H "Content-Type: application/json" \
  -d '{"email": "alice@example.com", "password": "mypassword"}'
```

---

## ⚡ Concurrency Strategy — Preventing Overbooking

See [DESIGN.md](./DESIGN.md) for the full deep-dive.

**Summary:** Every registration request runs inside a `BEGIN IMMEDIATE` SQLite transaction. This means:
1. SQLite acquires a write lock **before** we read `available_spots`
2. No other goroutine can write between our read and our decrement
3. If `available_spots == 0` → rollback → HTTP 409 (no overbooking)
4. If the user already registered → UNIQUE constraint fires → HTTP 409

Additionally:
- `DB.SetMaxOpenConns(1)` ensures SQLite's single-writer model is respected, preventing "database is locked" panics
- WAL mode allows concurrent **reads** while writing, improving throughput

### Test Results
```
=== RUN   TestConcurrentBooking
    concurrent_test.go: Results → success=5  conflict=45  error=0
    ✅ available_spots in DB = 0 (correct)
--- PASS: TestConcurrentBooking
=== RUN   TestNoOverbooking
    ✅ Exactly one registration succeeded — no overbooking
--- PASS: TestNoOverbooking
=== RUN   TestDoubleRegistrationPrevented
    ✅ Double registration correctly rejected
--- PASS: TestDoubleRegistrationPrevented
PASS
```

---

## 🗄 Database Schema

**SQLite** is used for zero-configuration simplicity. The database file (`events.db`) is created automatically on first run.

```sql
-- Users table
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    password TEXT NOT NULL,           -- bcrypt hashed
    role TEXT NOT NULL CHECK(role IN ('organizer','attendee')),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Events table
CREATE TABLE events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    location TEXT NOT NULL,
    date DATETIME NOT NULL,
    capacity INTEGER NOT NULL CHECK(capacity > 0),
    available_spots INTEGER NOT NULL CHECK(available_spots >= 0),
    organizer_id INTEGER NOT NULL REFERENCES users(id),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Registrations table
CREATE TABLE registrations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id INTEGER NOT NULL REFERENCES events(id),
    user_id INTEGER NOT NULL REFERENCES users(id),
    ticket_no TEXT NOT NULL UNIQUE,
    registered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(event_id, user_id)  -- prevents double-booking
);
```

---

## 🛡 Roles

| Role | Permissions |
|------|-------------|
| `organizer` | Create / update / delete own events; view their event's registrations |
| `attendee` | Browse events; register for events |

---

## 📦 Dependencies

| Package | Purpose |
|---------|---------|
| `modernc.org/sqlite` | Pure-Go SQLite driver (no CGO — works on all platforms) |
| `golang.org/x/crypto` | bcrypt password hashing |

All other code uses the Go standard library.

---

## 🧑‍💻 Design Decisions

1. **SQLite** over PostgreSQL/MySQL — perfect for single-binary deployment and evaluation without needing a running DB server.
2. **No external frameworks** — uses only `net/http` from the standard library.
3. **HMAC-SHA256 tokens** instead of full JWT — same security, no external libraries.
4. **`BEGIN IMMEDIATE` transactions** — the gold standard for preventing race conditions in SQLite.

---

*Built for Infosys Capstone Project 5 — Go (Golang)*
