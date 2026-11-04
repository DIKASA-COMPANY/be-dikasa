# CI/CD backend: GitHub Actions, GHCR, dan Koyeb

```text
PR → format / vet / OpenAPI → unit test + race → PostgreSQL integration → build validation
Push main → CI → build Docker sekali → push GHCR → scan digest → release.json
Deploy aktif → DEV otomatis → SIT → UAT → STG → PRD (digest yang sama)
```

Tidak memakai Firebase App Distribution atau Google Cloud. PostgreSQL tetap
menyimpan data bisnis; Supabase menyediakan Auth dan Storage. Image release
adalah `ghcr.io/dikasa-company/be-dikasa@sha256:...`, arsitektur `linux/amd64`,
berjalan sebagai user non-root pada port 8080. Jangan memakai Git autodeploy atau
build ulang di Koyeb: sumber service harus **pre-built Docker image**.

## CI dapat berjalan sebelum Koyeb siap

Biarkan repository variable `KOYEB_DEPLOY_ENABLED` kosong atau `false`. PR tetap
menjalankan seluruh CI. Push main tetap membangun, mengunggah, memindai, dan
mencatat image di GHCR dengan `GITHUB_TOKEN`; job DEV dilewati. Tidak dibutuhkan
token Koyeb untuk CI/publish image. Organisasi harus mengizinkan GitHub Actions
membuat package GHCR. HIGH/CRITICAL yang memiliki perbaikan atau kegagalan scanner
memblokir release manifest dan deployment.

Set `KOYEB_DEPLOY_ENABLED=true` hanya setelah konfigurasi DEV dan seluruh aturan
approval siap. Pemicu manual promosi menolak deployment ketika flag belum aktif.
Setelah aktivasi, buat release baru dari main agar digest tersebut memperoleh
catatan DEV sukses sebelum dipromosikan. Tombol manual tidak dapat melompati DEV.

## File pipeline

| File | Fungsi |
| --- | --- |
| `.github/workflows/be-ci.yml` | PR/reusable CI; tidak mengakses secret deployment |
| `.github/workflows/be-release.yml` | Push main, CI, image GHCR, scan, manifest, DEV opsional |
| `.github/workflows/be-promote.yml` | Promosi/rollback manual dari main |
| `.github/workflows/reusable-deploy.yml` | Gate, approval environment, Koyeb API, catat sukses |
| `scripts/cicd/koyeb.py` | Migrasi dari image, update service, tunggu deployment, smoke, recovery |
| `scripts/cicd/release.py` | Verifikasi sumber release, digest, dan bukti environment sebelumnya |
| `deploy/migrate.sh` | Migrasi sekali saja, checksum, transaksi, advisory lock |

Pemeriksaan gofmt hanya berlaku pada file Go yang berubah terhadap base PR/commit
sebelumnya; file lama tidak diformat massal. Integration memakai PostgreSQL 18
sementara di runner. Runtime, endpoint, OpenAPI, dan Bruno tidak diubah oleh CI/CD.

## Siapkan GitHub Environments

Buat `dev`, `sit`, `uat`, `stg`, `prd` sebelum mengaktifkan deploy. Batasi semua
deployment ke branch `main`. Lindungi main dengan PR, review, dan required checks
keempat job Backend CI. Pasang reviewer bisnis/QA pada UAT, operasional pada STG,
dan release owner pada PRD; cegah self-review bila tersedia. Dukungan required
reviewers bergantung paket/visibilitas repo. Jika fitur itu tidak tersedia,
sediakan gate approval eksternal sebelum mengaktifkan produksi.

| Lokasi | Nama | Nilai yang dibutuhkan |
| --- | --- | --- |
| Repository variable | `KOYEB_DEPLOY_ENABLED` | `false` saat setup, `true` setelah siap |
| Environment variable | `KOYEB_SERVICE_ID` | UUID service Koyeb tujuan, bukan nama app |
| Environment variable | `KOYEB_SERVICE_URL` | URL HTTPS service itu, tanpa query/fragment |
| Environment secret | `KOYEB_API_TOKEN` | API token organisasi/akun Koyeb yang dapat membaca dan mengubah service tujuan |
| Environment secret | `MIGRATION_DATABASE_URL` | DSN PostgreSQL environment tujuan, user migrator/DDL, TLS |
| Environment secret | `SMOKE_USERNAME` | Identifier akun smoke aktif di PostgreSQL/Supabase tujuan |
| Environment secret | `SMOKE_PASSWORD` | Password akun smoke tujuan |

