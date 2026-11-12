# Xazavier Backend API (Go)

This Go service provides the RESTful API for the **Xazavier Personal Activity Ledger**. It enforces historical integrity rules, session-based authentication with Argon2id and JWT, cluster and activity management, daily activity logs with strict tamper-resistant constraints, and complete audit trail tracking.

---

## ⚡ Quick Start

### Option 1: In-Memory Mode (Zero Setup / Instant Run)
If you don't have PostgreSQL running or want to quickly test with the frontend:
```bash
# In .env: set DATABASE_URL=memory
# Or run directly:
DATABASE_URL=memory go run ./cmd/Server
```
*Note: In-memory mode automatically pre-seeds demo user (`demo@xazavier.app` / `demo1234`), 3 clusters, 7 activities, and 7 days of realistic logs matching the UI requirements.*

### Option 2: PostgreSQL Mode
```bash
cp .env.example .env
# Edit .env with your PostgreSQL credentials:
# DATABASE_URL=postgres://postgres:postgres@localhost:5432/xazavier?sslmode=disable
# AUTO_MIGRATE=true

go run ./cmd/Server
```

To seed demo data in PostgreSQL:
```bash
go run ./cmd/seed
```

### Option 3: Docker Compose
```bash
docker compose up --build
```
API runs on `:8080` and PostgreSQL on `:5432`.

---

## 🧪 Testing

Run all unit and API router tests:
```bash
go test -v ./...
```
All unit tests in `internal/service` and `internal/httpapi` run in memory without requiring an external database.

---

## 🗄️ Database Migrations

Layanan ini mengelola skema tabel dan fungsi trigger PostgreSQL secara berurutan menggunakan file SQL di dalam direktori `migrations/`.

### 0. Persiapan: Pembuatan Database `xazavier` di PostgreSQL / pgAdmin
Sebelum tabel dapat dimigrasikan, database target `xazavier` harus ada di server PostgreSQL lokal Anda.

**Cara Membuat Database `xazavier`:**
- **Via pgAdmin 4:**
  1. Buka pgAdmin dan login ke server PostgreSQL Anda.
  2. Klik kanan menu **Databases** -> pilih **Create** -> **Database...**
  3. Masukkan nama Database: `xazavier`
  4. Klik **Save**.
- **Via SQL Query Tool di pgAdmin / psql:**
  ```sql
  CREATE DATABASE xazavier;
  ```
- **Via Auto-Create Go (Otomatis saat Server / Seed dijalankan):**
  Layanan backend Go telah dilengkapi fungsi `EnsureDatabaseExists()`: jika kredensial di `.env` memiliki hak akses `CREATEDB`, server Go akan otomatis membuatkan database `xazavier` jika belum ditemukan saat startup.

> ⚠️ **PENTING: Cek Password PostgreSQL di `.env`**
> Pastikan username dan password pada file `.env` sudah sesuai dengan password PostgreSQL / pgAdmin lokal Anda:
> ```env
> DATABASE_URL=postgres://<USER>:<PASSWORD>@localhost:5432/xazavier?sslmode=disable
> ```
> *(Ganti `<USER>` dan `<PASSWORD>` dengan user & password pgAdmin Anda, default postgres biasanya adalah user `postgres`)*

### 1. Migrasi Otomatis (Auto-Migrate)
Secara *default*, server akan mendeteksi dan menjalankan migrasi secara otomatis setiap kali server *startup*. Pastikan variabel berikut aktif di `.env`:
```env
AUTO_MIGRATE=true
DATABASE_URL=postgres://postgres:postgres@localhost:5432/xazavier?sslmode=disable
```
> **Catatan:** Seluruh file SQL migrasi di-*embed* langsung ke dalam binary Go melalui `embed.FS` (`migrations.FS`). Aplikasi dapat melakukan migrasi tanpa perlu menyalin folder `migrations/` secara terpisah di container Docker atau server produksi.

### 2. Eksekusi Migrasi Manual

#### Opsi A: Menggunakan CLI `psql`
Jika ingin mengeksekusi langsung ke PostgreSQL:
```bash
# Eksekusi per file migrasi secara berurutan:
psql -U postgres -d xazavier -f migrations/0001_init.up.sql
psql -U postgres -d xazavier -f migrations/0002_delete_guard.up.sql
psql -U postgres -d xazavier -f migrations/0003_fix_audit_fkey.up.sql

# Atau eksekusi file skema master lengkap:
psql -U postgres -d xazavier -f schema.sql
```

#### Opsi B: Menggunakan tool `golang-migrate`
```bash
# Instalasi golang-migrate CLI (jika belum terpasang):
# go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Jalankan migrasi:
migrate -path migrations -database "postgres://postgres:postgres@localhost:5432/xazavier?sslmode=disable" up
```

### 3. Daftar Berkas Migrasi

