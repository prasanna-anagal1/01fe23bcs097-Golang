# AI Prompts Used

This document lists all prompts used when generating code for this project using AI assistance, as required by the project guidelines for transparency.

---

## Prompt 1 — Project Architecture Design

**Tool:** Antigravity (Google DeepMind AI)

**Prompt:**
> The user provided the capstone project specification (Capstone Project 5: Event Registration & Ticketing System) from an Instructions.txt file and asked for a complete Go REST API implementation to be generated directly into the project directory, ready for GitHub upload. The instruction asked for:
> - Complete REST API in Go
> - Database schema with proper constraints
> - Concurrent booking test (simulate multiple users booking last spot)
> - README with concurrency strategy explanation
> - Document explaining approach to preventing race conditions

**Files generated:**
- `main.go` — Server entry point and route registration
- `go.mod` — Module definition with SQLite and bcrypt dependencies
- `schema.sql` — Database schema reference
- `db/db.go` — Database initialisation with WAL mode and schema migration
- `models/user.go` — User struct and bcrypt password operations
- `models/event.go` — Event CRUD database operations
- `models/registration.go` — Concurrency-safe atomic booking logic
- `middleware/auth.go` — HMAC-SHA256 token generation and validation middleware
- `handlers/auth.go` — Register and Login HTTP handlers
- `handlers/events.go` — Event CRUD HTTP handlers
- `handlers/registrations.go` — Registration HTTP handlers
- `handlers/helpers.go` — Shared JSON response utilities
- `concurrent_test.go` — 50-goroutine concurrent booking simulation test
- `README.md` — Project documentation
- `DESIGN.md` — Race condition prevention design document
- `.gitignore` — Git ignore file

---

## Notes

All code was reviewed and understood before submission. The core concurrency algorithm (BEGIN IMMEDIATE transactions with single-connection constraint) was specifically chosen and understood for its correctness properties. The AI was used as a code generation tool; the design decisions were explained in DESIGN.md.
