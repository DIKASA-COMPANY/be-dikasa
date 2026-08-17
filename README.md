# be-dikasa

Backend Dikasa memakai feature-oriented architecture dengan Gin, Huma, GORM,
PostgreSQL, go-playground/validator, dan Firebase Authentication.

PostgreSQL adalah sumber data user serta password hash. Firebase tidak menyimpan
data bisnis atau password Dikasa; Firebase Admin hanya menerbitkan custom token,
menukarnya menjadi ID/refresh token, dan memverifikasi ID token pada request API.
UID Firebase selalu sama dengan UUID `users.id` di PostgreSQL.

## Struktur

```text
be-dikasa/
├── cmd/
│   ├── api/main.go             # Composition root dan HTTP server
│   └── openapi/main.go         # Generator OpenAPI
├── internal/
│   ├── auth/                   # Login dan penerbitan Firebase token
│   ├── registration/           # Register, konfirmasi OTP, resend, dan SMTP
│   ├── user/                   # Entity, repository, service, handler, route
│   ├── config/                 # Environment configuration
│   ├── database/               # PostgreSQL dan Firebase clients
│   ├── server/                 # Gin/Huma router dan auth middleware
│   └── shared/                 # Error, response, dan validator lintas feature
├── migrations/                 # PostgreSQL up/down migrations
├── api/
│   ├── openapi.yaml            # Generated contract
│   └── bruno/                  # Manual API collection
└── docs/
    ├── architecture/DIKASA.drawio
    └── development/naming.md
```

## Konfigurasi

Salin `.env.example` menjadi `.env` dan masukkan nilainya ke environment
terminal. Aplikasi tidak membaca `.env` secara otomatis.

- `DATABASE_URL`: DSN PostgreSQL.
- `FIREBASE_PROJECT_ID`: Firebase project ID.
- `FIREBASE_WEB_API_KEY`: Web API key untuk menukar custom token.
- `GOOGLE_APPLICATION_CREDENTIALS`: path absolut service-account JSON Firebase
  Admin. Jangan commit atau kirim file ini melalui chat.
- `OTP_SECRET`: secret acak minimal 32 karakter untuk HMAC OTP; jangan commit.
- `SMTP_HOST`, `SMTP_PORT`, dan `SMTP_FROM`: koneksi pengiriman email OTP.
- `SMTP_USERNAME` dan `SMTP_PASSWORD`: credential SMTP bila server mewajibkan
  autentikasi.
- `PORT`: opsional, default `8080`.

Yang dibutuhkan dari Firebase adalah project ID, Web API key, dan service account
yang tersedia secara lokal melalui Application Default Credentials. Tidak perlu
memberikan credential Firebase kepada Codex atau menyimpannya di repository.

## Database dan menjalankan API

Jalankan migration identity awal, lalu server:

```powershell
$env:DATABASE_URL = "postgres://dikasa:password@localhost:5432/dikasa?sslmode=disable"
make migrate-up
make run
```

Migration membuat `roles`, `genders`, `outlets`, `users`, dan
`registration_otps`, termasuk foreign key dan index case-insensitive. Password
disimpan sebagai bcrypt hash; OTP disimpan sebagai HMAC-SHA256, bukan plaintext.

## API dan dokumentasi

- Login: `POST /dikasa/v1/login`
- Register + kirim OTP: `POST /dikasa/v1/register`
- Konfirmasi OTP: `POST /dikasa/v1/register/otp/confirm`
- Kirim ulang OTP: `POST /dikasa/v1/register/otp/resend`
- Profil: `GET /dikasa/v1/users/me`
- Health: `GET /dikasa/v1/health`
- Swagger UI: `http://localhost:8080/dikasa/v1/docs`
- OpenAPI: `api/openapi.yaml` atau runtime `/dikasa/v1/openapi.yaml`
- Bruno collection: `api/bruno`

```json
{
  "identifier": "kasir",
  "password": "rahasia123"
}
```

`identifier` menerima username, nomor HP, atau email PostgreSQL. Respons login
berisi Firebase `access_token` dan `refresh_token`; kirim access token pada semua
endpoint terproteksi:

```http
Authorization: Bearer <access_token>
```

Tiga endpoint registrasi bersifat publik. Register membuat user dan outlet dalam
status nonaktif, mengirim OTP 5 digit yang berlaku 5 menit, dan mengizinkan
resend setelah 60 detik. Konfirmasi yang berhasil mengaktifkan keduanya secara
atomik. Maksimal 5 percobaan berlaku untuk setiap kode; resend merotasi kode dan
mereset jumlah percobaan.

Regenerasi kontrak setelah route/DTO berubah:

```powershell
make openapi
```

Konvensi folder, file, JSON, database, dan path API dijelaskan di
`docs/development/naming.md`.

## Pengujian

Pengembangan mengikuti TDD: test gagal dibuat terlebih dahulu, baru implementasi.

```powershell
make test
make vet
```