Masukkan nilai rahasia langsung ke GitHub/Koyeb; jangan menaruhnya di PR, file
Git, atau chat. Gunakan identitas/organisasi terpisah untuk PRD jika pembatasan
token per service tidak tersedia. Jangan menggunakan token admin bersama untuk
seluruh environment bila batas akses yang lebih sempit tersedia.

## Siapkan service Koyeb

1. Buat satu service per environment dan catat app, UUID service, region, ukuran
   instance, serta URL-nya. Jumlah service/instance harus sesuai kapasitas paket
   Koyeb Anda. Tidak ada resource berbayar yang dibuat otomatis oleh workflow ini.
2. Pilih sumber **Docker**. Bootstrap menggunakan image yang telah disetujui dan
   **dikunci ke sha256 digest**, lalu pastikan deployment healthy. Pipeline sengaja
   membutuhkan service yang sudah sehat agar tersedia definisi untuk recovery.
   Jangan menyimpan unapplied changes atau mengubah console selama deployment.
3. Untuk GHCR private, buat Koyeb **private container registry secret** dengan
   registry `ghcr.io`, username GitHub, dan token read-only `read:packages` yang
   dapat mengakses package organisasi (termasuk SSO bila diwajibkan). Pasang secret
   tersebut pada sumber Docker service. `GITHUB_TOKEN` job tidak cocok untuk pull
   Koyeb jangka panjang karena berakhir setelah job; jangan menggunakannya sebagai
   registry credential Koyeb. Package boleh public hanya jika disetujui pemiliknya;
   pipeline tidak mengubah visibility package secara otomatis.
4. Konfigurasi port `8080:http` dan route `/` ke port 8080, `PORT=8080`, serta
   HTTP health check `/dikasa/v1/openapi.json` atau TCP health check port 8080.
   `/dikasa/v1/health` tetap membutuhkan Supabase Bearer token sehingga tidak
   cocok sebagai health probe anonim.
5. Pasang konfigurasi runtime berikut pada service Koyeb. Nilai sensitif memakai
   referensi **Koyeb Secrets**. Secret bersifat global dalam organisasi Koyeb;
   gunakan nama berawalan environment dan grant akses hanya ke service yang perlu.
   Database, Supabase project/bucket, SMTP, dan akun smoke dipisahkan per
   environment. Jangan memakai identitas bisnis nyata untuk pengujian otomatis.

| Runtime environment | Sumber |
| --- | --- |
| `DATABASE_URL` | Koyeb Secret: DSN user aplikasi (berbeda dari user migrator) |
| `SUPABASE_SECRET_KEY` | Koyeb Secret |
| `OTP_SECRET` | Koyeb Secret, minimal 32 karakter |
| `SMTP_PASSWORD` | Koyeb Secret |
| `SUPABASE_URL` | Konfigurasi project Supabase tujuan |
| `SUPABASE_PUBLISHABLE_KEY` | Publishable key tujuan |
| `SUPABASE_STORAGE_BUCKET` | Public bucket gambar tujuan |
| `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_FROM` | Konfigurasi SMTP tujuan |
| `CORS_ALLOWED_ORIGINS` | Daftar origin frontend yang diizinkan |
| `PORT` | `8080` |

Workflow mempertahankan definisi service yang ada (secret reference, registry
credential reference, region, instance, port, route, health check, scaling) dan
hanya mengganti `docker.image`. Pengaturan resource dibuat dan ditinjau di Koyeb.
Bootstrap service tidak boleh mengoverride entrypoint/command image sehingga API
yang dijalankan tetap `/app/api`.

## Migrasi

Sebelum mengubah service, runner GitHub menarik **digest yang akan dideploy** dan
menjalankan `/bin/sh /app/deploy/migrate.sh` di container terpisah. Hanya
`DATABASE_URL` dari secret `MIGRATION_DATABASE_URL` yang diteruskan ke container.
Runner harus dapat menjangkau PostgreSQL lewat TLS. Jika database hanya dapat
diakses dari jaringan privat, gunakan runner dengan akses jaringan itu sebelum
mengaktifkan deploy; jangan membuka akses database tanpa batas sebagai solusi.

