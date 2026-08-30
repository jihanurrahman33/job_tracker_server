

# Job Application Tracker API

**Version:** 1.0
**Type:** REST API
**Language:** Go
**HTTP:** `net/http`
**Database:** PostgreSQL
**Database API:** `database/sql`
**Architecture:** Handler → Service → Repository
**Authentication:** Session-based or JWT
**API format:** JSON

`database/sql` is Go's standard-library abstraction for SQL databases, although you'll still need a database driver because Go doesn't ship a PostgreSQL driver itself. `sql.DB` also provides a concurrency-safe database handle with connection pooling. ([Go.dev][2])

---

# 1. Project Overview

## Problem

Job seekers often apply to many companies and lose track of:

* where they applied
* which position they applied for
* application status
* interview dates
* recruiter information
* salary expectations
* follow-up dates
* rejection/offer information

The Job Application Tracker provides a centralized API for managing this information.

---

# 2. Main Goals

The project should demonstrate your ability to build a backend from the ground up using Go.

### Primary goals

* Build a REST API without a web framework
* Use `net/http`
* Use `encoding/json`
* Use `database/sql`
* Implement authentication
* Design a relational database
* Implement clean separation of concerns
* Handle validation and errors properly
* Implement pagination and filtering
* Write unit and integration tests
* Implement graceful server shutdown
* Add structured logging
* Run the application with Docker

---

# 3. Technology Restrictions

To make this a proper Go learning project:

### HTTP

Allowed:

```text
net/http
```

Not allowed:

```text
Gin
Fiber
Echo
Chi
Gorilla Mux
```

### JSON

```text
encoding/json
```

### Database

```text
database/sql
```

### Logging

```text
log/slog
```

### Configuration

```text
os
flag
```

### Testing

```text
testing
httptest
```

### Context

```text
context
```

### Cryptography/security

Use Go's standard cryptographic packages where appropriate.

A PostgreSQL driver is still necessary because `database/sql` is an abstraction and requires a driver. ([Go Packages][3])

---

# 4. User Roles

For version 1, keep this simple.

There is only one role:

```text
USER
```

A user can only access their own:

```text
applications
interviews
notes
reminders
statistics
```

For example:

```text
User A
 ├── Google application
 ├── Microsoft application
 └── Apple application

User B
 ├── Amazon application
 └── Meta application
```

User A must never be able to access User B's records.

---

# 5. Application Lifecycle

The application status system is central to the project.

```text
APPLIED
   │
   ▼
SCREENING
   │
   ▼
INTERVIEW
   │
   ▼
TECHNICAL_INTERVIEW
   │
   ├──────────────► REJECTED
   │
   ▼
OFFER
```

Other possible terminal states:

```text
WITHDRAWN
REJECTED
ACCEPTED
```

You don't have to enforce a strict state machine initially.

For example:

```text
APPLIED → REJECTED
APPLIED → INTERVIEW
INTERVIEW → OFFER
INTERVIEW → REJECTED
OFFER → ACCEPTED
```

---

# 6. Core Entities

The initial version should contain these entities:

```text
User
Application
ApplicationEvent
Interview
Reminder
```

You can initially implement only:

```text
User
Application
```

Then add the others.

---

# 7. Database Schema

## Users

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    name VARCHAR(100),
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);
```

---

# 8. Applications

```sql
CREATE TABLE applications (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,

    company VARCHAR(255) NOT NULL,
    position VARCHAR(255) NOT NULL,
    location VARCHAR(255),

    job_url TEXT,

    salary_min NUMERIC,
    salary_max NUMERIC,
    salary_currency VARCHAR(10),

    status VARCHAR(50) NOT NULL,

    applied_at TIMESTAMP,

    notes TEXT,

    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,

    FOREIGN KEY (user_id)
        REFERENCES users(id)
        ON DELETE CASCADE
);
```

---

# 9. Application Events

This stores the history of status changes.

```sql
CREATE TABLE application_events (
    id UUID PRIMARY KEY,

    application_id UUID NOT NULL,

    old_status VARCHAR(50),
    new_status VARCHAR(50),

    note TEXT,

    created_at TIMESTAMP NOT NULL,

    FOREIGN KEY (application_id)
        REFERENCES applications(id)
        ON DELETE CASCADE
);
```

Example:

```text
Application created
        ↓
APPLIED
        ↓
SCREENING
        ↓
INTERVIEW
        ↓
