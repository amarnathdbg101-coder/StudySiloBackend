# 📚 Complete File & Directory Index (Sabhi Files Ka Use Case)

Is document me project ki **har ek file aur directory ka exact role, purpose, aur use case** detail me explain kiya gaya hai, taaki GitHub se koi bhi naya developer isse aasani se samajh sake.

---

## 📂 Root Files & Configurations

| File / Folder | Role & Purpose | Kab Modify Karein? |
|---|---|---|
| `.env.example` | Environment variables ka public template (Port, DB URL, JWT secrets). | Jab naya environment variable add karna ho. |
| `.env` | Local secrets file (Git me commit nahi hoti). | Local DB password ya JWT key badalne ke liye. |
| `.gitignore` | Unwanted files (binaries, `.env`, logs, temp files) ko Git me jane se rokta hai. | Nayi temporary files ko ignore karne ke liye. |
| `.air.toml` | Live Hot-Reload tool (`air`) ki configuration file. | File watch delay ya watched extensions badalne ke liye. |
| `Dockerfile` | Multi-stage production container build recipe (Go binary compile karke lightweight Alpine image banata hai). | Cloud deployment ya production containerize karne ke liye. |
| `docker-compose.yml` | 1-Click local setup file jo PostgreSQL 16 aur Redis 7 containers spin-up karti hai. | Nayi service (e.g. MinIO) add karne ya ports change karne ke liye. |
| `.dockerignore` | Build context se heavy/secret files (`bin/`, `.git/`, `.env`) ko Docker me copy hone se rokta hai. | Docker build speed optimize karne ke liye. |
| `run-backend.bat` | Windows users ke liye 1-click launcher shortcut. | Native Windows execution ke liye. |
| `taskfile.yaml` | Task runner recipes (`task run`, `task test`, `task build`, `task seed`, `task docker:up`). | Naye shortcuts / commands add karne ke liye. |
| `go.mod` & `go.sum` | Go module declaration aur exact dependency versions lockfile. | Nayi third-party library install karne par (`go get`). |
| `README.md` | GitHub repository ka main landing page aur overview. | Project description aur badges update karne ke liye. |
| `ARCHITECTURE.md` | Clean 6-layer architecture ka complete design guide. | Architecture guidelines reference ke liye. |

---

## 🤖 `.github/` (CI/CD Automation)

| Path | Purpose |
|---|---|
| `.github/workflows/ci.yml` | **GitHub Actions CI Workflow:** Jab bhi code GitHub pe push hota hai, ye automated robot code checkout karta hai, Postgres container banata hai, `go vet` linting karta hai, `go test` chalata hai, aur binaries build karke verify karta hai. |

---

## 🚀 `cmd/` (Application Entrypoints)

| File | Purpose |
|---|---|
| `cmd/api/main.go` | **Main Server Entrypoint:** Config load karta hai, Zap Logger banata hai, PostgreSQL pool connect karta hai, Auto-migrations check karta hai, Router wire karta hai, Background workers start karta hai, aur Graceful OS Shutdown handle karta hai. |
| `cmd/migrate/main.go` | **Standalone Migration CLI Tool:** Command line se migrations chalane ke liye (`go run ./cmd/migrate up` ya `down`). |
| `cmd/seed/main.go` | **Database Seeder Tool:** Initial database state aur default admin user (`admin@example.com` / `Admin@123456`) create karne ke liye. |

---

## 📖 `docs/` (Interactive Swagger OpenAPI Documentation)

| File | Purpose |
|---|---|
| `docs/docs.go` | Go code me embedded OpenAPI 2.0 / Swagger JSON specifications. |
| `docs/swagger.json` | JSON format me API documentation schema. |
| `docs/swagger.yaml` | YAML format me API documentation schema. |
*Ye files browser me `/swagger/index.html` par interactive testing UI provide karti hain.*

---

## 🏢 `internal/handler/` (Business Domain Layer)

Ye folder project ka core business logic hold karta hai:

### 1. `internal/handler/model/` (Database Entities)
- **`user.go`**: `users` table ka Go struct representation (`ID`, `Name`, `Email`, `Password`, `Role`, `CreatedAt`, etc.).

### 2. `internal/handler/dto/` (Data Transfer Objects)
- **`auth_dto.go`**: Request & Response schemas with validation tags (e.g. `RegisterRequest`, `LoginRequest`, `AuthResponse`).

### 3. `internal/handler/repository/` (Raw SQL Queries)
- **`user_repository.go`**: High-performance parameterized SQL queries (`FindByEmail`, `Create`, `FindByID`) using `pgxpool.Pool`.

### 4. `internal/handler/services/` (Business Logic)
- **`auth_service.go`**: Authentication rules, password verification via Bcrypt, JWT token issuance, aur registration workflows.

