# Xazavier

**Personal activity ledger: record, verify, observe, reflect.**

![CI](https://github.com/your-org/xazavier/actions/workflows/ci.yml/badge.svg)

Xazavier is a personal activity tracking application built with React, Go, and PostgreSQL. It helps users organize activities, maintain daily logs, track progress, and review changes through an audit trail.

## Features

- User registration and authentication
- Activity and cluster management
- Daily activity logging
- Automatic activity status and timestamp management
- Audit trail for tracking record changes
- PostgreSQL persistence and database migrations
- In-memory development mode
- Docker-based deployment

## Tech Stack

| Layer | Technology |
|---|---|
| Frontend | React 18, Vite 8 |
| Backend | Go 1.24 |
| HTTP Router | Chi |
| Database | PostgreSQL 16 |
| Database Driver | pgx |
| Authentication | JWT, Argon2id |
| Deployment | Docker, Docker Compose |

## Project Structure

```text
xazavier/
├── cmd/
│   ├── server/          # API server
│   └── seed/            # Demo data seeder
├── internal/
│   ├── apperr/          # Application errors
│   ├── auth/            # Authentication utilities
│   ├── config/          # Environment configuration
│   ├── domain/          # Domain models and context keys
│   ├── httpapi/         # HTTP routes and handlers
│   ├── repository/      # Memory and PostgreSQL repositories
│   └── service/         # Application business logic
├── migrations/          # PostgreSQL migrations
├── scripts/             # Smoke tests and utilities
├── src/                 # React frontend
├── Dockerfile
├── docker-compose.yml
├── go.mod
├── package.json
└── README.md
```

## Requirements

- Go 1.24 or newer
- Node.js 22.12 or newer
- npm
- PostgreSQL 16 (for persistent storage)
- Docker Desktop and Docker Compose (for containerized setup)

## Getting Started

### 1. Clone the repository

```bash
git clone <repository-url>
cd xazavier
```

### 2. Configure environment variables

Copy `.env.example` to `.env`.

```bash
cp .env.example .env   # Linux / macOS
# Windows (PowerShell): Copy-Item .env.example .env
```

Update `JWT_SECRET` with a strong random secret of at least 32 characters.

Do not commit `.env` or expose its contents.

### 3. Start PostgreSQL

You can use an existing PostgreSQL instance or Docker Compose.

For a local PostgreSQL instance, configure `DATABASE_URL` in `.env`.

### 4. Run database migrations

The API supports automatic migrations through `AUTO_MIGRATE=true`.

Alternatively, start the API and let it apply migrations during initialization.

### 5. Start the backend

```bash
go mod download
go run ./cmd/server
```

The API listens on port `8080` by default.

### 6. Start the frontend

Open another terminal:

```bash
npm ci
npm run dev
```

Vite starts the frontend at `http://localhost:5173`.

By default, development mode uses the mock API when `VITE_API_URL` is not configured.

To connect the frontend to the Go API, configure `VITE_API_URL` with the backend URL and ensure the backend CORS configuration allows the frontend origin.

### 7. Build the frontend

```bash
npm run build
```

Production assets are generated in `dist/`.

## Docker Deployment

Docker Compose starts the PostgreSQL database and Go API.

1. Configure `.env`.
2. Set a strong `JWT_SECRET`.
3. Start the services:

```bash
docker compose up -d --build
```

4. Check service status:

```bash
docker compose ps
```

5. Verify the API:

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

6. Stop the services:

```bash
docker compose down
```

**Important:** `docker compose down -v` removes named volumes, including the PostgreSQL data volume. Do not use it unless you intentionally want to delete the database data.

The Docker Compose configuration currently deploys the backend and database. The frontend must be built and deployed separately.

## Environment Variables

| Variable | Description |
|---|---|
| `PORT` | API listening port |
| `APP_ENV` | Application environment |
| `DATABASE_URL` | PostgreSQL connection string or `memory` |
| `JWT_SECRET` | JWT signing secret (minimum 32 characters) |
| `ACCESS_TOKEN_TTL` | Access token lifetime |
| `REFRESH_TOKEN_TTL` | Refresh token lifetime |
| `COOKIE_SECURE` | Enable secure cookies under HTTPS |
| `CORS_ALLOWED_ORIGINS` | Allowed frontend origins |
| `AUTO_MIGRATE` | Automatically apply database migrations |
| `LOG_LEVEL` | Logging verbosity |
| `TRUSTED_PROXY` | Trusted proxy IP or CIDR |
| `VITE_API_URL` | Frontend API base URL |
| `POSTGRES_DB` | Docker PostgreSQL database name |
| `POSTGRES_USER` | Docker PostgreSQL username |
| `POSTGRES_PASSWORD` | Docker PostgreSQL password |
| `TEST_DATABASE_URL` | Dedicated PostgreSQL database for integration tests |

## Testing and Quality Checks

### Backend

```bash
go vet ./...
go test ./...
```

Integration tests require `TEST_DATABASE_URL` pointing to a dedicated test database. Tests skip database integration when this variable is not configured.

Never point integration tests at a production database.

### Frontend

```bash
npm ci
npm audit
npm run build
```

### Docker Smoke Test

The smoke test builds and starts an isolated Docker Compose project, then checks API health and readiness.

Run using Bash:

```bash
bash ./scripts/smoke.sh
```

The smoke test uses the `xazavier-smoke` Compose project and preserves its PostgreSQL volume during cleanup.

## API Endpoints

| Method | Endpoint | Description |
|---|---|---|
| POST | `/api/auth/register` | Register a user |
| POST | `/api/auth/login` | Authenticate a user |
| POST | `/api/auth/refresh` | Refresh authentication |
| POST | `/api/auth/logout` | End authentication |
| GET | `/api/me` | Get current user |
| GET / POST | `/api/clusters` | List or create clusters |
| PUT / DELETE | `/api/clusters/{id}` | Update or delete a cluster |
| GET / POST | `/api/activities` | List or create activities |
| PUT / DELETE | `/api/activities/{id}` | Update or delete an activity |
| GET | `/api/logs` | List daily logs |
| GET | `/api/logs/{id}` | Get a daily log |
| POST / PUT / DELETE | `/api/logs/{id}` | Create, update, or delete a log |
| GET | `/api/logs/{id}/audit` | Get audit history for a log |
| GET | `/api/audit` | Get user audit history |
| POST | `/api/reset` | Reset user data |

> **Note:** `/api/logs` and `/api/audit` support `limit` and `offset` query parameters for pagination. Auth endpoints are rate-limited to 10 requests per minute per IP.

Health endpoints:

- `GET /healthz`
- `GET /readyz`

## Security Notes

- Use a strong, randomly generated JWT secret.
- Never commit `.env`, credentials, or private keys.
- Use HTTPS in production.
- Enable secure cookies when serving over HTTPS.
- Restrict `CORS_ALLOWED_ORIGINS` to trusted frontend origins.
- Refresh tokens are transmitted exclusively via HttpOnly cookies and are never included in JSON responses.
- Do not use the in-memory demo store in production.
- Use a dedicated database for integration testing.
- Back up PostgreSQL data before destructive database operations.
- Do not expose database credentials or demo accounts publicly.

## Development Notes

- Database schema changes are maintained in `migrations/`.
- PostgreSQL and in-memory repositories implement the repository layer.
- Business logic belongs in `internal/service/`.
- HTTP routing and handlers belong in `internal/httpapi/`.
- Frontend API communication is centralized in `src/services/api.js`.

## License

See the `LICENSE` file for licensing information.