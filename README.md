# Job Application Tracker API

A production-ready REST API written in pure Go to track job applications, manage interview timelines, reminders, and aggregate application metrics.

Built completely using standard library Go (`net/http`, `database/sql`, `encoding/json`, `log/slog`) with zero third-party web frameworks.

---

## 🌐 Live Deployments

- **Render**: [https://job-tracker-server-9drb.onrender.com/](https://job-tracker-server-9drb.onrender.com/)
- **Vercel**: [https://job-tracker-server-nu.vercel.app/](https://job-tracker-server-nu.vercel.app/)

---

## 🏗️ Architecture & Project Structure

```text
job-tracker/
├── cmd/
│   └── server/
│       └── main.go                     # Entrypoint & server orchestration
│
├── internal/
│   ├── application/                    # Application domain (CRUD, events, filtering)
│   │   ├── handler.go
│   │   ├── handler_test.go
│   │   ├── model.go
│   │   ├── postgres.go
│   │   ├── repository.go
│   │   ├── service.go
│   │   ├── validation.go
│   │   └── validation_test.go
│   │
│   ├── auth/                           # Authentication & session management
│   │   ├── auth_test.go
│   │   ├── handler.go
│   │   ├── handler_test.go
│   │   ├── service.go
│   │   └── session.go
│   │
│   ├── config/                         # Environment & application config
│   │   └── config.go
│   │
│   ├── database/                       # Database connection pooling & migrations
│   │   ├── database.go
│   │   └── migrate.go
│   │
│   ├── interview/                      # Interview scheduling domain
│   │   ├── handler.go
│   │   ├── model.go
│   │   ├── postgres.go
│   │   ├── repository.go
│   │   └── service.go
│   │
│   ├── middleware/                     # HTTP middleware
│   │   ├── auth.go                     # Context-based authentication
│   │   ├── logging.go                  # Structured request logging (slog)
│   │   ├── recovery.go                 # Panic recovery middleware
│   │   └── request_id.go               # Traceable X-Request-ID propagation
│   │
│   ├── reminder/                       # Task & follow-up reminders domain
│   │   ├── handler.go
│   │   ├── model.go
│   │   ├── postgres.go
│   │   ├── repository.go
│   │   ├── service.go
│   │   └── worker.go                   # Background reminder worker
│   │
│   ├── response/                       # JSON and error serialization helpers
│   │   ├── errors.go
│   │   └── json.go
│   │
│   ├── statistics/                     # Application metrics and rate calculations
│   │   ├── handler.go
│   │   ├── service.go
│   │   └── statistics_test.go
│   │
│   └── user/                           # User domain
│       ├── handler.go
│       ├── model.go
│       ├── postgres.go
│       ├── repository.go
│       └── service.go
│
├── migrations/                         # PostgreSQL DDL migrations
│   ├── 001_create_users.sql
│   ├── 002_create_applications.sql
│   ├── 003_create_application_events.sql
│   ├── 004_create_interviews.sql
│   ├── 005_create_reminders.sql
│   └── 006_create_sessions.sql
│
├── tests/                              # Integration and e2e tests
│   ├── api_integration_test.go
│   └── health_test.go
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

## 🚀 Quick Start

### 1. Run with Docker Compose (Recommended)

```bash
docker-compose up --build
```

The API will be accessible at `http://localhost:8080`.

### 2. Run Locally

```bash
# Copy sample environment configuration
cp .env.example .env

# Run database migrations in your PostgreSQL instance
psql "$DATABASE_URL" -f migrations/001_create_users.sql
psql "$DATABASE_URL" -f migrations/002_create_applications.sql
psql "$DATABASE_URL" -f migrations/003_create_application_events.sql
psql "$DATABASE_URL" -f migrations/004_create_interviews.sql
psql "$DATABASE_URL" -f migrations/005_create_reminders.sql
psql "$DATABASE_URL" -f migrations/006_create_sessions.sql

# Start the server
go run ./cmd/server
```

---

## 🧪 Testing

Run all unit and integration tests:

```bash
go test -v -race ./...
```

---

## 📖 API Endpoints Reference

### Welcome & Health Checks
- `GET /` - API welcome information and status
- `GET /health` - Liveness health check
- `GET /healthz` - Liveness health check (Kubernetes / Cloud provider standard)
- `GET /ready` - Readiness check with database verification

### Authentication & Profile
- `POST /api/v1/auth/register` - Create a new user account
- `POST /api/v1/auth/login` - Authenticate and obtain bearer token
- `POST /api/v1/auth/logout` - Invalidate current session token
- `GET /api/v1/me` - Get current authenticated user profile

### Applications
- `POST /api/v1/applications` - Create a new application
- `GET /api/v1/applications` - List applications (supports `status`, `company`, `location`, `from`, `to`, `sort`, `page`, `limit`)
- `GET /api/v1/applications/{id}` - Get application by ID
- `PATCH /api/v1/applications/{id}` - Partial update application
- `DELETE /api/v1/applications/{id}` - Delete application
- `GET /api/v1/applications/{id}/events` - Application status timeline

### Interviews
- `POST /api/v1/applications/{id}/interviews` - Schedule an interview
- `GET /api/v1/applications/{id}/interviews` - List interviews for an application
- `GET /api/v1/interviews/{id}` - Get interview details
- `PATCH /api/v1/interviews/{id}` - Update interview details
- `DELETE /api/v1/interviews/{id}` - Remove interview

### Reminders
- `POST /api/v1/reminders` - Create a follow-up reminder
- `GET /api/v1/reminders` - List all reminders
- `PATCH /api/v1/reminders/{id}` - Update reminder / mark completed
- `DELETE /api/v1/reminders/{id}` - Delete reminder

### Statistics
- `GET /api/v1/statistics` - Get aggregated metrics (response rate, interview rate, offer rate)
