# CI/CD backend: GitHub Actions, GHCR, dan Render

```text
PR → format / vet / OpenAPI → unit test + race → PostgreSQL integration → build validation
Push main → CI → build Docker sekali → push GHCR → scan digest → release.json
Deploy aktif → DEV otomatis → SIT → UAT → STG → PRD (digest yang sama)
```

Target hosting adalah **Render Web Service dari Existing Image**. PostgreSQL
menyimpan data bisnis; Supabase tetap menyediakan Auth dan Storage. Image release
adalah `ghcr.io/dikasa-company/be-dikasa@sha256:...`, arsitektur `linux/amd64`,
berjalan sebagai user non-root. Runtime membaca port dari `PORT`.

## Mulai dari DEV

Tidak perlu menyiapkan kelima environment sekaligus. Mulai dengan satu service
DEV. Promosi ke SIT/UAT/STG/PRD baru dilakukan setelah environment tujuan siap.
Paket Free dapat dipakai untuk mencoba DEV; service tidur setelah 15 menit tanpa
traffic dan bangun kembali sekitar satu menit. Kuota 750 jam per workspace per
bulan dibagi antarlayanan. Paket ini tidak disarankan Render untuk produksi.
Free Render Postgres kedaluwarsa setelah 30 hari; pilih database yang sesuai
kebutuhan dan backup. Lihat [batas paket Free](https://render.com/docs/free).

### 1. Biarkan deploy nonaktif sampai konfigurasi selesai

Biarkan repository variable `RENDER_DEPLOY_ENABLED` kosong atau `false`. PR
menjalankan CI; push main tetap build, upload, scan, dan mencatat image GHCR
menggunakan `GITHUB_TOKEN`. Job DEV dilewati dan tidak membutuhkan akun atau API
key Render. Organisasi harus mengizinkan Actions membuat package GHCR.
HIGH/CRITICAL yang memiliki perbaikan atau kegagalan scanner memblokir manifest.

Setelah merge, ambil digest dari summary **Backend release** untuk membuat
service Render awal. Jangan memakai Git-backed service atau build ulang di
Render, karena pipeline mempromosikan image yang sudah diuji.

### 2. Buat service Render

Di [dashboard Render](https://dashboard.render.com), pilih **New → Web Service →
Existing Image**, lalu masukkan image GHCR dengan digest yang telah ditinjau.
Beri nama misalnya `dikasa-api-dev`, pilih region dekat database, dan pilih ukuran
instance/plan sendiri. Workflow tidak membuat resource atau mengubah paket.

Jika package GHCR private, tambahkan registry credential pada Render: registry
GitHub, username GitHub, dan token dengan `read:packages` yang dapat membaca
package organisasi, termasuk otorisasi SSO jika diwajibkan. Buat token dari
GitHub Settings → Developer settings → Personal access tokens → Tokens (classic).
Simpan token langsung di Render; jangan memakai `GITHUB_TOKEN` job sebagai
credential pull jangka panjang karena token job akan kedaluwarsa. Pipeline tidak
mengubah visibility package menjadi public.

Konfigurasi runtime berikut di tab **Environment** service Render. Nilai secret
dimasukkan langsung ke Render; jangan dimasukkan ke Git atau chat.

| Runtime variable | Nilai yang dibutuhkan |
| --- | --- |
| `PORT` | `8080` (atau port yang disediakan Render) |
| `DATABASE_URL` | DSN PostgreSQL tujuan, user aplikasi, TLS; bukan localhost runner |
| `SUPABASE_URL` | URL project Supabase tujuan |
| `SUPABASE_PUBLISHABLE_KEY` | Publishable key tujuan |
| `SUPABASE_SECRET_KEY` | Server-only secret key tujuan |
| `SUPABASE_STORAGE_BUCKET` | Public bucket gambar tujuan |
| `OTP_SECRET` | Secret acak minimal 32 karakter |
| `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_FROM` | Akun SMTP tujuan |
| `CORS_ALLOWED_ORIGINS` | Daftar origin frontend yang diizinkan |

Atur **Health Check Path** ke `/dikasa/v1/openapi.json`. Endpoint
`/dikasa/v1/health` tetap memerlukan Supabase Bearer token sehingga tidak dapat
dipakai sebagai probe HTTP anonim. Kosongkan **Docker Command** dan
**Pre-Deploy Command**: image menjalankan `/app/api` dan CI menangani migrasi.
Pre-deploy command Render hanya tersedia pada tipe/paket layanan tertentu;
pipeline tidak bergantung pada fitur tersebut sehingga jalur DEV Free didukung.

Pastikan initial deploy berstatus **Live**. API memerlukan PostgreSQL yang dapat
dihubungi dan bucket Supabase yang valid saat startup. Sebelum membuat akun smoke,
siapkan skema sesuai langkah migrasi di bawah. Jangan mengubah service/deployment
secara manual ketika pipeline berjalan.

### 3. Ambil Service ID, URL, dan API key

- **Service ID** berbentuk `srv-...`, dapat dilihat pada URL halaman service Render.
- **Service URL** adalah HTTPS `https://nama-service.onrender.com` yang ditampilkan
  di halaman service, atau custom domain HTTPS milik service tersebut.
- **API key** dibuat dari Render **Account Settings → API Keys → Create API Key**.
  Gunakan identitas dengan akses yang diperlukan untuk service tujuan. Scoped API
  keys masih early access; pisahkan akses PRD bila pembatasan yang dibutuhkan
  belum tersedia. Lihat [pengaturan API key](https://render.com/docs/api).

### 4. Simpan konfigurasi GitHub

Buat GitHub Environment `dev` melalui repo **Settings → Environments**. Tambahkan:

| Jenis | Nama | Nilai |
| --- | --- | --- |
| Environment variable | `RENDER_SERVICE_ID` | ID `srv-...` service DEV |
| Environment variable | `RENDER_SERVICE_URL` | URL HTTPS service DEV tanpa path/query/fragment |
| Environment secret | `RENDER_API_KEY` | API key Render yang dapat membaca service/deploy dan memicu/membatalkan deploy |
| Environment secret | `MIGRATION_DATABASE_URL` | DSN PostgreSQL DEV, user migrator/DDL, TLS |
| Environment secret | `SMOKE_USERNAME` | Identifier akun pengujian DIKASA yang aktif |
| Environment secret | `SMOKE_PASSWORD` | Password akun pengujian DIKASA tersebut |

Akun smoke adalah akun aplikasi DIKASA di PostgreSQL/Supabase, **bukan** akun login
Render. Gunakan akun khusus dengan hak minimum, bukan identitas bisnis nyata.
User PostgreSQL API dan migrator sebaiknya berbeda. Runner harus dapat mengakses
alamat database pada `MIGRATION_DATABASE_URL`.

Untuk aktivasi, buat **repository variable** `RENDER_DEPLOY_ENABLED=true` melalui
Settings → Secrets and variables → Actions → Variables. Buat release baru dari
main agar memperoleh bukti DEV sukses. Pemicu manual promosi menolak deployment
ketika flag belum aktif. Flag Koyeb lama tidak digunakan lagi.

### 5. Aturan promosi

Sebelum membuat SIT/UAT/STG/PRD, siapkan service, database, Supabase project/bucket,
SMTP, akun smoke, dan GitHub Environment masing-masing dengan konfigurasi yang
sama nama variabelnya tetapi nilai berbeda. Batasi deployment ke branch `main`.
Lindungi main dengan PR, review, dan keempat required checks Backend CI.

Pasang reviewer bisnis/QA pada UAT, operasional pada STG, dan release owner pada
PRD; cegah self-review jika tersedia. Dukungan required reviewers bergantung
paket/visibilitas repo. Jika tidak tersedia, sediakan approval eksternal sebelum
mengaktifkan produksi. Tombol manual sendiri tidak memberi persetujuan terpisah.

## File pipeline

| File | Fungsi |
| --- | --- |
| `.github/workflows/be-ci.yml` | PR/reusable CI tanpa secret deployment |
| `.github/workflows/be-release.yml` | Push main, CI, GHCR, scan, manifest, DEV opsional |
| `.github/workflows/be-promote.yml` | Promosi/rollback manual dari main |
| `.github/workflows/reusable-deploy.yml` | Gate, environment approval, Render, catat sukses |
| `scripts/cicd/render.py` | Migrasi dari image, Render deploy, polling, smoke, recovery |
| `scripts/cicd/release.py` | Verifikasi sumber release, digest dan bukti environment sebelumnya |
| `deploy/migrate.sh` | Migrasi sekali saja, checksum, transaksi dan advisory lock |

Pemeriksaan gofmt berlaku pada file Go yang berubah terhadap base PR/commit
sebelumnya. Integration memakai PostgreSQL 18 sementara di runner. Endpoint,
OpenAPI, Bruno, tabel bisnis, dan relasi ERD tidak berubah oleh pergantian provider.

## Migrasi dan baseline

Sebelum memicu Render deploy, runner menarik digest yang akan dipromosikan lalu
menjalankan `/bin/sh /app/deploy/migrate.sh` pada container terpisah. Hanya
`DATABASE_URL` dari secret `MIGRATION_DATABASE_URL` diteruskan ke container.
Database harus dapat dijangkau runner lewat TLS. Jika hanya tersedia pada jaringan
privat, gunakan runner yang memiliki akses tersebut; jangan membuka akses tanpa
batas. Migrasi tidak bergantung pada shell/one-off jobs Render Free.

Ledger `public.dikasa_schema_migrations(name, sha256, applied_at)` dan seluruh SQL
baru ditulis pada satu transaksi dengan advisory lock. Replay menjadi no-op,
checksum berubah ditolak, dan kegagalan SQL membatalkan batch. Migrasi di luar
transaksi belum didukung. Pertahankan line ending LF untuk checksum stabil.

Database kosong dapat langsung dimigrasikan. Database lama yang sudah berisi
`users` tetapi belum memiliki ledger ditolak. Setelah backup dan verifikasi skema,
operator mencatat baseline sekali dengan image yang ditinjau. Contoh `000017`
hanya benar bila seluruh migrasi hingga versi tersebut sudah diterapkan:

```bash
# Ekspor DATABASE_URL dari secret manager ke terminal operator; jangan simpan di Git.
docker run --rm --env DATABASE_URL --entrypoint /bin/sh \
  ghcr.io/dikasa-company/be-dikasa@sha256:REPLACE_WITH_REVIEWED_DIGEST \
  /app/deploy/migrate.sh baseline 000017
```

Untuk database kosong, gunakan command yang sama tanpa `baseline 000017`.
Baseline hanya mencatat checksum; tidak menebak keadaan skema. Jangan menjalankan
`make migrate-up` berulang pada database berisi data.

## Deploy, smoke dan recovery

Pilih **Backend promote or rollback**, branch main, release run ID dari summary
Backend release, dan tujuan SIT/UAT/STG/PRD. Sumber harus push main melalui
be-release.yml yang sukses; manifest harus cocok dengan commit/run/GHCR dan
digest yang sama harus pernah sukses di environment sebelumnya. Catatan GitHub
Deployments task `dikasa-release` dipaginasi. Lock environment mencegah dua
workflow deployment berjalan bersamaan.

Script memverifikasi Render service bertipe image-backed Web Service, tidak
suspended, dan image repository cocok. Existing deploy harus Live dan tidak ada
deploy pending. Script hanya memanggil `POST /v1/services/{id}/deploys` dengan
`imageUrl` ber-digest; region, plan, runtime secret, registry credential, dan
pengaturan service tidak diubah. Image-backed service tidak auto-redeploy saat
registry berubah, sehingga gunakan pipeline untuk setiap promosi.

Script menunggu ID deploy baru berstatus `live` dan memeriksa **resolved image
SHA** yang dilaporkan Render. Status Live dari deploy lama tidak cukup. Smoke
memeriksa OpenAPI, penolakan health tanpa token dan dengan token invalid, login,
serta health dengan token sah. Traffic dapat melayani image baru sebelum smoke
selesai; ini bukan canary bertahap.

Jika deploy gagal, timeout atau smoke gagal dan ID update diketahui, script
membatalkan deploy yang masih berjalan lalu meminta deploy baru dari **digest
sebelumnya** dan menunggu Live. Workflow tetap gagal dan tidak memberi bukti
promosi sukses. Recovery menolak menimpa deploy lain yang lebih baru. Respons
create-deploy yang hilang mempunyai hasil tidak pasti: periksa Render sebelum
retry. Recovery yang gagal memerlukan operator. Cancel paksa workflow dapat
menghentikan cleanup; periksa state Render sebelum menjalankan ulang.

Rollback manual hanya menerima release yang pernah sukses di environment tujuan.
Tidak ada rebuild atau down migration; migrasi dilewati. Gunakan expand-contract
dan review kompatibilitas skema sebelum approval. Recovery/rollback aplikasi
tidak membatalkan SQL/data yang sudah berhasil dimigrasikan. Tetap kosongkan
Pre-Deploy Command agar Render tidak menjalankan migrasi tambahan saat rollback.

Manifest disimpan 90 hari. Pertahankan run, artifact, deployment records, dan
digest GHCR selama jendela rollback. Artifact kedaluwarsa/image terhapus akan
memblokir promosi atau rollback. Rerun build yang telah mengunggah manifest ditolak
oleh artifact immutable; kandidat baru menggunakan release baru.

## Verifikasi

```bash
python3 -m unittest discover -s scripts/cicd -p 'test_*.py'
go generate ./internal/server
go test ./...
go vet ./...
go build ./...
go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
shellcheck deploy/*.sh
```

Test Render memakai API/migrasi/smoke palsu dan tidak membuat resource cloud.
`bash deploy/test-migrate.sh` memerlukan database CI kosong khusus melalui
`DATABASE_URL`; `DIKASA_TEST_DATABASE_URL` mengaktifkan integration test Go.

Referensi: [Existing Docker image](https://render.com/docs/deploying-an-image),
[API deploy](https://api-docs.render.com/reference/create-deploy),
[Render API keys](https://render.com/docs/api),
[Free plan](https://render.com/docs/free),
[GitHub Environments](https://docs.github.com/en/actions/reference/workflows-and-actions/deployments-and-environments).