| File | Deskripsi |
|---|---|
| [`0001_init.up.sql`](file:///migrations/0001_init.up.sql) | Membuat tabel dasar (`users`, `sessions`, `clusters`, `activities`, `logs`, `audit`) serta trigger fungsi integritas `enforce_log_integrity()`. |
| [`0002_delete_guard.up.sql`](file:///migrations/0002_delete_guard.up.sql) | Menambahkan fungsi dan trigger `guard_delete_log()` untuk menolak penghapusan data log yang sudah melewati batas waktu 24 jam. |
| [`0003_fix_audit_fkey.up.sql`](file:///migrations/0003_fix_audit_fkey.up.sql) | Menghapus cascade foreign key pada `audit.log_id` agar data audit record bertipe `DELETE` tidak ikut terhapus saat log di-*delete*. |

### 4. Menambahkan Migrasi Baru
Untuk menambahkan perubahan skema database di masa mendatang:
1. Buat file baru di folder `migrations/` dengan format penomoran berurutan:
   ```
   migrations/0004_nama_fitur_baru.up.sql
   ```
2. Karena direktori `migrations` menggunakan Go `//go:embed *.sql`, file SQL baru akan otomatis terdeteksi saat `go build` dan otomatis dieksekusi saat server dijalankan dengan `AUTO_MIGRATE=true`.

---

## 🔒 Integrity Rules Enforced by Server

1. **Server-Assigned Timestamps & Status**: `created_at`, `updated_at`, dan `entry_status` selalu dihitung dan ditentukan secara otoritatif di sisi server.
2. **Future Activity Rejection**: Aktivitas yang waktu mulai/selesainya > 5 menit ke masa depan akan ditolak (`FUTURE_LOG`).
3. **24-Hour Backdate Barrier**: Aktivitas yang dimulai lebih dari 24 jam yang lalu tidak dapat ditambahkan (`LATE_WINDOW`).
4. **24-Hour Edit & Delete Lock**: Log aktivitas yang umurnya sudah melewati 24 jam dari waktu mulai terkunci permanen (*read-only*), tidak dapat diedit maupun dihapus.
5. **Klasifikasi Normal vs. Late**:
   - `NORMAL`: Dicatat dalam rentang 3 jam sejak waktu mulai pada hari lokal yang sama.
   - `LATE`: Dicatat setelah > 3 jam atau pada tanggal lokal yang berbeda.
6. **Immutable Audit Trail**:
   - Setiap perubahan waktu/nilai mencatat audit `UPDATE` yang menyimpan nilai lama.
   - Penghapusan log mencatat audit `DELETE` yang tetap tersimpan permanen untuk ditampilkan di daftar *Deleted records*.

---

## 📡 API Endpoints

### Health & Readiness
| Method | Path | Description |
|---|---|---|
| `GET` | `/healthz` | Health check (`{"status":"ok"}`) |
| `GET` | `/readyz` | Readiness check (`{"status":"ready"}`) |

### Authentication
| Method | Path | Description |
|---|---|---|
| `POST` | `/api/auth/register` | Register new user (email, password, full_name, timezone) |
| `POST` | `/api/auth/login` | Sign in (email, password) |
| `POST` | `/api/auth/refresh` | Exchange refresh token for new access token |
| `POST` | `/api/auth/logout` | Revoke session and logout |
| `GET` | `/api/me` | Current authenticated user profile |

### Clusters
| Method | Path | Description |
|---|---|---|
| `GET` | `/api/clusters` | List clusters for current user |
| `POST` | `/api/clusters` | Create cluster (`name`, `icon`, `color`) |
| `PUT` | `/api/clusters/{id}` | Update cluster (rename or `{archived: true/false}`) |
| `DELETE` | `/api/clusters/{id}` | Soft delete cluster |

### Activities
| Method | Path | Description |
|---|---|---|
| `GET` | `/api/activities` | List activities for current user |
| `POST` | `/api/activities` | Create activity (`cluster_id`, `name`, `tracking_type`, `unit_name`) |
| `PUT` | `/api/activities/{id}` | Update activity (rename, change unit, or `{archived: true/false}`) |
| `DELETE` | `/api/activities/{id}` | Soft delete activity |

### Daily Activity Logs
| Method | Path | Description |
|---|---|---|
| `GET` | `/api/logs` | List logs for user (returns `status`: `"normal"` or `"late"`, `edited` count) |
| `GET` | `/api/logs/{id}` | Get single log by ID |
| `POST` | `/api/logs` | Record new activity log |
| `PUT` | `/api/logs/{id}` | Edit log (`end_time`, `logged_value`, `notes`) |
| `DELETE` | `/api/logs/{id}` | Delete log (creates DELETE audit record) |

### Audit & Admin
| Method | Path | Description |
|---|---|---|
| `GET` | `/api/logs/{id}/audit` | Get audit change history for a specific log |
| `GET` | `/api/audit` | List all user audit events (including deleted records) |
| `POST` | `/api/reset` | Reset all user transactional data for demo testing |

---

## 💻 Connecting React Frontend

In the root `.env` or React app configuration:
```env
VITE_API_URL=http://localhost:8080
```
Run the frontend:
```bash
npm run dev
```
The React frontend in `src/services/api.js` connects directly to the Go API with Bearer token authentication, automatic token refresh, and error toast feedback.
