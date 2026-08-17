# Artefak API Dikasa

- `openapi.yaml` adalah kontrak OpenAPI yang dihasilkan dari operasi Huma. Jangan
  mengubahnya secara manual; jalankan `make openapi` atau
  `go generate ./internal/server`.
- `bruno/` berisi collection, environment contoh, dan request untuk pengujian
  manual API.
- Swagger UI tidak membutuhkan folder statis. Huma menyajikannya saat aplikasi
  berjalan di `/dikasa/v1/docs` menggunakan kontrak yang sama.

Semua endpoint aplikasi menggunakan prefix `/dikasa/v1`. Login, register,
konfirmasi/resend OTP, dan dokumentasi bersifat publik; endpoint lain harus
mengirim `Authorization: Bearer <Firebase ID token>`.
