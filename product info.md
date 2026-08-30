

## 🎯 Project: Job Application Tracker

A REST API where a user can track jobs they've applied to and manage the entire application lifecycle.

### Core concept

```text
User
 │
 └── Job Applications
       ├── Company
       ├── Position
       ├── Location
       ├── Salary
       ├── Job URL
       ├── Status
       ├── Applied Date
       └── Notes
```

### Statuses

```text
APPLIED
SCREENING
INTERVIEW
TECHNICAL_INTERVIEW
OFFER
REJECTED
WITHDRAWN
```

---

## API Design

### Applications

```http
POST   /api/applications
GET    /api/applications
GET    /api/applications/{id}
PATCH  /api/applications/{id}
DELETE /api/applications/{id}
```

### Filtering

```http
GET /api/applications?status=INTERVIEW

GET /api/applications?company=Google

GET /api/applications?location=Remote

GET /api/applications?from=2026-08-01&to=2026-08-30
```

### Pagination

```http
GET /api/applications?page=1&limit=20
```

### Statistics

```http
GET /api/statistics
```

Example:

```json
{
  "total": 42,
  "applied": 20,
  "screening": 5,
  "interviews": 10,
  "offers": 2,
  "rejected": 5
}
```

---

# Database

Start with two tables.

### `users`

```text
id
email
password_hash
created_at
updated_at
```

### `applications`

```text
id
user_id
company
position
location
job_url
salary_min
salary_max
status
applied_at
notes
created_at
updated_at
```

Relationship:

```text
users
  │
  │ 1
  │
  │ N
  ▼
applications
```

---

# Go Architecture

Since you don't want a framework, I'd structure it like this:

```text
job-tracker/
│
├── cmd/
│   └── server/
│       └── main.go
│
├── internal/
│   │
│   ├── application/
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── repository.go
│   │   └── model.go
│   │
│   ├── user/
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── repository.go
│   │   └── model.go
│   │
│   ├── auth/
│   │   ├── handler.go
│   │   ├── service.go
│   │   └── middleware.go
│   │
│   ├── middleware/
│   │   ├── logging.go
│   │   ├── recovery.go
│   │   └── auth.go
│   │
│   └── database/
│       └── database.go
│
├── migrations/
│   ├── 001_create_users.sql
│   └── 002_create_applications.sql
│
├── tests/
│
├── go.mod
└── README.md
```

The request flow would be:

```text
HTTP Request
     │
     ▼
net/http
     │
     ▼
Middleware
     │
     ▼
Handler
     │
     ▼
Service
     │
     ▼
Repository
     │
     ▼
Database
```

This is much better for learning than putting everything inside `main.go`.

---

# Use Standard Library

You can keep the HTTP side completely standard-library based:

```go
import (
    "context"
    "database/sql"
    "encoding/json"
    "errors"
    "log/slog"
    "net/http"
    "strconv"
    "time"
)
```

For routing, modern Go's `http.ServeMux` is enough:

```go
mux := http.NewServeMux()

mux.HandleFunc("POST /api/applications", handler.Create)
mux.HandleFunc("GET /api/applications", handler.List)
mux.HandleFunc("GET /api/applications/{id}", handler.Get)
mux.HandleFunc("PATCH /api/applications/{id}", handler.Update)
mux.HandleFunc("DELETE /api/applications/{id}", handler.Delete)
mux.HandleFunc("GET /api/statistics", handler.Statistics)
```

No Gin.

No Fiber.

No Echo.

No Chi.

---

# Authentication

Add authentication after the basic CRUD works.

```text
POST /api/auth/register
POST /api/auth/login
GET  /api/me
```

Flow:

```text
Register
   ↓
Hash password
   ↓
Store user
```

Login:

```text
Email + Password
       ↓
Verify password
       ↓
Create session/token
       ↓
Return authentication credential
```

Then:

```text
Authorization
      ↓
Auth Middleware
      ↓
Identify User
      ↓
Handler
```

One important learning goal: **every application query must be scoped to the authenticated user**.

---

# Nice Features to Add Later

Once the basic version works, add:

### ⭐ Application timeline

```text
Applied
   ↓
Screening
   ↓
Interview
   ↓
Technical Interview
   ↓
Offer
```

Store status changes in:

```text
application_events

id
application_id
old_status
new_status
note
created_at
```

Then:

```http
GET /api/applications/{id}/timeline
```

---

### ⭐ Interview tracking

Add:

```text
interviews

id
application_id
type
scheduled_at
location
notes
created_at
```

API:

```http
POST   /api/applications/{id}/interviews
GET    /api/applications/{id}/interviews
PATCH  /api/interviews/{id}
DELETE /api/interviews/{id}
```

---

### ⭐ Reminders

Eventually you could have:

```text
POST /api/reminders

{
  "application_id": 12,
  "remind_at": "2026-09-05T10:00:00Z",
  "message": "Follow up with recruiter"
}
```

This gives you an opportunity to learn **goroutines, timers, context cancellation, and background workers**.

---

# Development Roadmap

Don't build everything at once.

### Phase 1 — HTTP

```text
net/http
ServeMux
JSON
request validation
HTTP status codes
error responses
```

### Phase 2 — CRUD

```text
Create application
List applications
Get application
Update application
Delete application
```

### Phase 3 — PostgreSQL/SQLite

```text
database/sql
queries
transactions
indexes
migrations
```

### Phase 4 — Authentication

```text
register
login
password hashing
sessions/JWT
middleware
user isolation
```

### Phase 5 — Better API

```text
pagination
filtering
sorting
search
statistics
```

### Phase 6 — Advanced Go

```text
background jobs
reminders
goroutines
channels
context
timeouts
graceful shutdown
```

### Phase 7 — Production

```text
Docker
environment configuration
structured logging
health checks
tests
integration tests
graceful shutdown
```

---


