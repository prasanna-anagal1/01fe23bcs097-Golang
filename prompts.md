# AI Prompts Used

The following prompts were used to get assistance for specific parts of this project.

---

**Prompt 1 — Concurrency & Race Condition Prevention**
> "I am building an event ticketing system in Go where multiple users can register for events. The event has a limited capacity. How do I make sure two users don't both get the last ticket at the same time? I am using SQLite as my database. What is the safest way to handle this concurrency problem?"

---

**Prompt 2 — Writing the Concurrent Booking Test**
> "How do I write a Go unit test that simulates multiple goroutines all trying to register for the same event at the same time, to verify that overbooking does not happen? I want to use the standard testing package and httptest."

---

**Prompt 3 — SQLite Transactions in Go**
> "What is the difference between BEGIN, BEGIN IMMEDIATE and BEGIN EXCLUSIVE in SQLite? Which one should I use when I need to read and then update a row atomically in a Go application to avoid race conditions?"

---

**Prompt 4 — Token-Based Authentication Without External Libraries**
> "How do I implement a simple token-based login system in Go using only the standard library? I want to sign tokens using HMAC-SHA256 and validate them in a middleware function."