### 5. `internal/handler/controller/` (HTTP Handlers)
- **`auth_controller.go`**: HTTP endpoints for `/register`, `/login`, aur `/profile`. Inputs validate karta hai aur JSON response return karta hai.
- **`health_controller.go`**: System health endpoints (`Check`, `Live`, `Ready`).
- **`health_controller_test.go`**: Unit tests jo `httptest.NewRecorder()` use karke health endpoints verify karte hain.

### 6. `internal/handler/routes/` (Router Wiring)
- **`router.go`**: Chi HTTP router ka central wiring hub. Saare middlewares pipeline, Swagger UI, aur `/api/v1` routes ko mount karta hai.

---

## 🛡️ `internal/middleware/` (Production Middlewares)

Ye middlewares har request aane par execute hote hain:

| File | Purpose |
|---|---|
| `request_id.go` | Har incoming request ko ek unique `X-Request-ID` (UUID) assign karta hai taaki distributed logs trace ho sakein. |
| `recoverer.go` | **Crash Protection:** Runtime panics / nil pointer dereferences ko catch karta hai, stack trace log karta hai, aur client ko safe 500 error deta hai (server band nahi hone deta). |
| `security.go` | OWASP security headers inject karta hai (`X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `X-XSS-Protection`). |
| `logger.go` | Uber Zap structured access logger jo method, path, HTTP status, aur execution latency log karta hai. |
| `cors.go` | Cross-Origin Resource Sharing (CORS) headers aur pre-flight `OPTIONS` requests handle karta hai. |
| `ratelimit.go` | In-memory token bucket rate limiter (e.g. 100 requests per minute per IP) jo DDoS aur brute-force attacks se bachata hai. |
| `auth.go` | JWT Bearer token verify karta hai aur `UserID`, `Email`, aur `Role` claims request `context` me daalta hai. |
| `role.go` | Role-based access control guard (e.g. Admin-only endpoints). |

---

## 🗄️ `internal/migration/` (Database Migrations)

| File | Purpose |
|---|---|
| `migration.go` | Auto-migrations engine jo server start hote hi schema consistency check karta hai. |
| `000001_create_users_table.up.sql` | `users` table create karne ka SQL script. |
| `000001_create_users_table.down.sql` | `users` table drop karne ka rollback SQL script. |

---

## 🧰 `internal/reuse/` (Reusable Utilities)

Har naye feature me kaam aane wale general-purpose helpers:

| File | Purpose |
|---|---|
| `pagination.go` | `?page=1&limit=10&order=desc&search=xyz` parse karta hai aur standard `PaginatedResponse` return karta hai. |
| `cache.go` | Concurrency-safe in-memory key-value cache with TTL (Time-to-Live) aur background auto-cleanup. |
| `errors.go` | Standardized JSON response envelopes (`reuse.Success`, `reuse.Created`, `reuse.Error`, `reuse.Message`). |
| `validateInput.go` | `go-playground/validator/v10` wrapper jo struct validation errors ko human-readable format me convert karta hai. |
| `generate_jwt.go` | JWT access tokens sign aur parse karta hai. |
| `hashPassword.go` | Secure Bcrypt password hashing (Cost 12) aur password match verification. |
| `claudeflare.go` | AWS S3 aur Cloudflare R2 file uploads aur presigned URLs ka helper. |
| `otp.go` | Cryptographically secure 6-digit OTP generator aur SHA-256 hash validator. |
| `slug.go` | String slugifier (e.g. "My First Post" ➡️ "my-first-post"). |

---

## ⚙️ `internal/utils/` & `internal/servers/` & `internal/worker/`

| Path | Purpose |
|---|---|
| `internal/utils/config.go` | `.env` variables load karta hai with fallback defaults aur required validation checks. |
| `internal/utils/database.go` | Supabase / PgBouncer friendly PostgreSQL connection pool (`pgxpool.Pool`) initialize karta hai. |
| `internal/utils/tx.go` | **Transaction Helper:** `utils.WithTx(ctx, db, func(tx) error { ... })` jo automatically error aane par rollback aur success par commit karta hai. |
| `internal/servers/http.go` | Production HTTP server configuration with Read/Write timeouts aur Graceful Shutdown signals. |
| `internal/worker/worker.go` | Background asynchronous tasks aur cron-like workers ke liye safe cancelable context template. |

---

## ⚡ `scripts/` (Automation & Scaffolding)

| File | Purpose |
|---|---|
| `scripts/generate-domain.bat` | Windows command-line shortcut: `.\scripts\generate-domain.bat <Name>`. |
| `scripts/generate-domain.ps1` | Automation engine jo 2 second me complete Model, DTO, Repo, Service, Controller, aur Migration generate karta hai. |
| `scripts/templates/*.tpl` | Clean scaffolding templates jinhe `generate-domain` use karta hai. |
