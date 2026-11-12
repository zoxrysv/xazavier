# Xazavier

Personal activity ledger: record, verify, observe, reflect.

## Jalankan
```
npm install
npm run dev        # development
npm run build      # hasil di dist/
```

## Deploy
Upload **isi folder `dist/`** ke hosting statis (cPanel public_html, Netlify, Vercel, Nginx, dll). Build memakai `base: './'`, jadi bisa ditaruh di subfolder. Aplikasi ini tidak memakai router, jadi tidak perlu rewrite rule.
Halaman harus dibuka lewat **HTTPS** (atau localhost): mock API memakai `crypto.subtle` dan `crypto.randomUUID`, yang hanya tersedia di secure context.

## Struktur
- `src/api/mockServer.js` — server tiruan (in-memory + localStorage) yang meniru skema PostgreSQL dan trigger integritasnya. **Ini yang diganti saat backend asli siap.**
- `src/services/api.js` — satu-satunya pintu UI ke API (auth, token, refresh, konversi bentuk data).
- `src/hooks`, `src/components`, `src/pages`, `src/layouts`, `src/utils`, `src/types`, `src/styles`.

## Menyambung ke backend asli
1. Di `src/services/api.js`, ganti pemanggilan `request(method, path, body)` dengan `fetch` ke `VITE_API_URL + path` (header `Authorization: Bearer <access_token>`), lalu hapus import `mockServer`.
2. Endpoint yang dipakai: `POST /api/auth/{login,register,refresh,logout}`, `GET /api/me`, `/api/clusters`, `/api/activities`, `/api/logs`, `GET /api/logs/:id/audit`, dan `GET /api/audit` (semua baris audit milik user, untuk daftar "Deleted records").
3. Server wajib menentukan `created_at` dan `entry_status` sendiri (trigger di database sudah melakukannya). Client tidak pernah mengirim keduanya.
4. Arsip: client mengirim `{archived:true|false}`, server mengisi `deleted_at`. Untuk activity, client mengirim `unit_name`.
5. Di produksi: hash password dengan argon2/bcrypt, simpan refresh token di cookie httpOnly, dan jangan simpan token di localStorage. Autentikasi di mock hanya untuk demo, bukan pengaman.
6. Saran database: tambahkan guard `BEFORE DELETE` pada `daily_logs`, karena trigger saat ini tidak menolak penghapusan log yang sudah lewat 24 jam.