Runner menyimpan `public.dikasa_schema_migrations(name, sha256, applied_at)`.
Satu transaksi dan advisory lock melindungi seluruh batch; replay menjadi no-op,
checksum yang berubah ditolak, dan kegagalan SQL membatalkan batch. Migrasi di
luar transaksi belum didukung. SQL menggunakan LF agar checksum stabil.

Database lama yang sudah berisi `users` tetapi belum memiliki ledger ditolak.
Setelah verifikasi skema aktual dan backup, operator melakukan baseline sekali
dari image yang ditinjau (contoh `000017` hanya bila seluruh migrasi sampai versi
tersebut telah diterapkan):

```bash
# DATABASE_URL diekspor dari secret manager ke terminal operator, jangan disimpan di Git.
docker run --rm --env DATABASE_URL --entrypoint /bin/sh \
  ghcr.io/dikasa-company/be-dikasa@sha256:REPLACE_WITH_REVIEWED_DIGEST \
  /app/deploy/migrate.sh baseline 000017
```

Baseline hanya mencatat checksum, tidak menebak atau mengubah skema aplikasi.
Jangan menjalankan `make migrate-up` berulang pada database berisi data.

## Promosi dan recovery

Pilih **Backend promote or rollback**, branch main, release run ID dari summary
**Backend release**, dan tujuan SIT/UAT/STG/PRD. Workflow memastikan release
berasal dari push main pada be-release.yml yang sukses; manifest cocok dengan
commit/run/GHCR repository dan digest itu pernah sukses di environment sebelumnya.
Catatan bukti menggunakan GitHub Deployments task `dikasa-release`, dipaginasi.
Lock environment mencegah dua workflow deployment berjalan bersamaan.

Koyeb menerima update service melalui API resmi `PUT /v1/services/{id}`. Workflow
menunggu ID deployment baru menjadi HEALTHY dan aktif; status healthy revision
lama tidak cukup. Setelah itu smoke test memeriksa OpenAPI, penolakan health tanpa
token dan token invalid, login, serta health dengan token sah pada URL tujuan.
Ini rolling deployment Koyeb; traffic dapat sudah melayani image baru ketika
smoke berjalan.

Jika deployment atau smoke gagal dan ID update diketahui, workflow memulihkan
definisi service sebelumnya dan menunggu recovery sehat. Workflow tetap gagal
sehingga release tidak memperoleh bukti promosi sukses. Recovery menolak menimpa
deployment lain yang lebih baru. Jika respons update hilang, statusnya tidak
pasti; periksa console Koyeb sebelum retry. Kegagalan recovery memerlukan operator.
Schema/data yang berhasil dimigrasikan tidak dibatalkan oleh recovery aplikasi.

Rollback manual memilih release yang sebelumnya sukses di **environment tujuan**
dan mencentang `rollback`. Tidak ada rebuild dan tidak ada down migration;
migrasi dilewati. Gunakan expand–contract dan tinjau kompatibilitas skema sebelum
approval. Smoke/recovery dan approval environment tetap berlaku saat rollback.

Manifest disimpan 90 hari. Pertahankan run, artifact, GitHub Deployment records,
dan digest GHCR untuk masa rollback yang diperlukan. Artifact kedaluwarsa atau
image terhapus akan memblokir promosi/rollback. Rerun build yang sudah mengunggah
manifest ditolak oleh artifact immutable; kandidat baru memakai release baru.

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

Test Koyeb memakai API/migrasi/smoke palsu dan tidak membuat resource cloud.
`bash deploy/test-migrate.sh` harus memakai database CI kosong khusus melalui
`DATABASE_URL`; `DIKASA_TEST_DATABASE_URL` mengaktifkan integration test Go.
ERD hanya menambahkan ledger operasional migrasi, tanpa relasi bisnis baru.

Referensi: [Pre-built Docker images](https://www.koyeb.com/docs/build-and-deploy/prebuilt-docker-images),
[Koyeb Secrets](https://www.koyeb.com/docs/reference/secrets),
[API resmi](https://www.koyeb.com/docs/reference/api),
[GitHub Environments](https://docs.github.com/en/actions/reference/workflows-and-actions/deployments-and-environments).