TECHNICAL_INTERVIEW
        ↓
OFFER
```

The database would preserve this history.

---

# 10. Interviews

```sql
CREATE TABLE interviews (
    id UUID PRIMARY KEY,

    application_id UUID NOT NULL,

    type VARCHAR(50) NOT NULL,

    scheduled_at TIMESTAMP NOT NULL,

    duration_minutes INTEGER,

    location TEXT,

    meeting_url TEXT,

    notes TEXT,

    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,

    FOREIGN KEY (application_id)
        REFERENCES applications(id)
        ON DELETE CASCADE
);
```

Possible interview types:

```text
PHONE_SCREEN
HR
TECHNICAL
BEHAVIORAL
SYSTEM_DESIGN
FINAL
OTHER
```

---

# 11. Reminders

```sql
CREATE TABLE reminders (
    id UUID PRIMARY KEY,

    user_id UUID NOT NULL,

    application_id UUID,

    title VARCHAR(255) NOT NULL,

    description TEXT,

    remind_at TIMESTAMP NOT NULL,

    completed BOOLEAN NOT NULL DEFAULT FALSE,

    created_at TIMESTAMP NOT NULL,

    FOREIGN KEY (user_id)
        REFERENCES users(id)
        ON DELETE CASCADE,

    FOREIGN KEY (application_id)
        REFERENCES applications(id)
        ON DELETE CASCADE
);
```

---

# 12. Indexes

This is an important part of the project.

```sql
CREATE INDEX idx_applications_user_id
ON applications(user_id);

CREATE INDEX idx_applications_status
ON applications(status);

CREATE INDEX idx_applications_company
ON applications(company);

CREATE INDEX idx_applications_applied_at
ON applications(applied_at);

CREATE INDEX idx_application_events_application_id
ON application_events(application_id);

