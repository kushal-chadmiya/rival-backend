# Rival Task Manager — Backend

Go REST API for the Rival task management assessment. Handles authentication, task CRUD, file attachments, activity logging, and realtime updates over SSE.

Companion frontend: Next.js app in the sibling `frontend/` directory.

## Stack

- **Go 1.26** with [chi](https://github.com/go-chi/chi) router
- **PostgreSQL** via Supabase (pgx connection pool)
- **Supabase Auth** — signup/login proxied to Supabase; JWT verification via JWKS or shared secret
- **Supabase Storage** — task file attachments (optional, requires service role key)

## Features

- Email/password auth (`/auth/signup`, `/auth/login`, `/auth/refresh`)
- JWT-protected task CRUD with per-user isolation
- Admin RBAC — users with `app_metadata.role=admin` can list all tasks
- Task filtering, search, sorting, and pagination
- Task activity log per task
- Multipart file upload/download for attachments
- Server-Sent Events hub for live task updates (`GET /tasks/events`)
- Auto-applies `db/schema.sql` on startup

## Project layout

```text
backend/
├── cmd/
│   ├── api/          # HTTP server entrypoint
│   └── migrate/      # Standalone schema migration CLI
├── db/
│   ├── schema.sql    # Postgres schema
│   └── migrate.go    # Migration runner used at startup
├── internal/
│   ├── activity/     # Task activity store
│   ├── app/          # Application bootstrap
│   ├── auth/         # JWT middleware, Supabase auth client
│   ├── config/       # Environment configuration
│   ├── httpapi/      # HTTP handlers and router
│   ├── realtime/     # SSE hub
│   ├── storage/      # Supabase Storage client
│   ├── store/        # Postgres task/attachment queries
│   └── tasks/        # Domain types
├── Dockerfile
└── .env.example
```

## Prerequisites

- Go 1.26+
- A [Supabase](https://supabase.com) project with:
  - Database connection string (Session pooler recommended if direct IPv6 fails)
  - Anon key and optional service role key
  - Storage bucket named `task-attachments` (for file uploads)

## Quick start

### 1. Configure environment

```bash
cp .env.example .env
```

Edit `.env` with your Supabase credentials. See [Environment variables](#environment-variables) below.

### 2. Run the API

```bash
go mod tidy
go run ./cmd/api
```

The server listens on `http://localhost:8080` by default. Schema migrations run automatically on boot.

### 3. Health check

```bash
curl http://localhost:8080/health
# {"status":"ok"}
```

## Environment variables

| Variable | Required | Description |
|----------|----------|-------------|
| `PORT` | No | HTTP port (default `8080`) |
| `FRONTEND_URL` | No | CORS origin (default `http://localhost:3000`) |
| `DATABASE_URL` | Yes | Postgres connection string — use the **Session pooler** URI from Supabase if the direct host is unreachable |
| `SUPABASE_URL` | Yes | `https://<project-ref>.supabase.co` |
| `SUPABASE_ANON_KEY` | Yes | Supabase anon/public key |
| `SUPABASE_SERVICE_ROLE_KEY` | For uploads | Server-side key for Supabase Storage |
| `SUPABASE_JWT_SECRET` | Optional | Shared JWT secret (alternative to JWKS) |
| `SUPABASE_JWKS_URL` | Optional | JWKS endpoint (derived from `SUPABASE_URL` if empty) |
| `STORAGE_BUCKET` | No | Storage bucket name (default `task-attachments`) |
| `MAX_UPLOAD_BYTES` | No | Max attachment size in bytes (default `10485760`) |
| `ALLOW_VIEW_AS_ADMIN` | No | Allow `X-View-Role: admin` header for demo/assessment (default `true`) |

## Supabase setup

1. Run `db/schema.sql` in the Supabase SQL editor (also applied automatically on API startup).
2. Create a **public** Storage bucket named `task-attachments`.
3. To grant admin access, set a user's `app_metadata.role` to `admin` in the Supabase dashboard.
4. For local dev, consider disabling email confirmation under **Authentication → Providers → Email**.

## API reference

All protected routes require `Authorization: Bearer <access_token>`.

### Auth

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/auth/signup` | Create account |
| `POST` | `/auth/login` | Sign in |
| `POST` | `/auth/refresh` | Refresh session |
| `GET` | `/auth/me` | Current user profile and role |

### Tasks

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/tasks` | List tasks (filter, search, sort, paginate) |
| `POST` | `/tasks` | Create task |
| `GET` | `/tasks/{id}` | Get task |
| `PATCH` | `/tasks/{id}` | Update task |
| `DELETE` | `/tasks/{id}` | Delete task |

**`GET /tasks` query parameters**

| Param | Values |
|-------|--------|
| `status` | `todo`, `in_progress`, `completed` |
| `search` | Title substring |
| `sort_by` | `created_at`, `due_date`, `priority` |
| `sort_dir` | `asc`, `desc` |
| `page` | Page number (1-based) |
| `page_size` | 1–50 |

### Activity

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/tasks/{id}/activity` | Activity timeline for a task |

### Attachments

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/tasks/{id}/attachments` | List attachments |
| `POST` | `/tasks/{id}/attachments` | Upload file (`multipart/form-data`, field `file`) |
| `DELETE` | `/tasks/{id}/attachments/{attachmentId}` | Delete attachment |

### Realtime

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/tasks/events?access_token=<token>` | SSE stream of task change events |

## Docker

Build and run the API image:

```bash
docker build -t rival-api .
docker run --env-file .env -p 8080:8080 rival-api
```

To run the full stack (API + frontend), use Docker Compose from the parent directory:

```bash
cd ..
docker compose up --build
```

## Tests

```bash
go test ./...
```

## Manual migration

If you prefer to run migrations separately:

```bash
go run ./cmd/migrate
```