CREATE INDEX idx_interviews_application_id
ON interviews(application_id);
```

Eventually you can think about composite indexes such as:

```sql
CREATE INDEX idx_applications_user_status
ON applications(user_id, status);
```

---

# 13. API Structure

Base URL:

```text
/api/v1
```

Therefore:

```text
/api/v1/auth/register
/api/v1/auth/login
/api/v1/applications
```

---

# 14. Authentication API

## Register

```http
POST /api/v1/auth/register
Content-Type: application/json
```

Request:

```json
{
  "name": "John Doe",
  "email": "john@example.com",
  "password": "password123"
}
```

Response:

```http
201 Created
```

```json
{
  "user": {
    "id": "uuid",
    "name": "John Doe",
    "email": "john@example.com"
  }
}
```

---

# 15. Login

```http
POST /api/v1/auth/login
```

Request:

```json
{
  "email": "john@example.com",
  "password": "password123"
}
```

Response:

```json
{
  "token": "..."
}
```

For a learning project, you can choose either:

### Option A

JWT authentication.

### Option B

Server-side sessions stored in PostgreSQL.

I'd recommend **sessions first** if your goal is backend fundamentals.

---

# 16. Current User

```http
GET /api/v1/me
```

Response:

```json
{
  "id": "uuid",
  "name": "John Doe",
  "email": "john@example.com"
}
```

---

# 17. Application API

## Create

```http
POST /api/v1/applications
```

Request:

```json
{
  "company": "Google",
  "position": "Software Engineer",
  "location": "Remote",
  "job_url": "https://example.com/job/123",
  "salary_min": 100000,
  "salary_max": 150000,
  "salary_currency": "USD",
  "status": "APPLIED",
  "applied_at": "2026-08-30T10:00:00Z",
  "notes": "Applied through referral."
}
```

Response:

```json
{
  "id": "uuid",
  "company": "Google",
  "position": "Software Engineer",
  "location": "Remote",
  "status": "APPLIED",
  "created_at": "2026-08-30T10:00:00Z"
}
```

---

# 18. List Applications

```http
GET /api/v1/applications
```

Response:

```json
{
  "data": [
    {
      "id": "uuid",
      "company": "Google",
      "position": "Software Engineer",
      "status": "INTERVIEW",
      "applied_at": "2026-08-20T10:00:00Z"
    },
    {
      "id": "uuid",
      "company": "Microsoft",
      "position": "Backend Engineer",
      "status": "APPLIED",
      "applied_at": "2026-08-25T10:00:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 2,
    "total_pages": 1
  }
}
```

---

# 19. Filtering

Support:

```text
?status=INTERVIEW
```

```text
?company=Google
```

```text
?location=Remote
```

```text
?from=2026-08-01
```

```text
?to=2026-08-30
```

Multiple:

```text
/api/v1/applications?status=INTERVIEW&location=Remote
```

---

# 20. Sorting

```text
?sort=applied_at
```

Descending:

```text
?sort=-applied_at
```

Possible fields:

```text
created_at
updated_at
applied_at
company
status
```

**Important:** never directly concatenate arbitrary user input into SQL.

Instead, whitelist:

```go
allowedSorts := map[string]string{
    "company":    "company",
    "status":     "status",
    "applied_at": "applied_at",
    "created_at": "created_at",
}
```

---

# 21. Get Application

```http
GET /api/v1/applications/{id}
```

Response:

```json
{
  "id": "uuid",
  "company": "Google",
  "position": "Software Engineer",
  "location": "Remote",
  "job_url": "https://example.com/job",
  "salary_min": 100000,
  "salary_max": 150000,
  "salary_currency": "USD",
  "status": "INTERVIEW",
  "applied_at": "2026-08-20T10:00:00Z",
  "notes": "Passed initial screening.",
  "created_at": "2026-08-20T10:00:00Z",
  "updated_at": "2026-08-29T10:00:00Z"
}
```

---

# 22. Update Application

```http
PATCH /api/v1/applications/{id}
```

Request:

```json
{
  "status": "INTERVIEW",
  "notes": "Technical interview scheduled."
}
```

The server should update only the supplied fields.

---

# 23. Delete Application

```http
DELETE /api/v1/applications/{id}
```

Response:

```http
204 No Content
```

---

# 24. Status History

```http
GET /api/v1/applications/{id}/events
```

Response:

```json
{
  "data": [
    {
      "old_status": null,
      "new_status": "APPLIED",
      "created_at": "2026-08-20T10:00:00Z"
    },
    {
      "old_status": "APPLIED",
      "new_status": "SCREENING",
      "created_at": "2026-08-22T14:00:00Z"
    },
    {
      "old_status": "SCREENING",
      "new_status": "INTERVIEW",
      "created_at": "2026-08-25T16:00:00Z"
    }
  ]
}
```

---

# 25. Interview API

```http
POST   /api/v1/applications/{id}/interviews
GET    /api/v1/applications/{id}/interviews

GET    /api/v1/interviews/{id}
PATCH  /api/v1/interviews/{id}
DELETE /api/v1/interviews/{id}
```

Create:

```json
{
  "type": "TECHNICAL",
  "scheduled_at": "2026-09-03T15:00:00Z",
  "duration_minutes": 60,
  "meeting_url": "https://meet.example.com/abc",
  "notes": "Prepare system design."
}
```

---

# 26. Reminder API

```http
POST   /api/v1/reminders
GET    /api/v1/reminders
PATCH  /api/v1/reminders/{id}
DELETE /api/v1/reminders/{id}
```

Example:

```json
{
  "application_id": "uuid",
  "title": "Follow up with recruiter",
  "description": "Send follow-up email.",
  "remind_at": "2026-09-05T10:00:00Z"
}
```

---

# 27. Statistics API

```http
GET /api/v1/statistics
```

Response:

```json
{
  "total_applications": 50,
  "by_status": {
    "APPLIED": 20,
    "SCREENING": 8,
    "INTERVIEW": 10,
    "TECHNICAL_INTERVIEW": 5,
    "OFFER": 2,
    "REJECTED": 5
  },
  "response_rate": 0.60,
  "interview_rate": 0.30,
  "offer_rate": 0.04
}
```

You can calculate:

```text
response rate =
(non-applied applications / total applications)

interview rate =
(interviews / total applications)

offer rate =
(offers / total applications)
```

---

# 28. HTTP Routing

Your router can be completely standard library:

```go
mux := http.NewServeMux()

mux.HandleFunc("POST /api/v1/auth/register", authHandler.Register)
mux.HandleFunc("POST /api/v1/auth/login", authHandler.Login)

mux.HandleFunc("GET /api/v1/me", authHandler.Me)

mux.HandleFunc("POST /api/v1/applications", applicationHandler.Create)
mux.HandleFunc("GET /api/v1/applications", applicationHandler.List)
mux.HandleFunc("GET /api/v1/applications/{id}", applicationHandler.Get)
mux.HandleFunc("PATCH /api/v1/applications/{id}", applicationHandler.Update)
mux.HandleFunc("DELETE /api/v1/applications/{id}", applicationHandler.Delete)

mux.HandleFunc("GET /api/v1/statistics", statisticsHandler.Get)
```

Go's current `ServeMux` supports method + wildcard patterns and `Request.PathValue`, which makes this style practical without a router framework. ([Go.dev][1])

---

# 29. Handler Layer

The handler should deal with HTTP concerns only.

Example responsibility:

```text
HTTP request
    ↓
decode JSON
    ↓
validate request
    ↓
call service
    ↓
convert result → JSON
    ↓
HTTP response
```

It should **not** contain SQL.

Bad:

```go
func CreateApplication(w http.ResponseWriter, r *http.Request) {
    // parse request
    // validation
    // SQL
    // business logic
    // response
}
```

Better:

```text
Handler
   ↓
Service
   ↓
Repository
   ↓
Database
```

---

# 30. Service Layer

The service contains business rules.

Example:

```go
type ApplicationService struct {
    repo ApplicationRepository
}
```

Methods:

```go
Create(...)
Get(...)
List(...)
Update(...)
Delete(...)
ChangeStatus(...)
```

Example business rule:

```text
Application doesn't belong to current user
        ↓
return ErrForbidden
```

Another:

```text
Invalid status
        ↓
return ErrInvalidStatus
```

---

# 31. Repository Layer

The repository owns database operations.

```go
type ApplicationRepository interface {
    Create(ctx context.Context, app *Application) error
    GetByID(ctx context.Context, userID, id string) (*Application, error)
    List(ctx context.Context, userID string, filter Filter) ([]Application, error)
    Update(ctx context.Context, userID, id string, input UpdateInput) error
    Delete(ctx context.Context, userID, id string) error
}
```

This is where `database/sql` belongs.

For queries returning multiple rows, use `QueryContext`; for single rows, `QueryRowContext`; for INSERT/UPDATE/DELETE, use `ExecContext`. ([Go.dev][4])

---

# 32. Models

Keep database/domain models separate from HTTP request models when appropriate.

### Application

```go
type Application struct {
    ID             string
    UserID         string
    Company        string
    Position       string
    Location       string
    JobURL         string
    SalaryMin      *float64
    SalaryMax      *float64
    SalaryCurrency string
    Status         Status
    AppliedAt      *time.Time
    Notes          string
    CreatedAt      time.Time
    UpdatedAt      time.Time
}
```

---

# 33. Request Models

```go
type CreateApplicationRequest struct {
    Company        string   `json:"company"`
    Position       string   `json:"position"`
    Location       string   `json:"location"`
    JobURL         string   `json:"job_url"`
    SalaryMin      *float64 `json:"salary_min"`
    SalaryMax      *float64 `json:"salary_max"`
    SalaryCurrency string   `json:"salary_currency"`
    Status         string   `json:"status"`
    AppliedAt      string   `json:"applied_at"`
    Notes          string   `json:"notes"`
}
```

---

# 34. Validation

Validate:

### Company

Required.

```text
1–255 characters
```

### Position

Required.

```text
1–255 characters
```

### Status

Must be one of:

```text
APPLIED
SCREENING
INTERVIEW
TECHNICAL_INTERVIEW
OFFER
REJECTED
WITHDRAWN
ACCEPTED
```

### Salary

```text
salary_min >= 0
salary_max >= salary_min
```

### URL

If provided, should be a valid URL.

---

# 35. Error Format

Use one consistent error structure.

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid request",
    "details": {
      "company": "company is required"
    }
  }
}
```

Possible error codes:

```text
VALIDATION_ERROR
UNAUTHORIZED
FORBIDDEN
NOT_FOUND
CONFLICT
INVALID_STATUS
INTERNAL_ERROR
```

---

# 36. HTTP Status Codes

Use them consistently.

```text
200 OK
201 Created
204 No Content

400 Bad Request
401 Unauthorized
403 Forbidden
404 Not Found
409 Conflict
422 Unprocessable Entity
500 Internal Server Error
```

---

# 37. Middleware

Implement your own middleware.

### Logging

```text
Request
   ↓
Logger
   ↓
Handler
```

Log:

```text
method
path
status
duration
request_id
```

Example:

```text
INFO request completed
method=POST
path=/api/v1/applications
status=201
duration=14ms
```

---

# 38. Request ID

Generate a request ID for every request.

```text
Request
   ↓
X-Request-ID
   ↓
Handler
   ↓
Logs
```

Response:

```http
X-Request-ID: 4f7c...
```

This becomes extremely useful when debugging.

---

# 39. Recovery Middleware

Protect the HTTP server from panics.

```text
panic
  ↓
recovery middleware
  ↓
log error
  ↓
500 response
```

Never expose internal panic details to the client.

---

# 40. Authentication Middleware

```text
Request
   ↓
Authorization
   ↓
Validate session/token
   ↓
User ID
   ↓
Context
   ↓
Handler
```

Store the authenticated user ID in the request context.

```go
ctx := context.WithValue(
    r.Context(),
    userIDKey,
    userID,
)
```

Then:

```go
userID, ok := UserIDFromContext(r.Context())
```

---

# 41. Important Security Rule

Every application query must include the authenticated user's ID.

Don't do:

```sql
SELECT *
FROM applications
WHERE id = $1;
```

Prefer:

```sql
SELECT *
FROM applications
WHERE id = $1
AND user_id = $2;
```

This prevents one user from accessing another user's application simply by changing the UUID.

---

# 42. Transactions

Changing application status should create an event at the same time.

These two operations should succeed or fail together:

```text
UPDATE application status
        +
INSERT application event
```

Therefore:

```text
BEGIN
   ↓
UPDATE applications
   ↓
INSERT application_events
   ↓
COMMIT
```

If something fails:

```text
ROLLBACK
```

Go's `database/sql` supports transactions through `sql.Tx`. ([Go Packages][3])

---

# 43. Project Structure

I'd use this final structure:

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
│   │   ├── model.go
│   │   ├── repository.go
│   │   ├── postgres.go
│   │   ├── service.go
│   │   ├── handler.go
│   │   └── validation.go
│   │
│   ├── user/
│   │   ├── model.go
│   │   ├── repository.go
│   │   ├── postgres.go
│   │   ├── service.go
│   │   └── handler.go
│   │
│   ├── auth/
│   │   ├── service.go
│   │   ├── handler.go
│   │   ├── session.go
│   │   └── middleware.go
│   │
│   ├── interview/
│   │   ├── model.go
│   │   ├── repository.go
│   │   ├── postgres.go
│   │   ├── service.go
│   │   └── handler.go
│   │
│   ├── reminder/
│   │   ├── model.go
│   │   ├── repository.go
│   │   ├── postgres.go
│   │   ├── service.go
│   │   └── handler.go
│   │
│   ├── statistics/
│   │   ├── service.go
│   │   └── handler.go
│   │
│   ├── middleware/
│   │   ├── logging.go
│   │   ├── recovery.go
│   │   ├── request_id.go
│   │   └── auth.go
│   │
│   ├── database/
│   │   └── database.go
│   │
│   └── response/
│       ├── json.go
│       └── errors.go
│
├── migrations/
│   ├── 001_create_users.sql
│   ├── 002_create_applications.sql
│   ├── 003_create_application_events.sql
│   ├── 004_create_interviews.sql
│   └── 005_create_reminders.sql
│
├── tests/
│
├── .env.example
├── .gitignore
├── Dockerfile
├── docker-compose.yml
├── go.mod
├── go.sum
└── README.md
```

---

# 44. Server Configuration

Don't hardcode:

```go
":8080"
```

Create configuration:

```go
type Config struct {
    Port            string
    DatabaseURL     string
    SessionSecret   string
    Environment     string
    ReadTimeout     time.Duration
    WriteTimeout    time.Duration
    IdleTimeout     time.Duration
}
```

Environment:

```text
PORT=8080

DATABASE_URL=postgres://...

SESSION_SECRET=...

ENVIRONMENT=development
```

---

# 45. HTTP Server

Configure timeouts.

```go
server := &http.Server{
    Addr:         ":" + cfg.Port,
    Handler:      handler,
    ReadTimeout:  10 * time.Second,
    WriteTimeout: 15 * time.Second,
    IdleTimeout:  60 * time.Second,
}
```

Don't simply rely on:

```go
http.ListenAndServe(...)
```

for your final production-style version.

---

# 46. Graceful Shutdown

Your application should handle:

```text
SIGINT
SIGTERM
```

Flow:

```text
Signal
  ↓
Stop accepting requests
  ↓
Wait for active requests
  ↓
Close database
  ↓
Exit
```

Use:

```go
signal.NotifyContext(...)
```

and:

```go
server.Shutdown(ctx)
```

---

# 47. Health Checks

Implement:

```http
GET /health
```

Response:

```json
{
  "status": "ok"
}
```

And:

```http
GET /ready
```

The readiness endpoint can verify the database:

```text
Application
     │
     ├── HTTP server ✓
     │
     └── PostgreSQL ✓
```

---

# 48. Testing

You should have several types of tests.

### Unit tests

Test:

```text
validation
status transitions
statistics
pagination
services
```

### Handler tests

Use:

```go
httptest.NewRequest(...)
httptest.NewRecorder()
```

Test:

```text
POST /applications
GET /applications
GET /applications/{id}
PATCH /applications/{id}
DELETE /applications/{id}
```

### Integration tests

Test:

```text
HTTP
 ↓
Handler
 ↓
Service
 ↓
Repository
 ↓
PostgreSQL
```

---

# 49. Example Test Cases

### Create application

```text
valid request → 201
missing company → 400
missing position → 400
invalid status → 400
unauthenticated → 401
```

### Get application

```text
existing application → 200
unknown ID → 404
another user's ID → 404/403
invalid UUID → 400
```

### Update status

```text
APPLIED → INTERVIEW → success

APPLIED → INVALID_STATUS → 400
```

### Delete

```text
owner → 204
non-owner → 404/403
unknown ID → 404
```

---

# 50. Docker

Your development environment:

```text
┌────────────────────────┐
│       Go API            │
│       :8080             │
└───────────┬────────────┘
            │
            ▼
┌────────────────────────┐
│      PostgreSQL         │
│       :5432             │
└────────────────────────┘
```

`docker-compose.yml`:

```text
api
postgres
```

You don't need Redis, Kafka, Kubernetes, etc. for version 1.

---

# 51. API Version 1 Scope

Don't implement everything immediately.

### MVP

```text
Authentication
    ↓
Applications CRUD
    ↓
Filtering
    ↓
Pagination
    ↓
Statistics
```

### V1.1

```text
Application events
    ↓
Interviews
```

### V1.2

```text
Reminders
    ↓
Background worker
```

### V2

```text
Email notifications
Advanced statistics
CSV export
Job search integration
Multiple resumes
Companies
Contacts
```

---

# 52. Advanced Background Worker

Once the core API works, this becomes a great Go exercise.

For reminders:

```text
PostgreSQL
    │
    │ every few seconds
    ▼
Worker
    │
    ├── find pending reminders
    │
    ▼
Process
    │
    ▼
Mark completed
```

You can use:

```go
ticker := time.NewTicker(...)
```

and a goroutine.

Later, add graceful worker shutdown using `context.Context`.

---

# 53. Final Architecture

The complete system becomes:

```text
                     ┌───────────────┐
                     │    Client     │
                     └───────┬───────┘
                             │
                             ▼
                     ┌───────────────┐
                     │   net/http    │
                     │   ServeMux    │
                     └───────┬───────┘
                             │
                     ┌───────▼───────┐
                     │  Middleware   │
                     │               │
                     │ Auth           │
                     │ Logging        │
                     │ Request ID     │
                     │ Recovery       │
                     └───────┬───────┘
                             │
                             ▼
                     ┌───────────────┐
                     │   Handler     │
                     └───────┬───────┘
                             │
                             ▼
                     ┌───────────────┐
                     │   Service     │
                     │ Business Logic│
                     └───────┬───────┘
                             │
                             ▼
                     ┌───────────────┐
                     │  Repository   │
                     └───────┬───────┘
                             │
                             ▼
                     ┌───────────────┐
                     │  PostgreSQL   │
                     └───────────────┘

                             ▲
                             │
                     ┌───────┴───────┐
                     │ Background    │
                     │ Worker        │
                     │               │
                     │ Reminders     │
                     └───────────────┘
```

## Recommended build order

**Don't start by creating all these files.** Build vertically:

```text
Day 1
├── project setup
├── net/http server
├── ServeMux
├── health endpoint
└── JSON response helper

Day 2
├── PostgreSQL
├── migrations
├── User model
└── Application model

Day 3
├── Create application
├── Get application
├── List applications
└── Delete application

Day 4
├── Update application
├── validation
├── filtering
└── pagination

Day 5
├── registration
├── login
├── sessions
└── auth middleware

Day 6
├── status history
├── transactions
└── statistics

Day 7
├── interviews
├── tests
├── logging
└── graceful shutdown
```


