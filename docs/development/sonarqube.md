# SonarQube Cloud gratis untuk backend DIKASA

Integrasi menggunakan **SonarQube Cloud (sebelumnya SonarCloud), paket Free**,
region EU di `https://sonarcloud.io`. Tidak membutuhkan server SonarQube, VPS,
Docker service tambahan, atau konfigurasi pada Render. Repo GitHub ini public;
paket Free mendukung analisis project public. Bila repo berubah menjadi private,
Free memiliki batas total 50k private LOC dan maksimal 5 anggota organisasi.
Lihat [perbandingan paket resmi](https://docs.sonarsource.com/sonarqube-cloud/administering-sonarcloud/managing-subscription/subscription-plans).

Free mendukung main branch dan PR dengan target main; pipeline hanya memakai
keduanya. OSS adalah paket gratis lain dengan kelayakan tersendiri; repo public
tidak otomatis membuktikan kelayakan organisasi OSS. Tidak diperlukan trial atau
paket Team untuk cakupan integrasi ini. Pilih Free ketika setup organisasi.

## Yang perlu Anda siapkan

1. Buka [SonarQube Cloud EU](https://sonarcloud.io) dan login memakai GitHub.
2. Import/bind organisasi GitHub `DIKASA-COMPANY`, lalu pilih repo `be-dikasa`.
   Otorisasi GitHub App SonarQube Cloud untuk repo tersebut. Bila organisasi
   membatasi pemasangan app, owner organisasi perlu menyetujuinya.
3. Pilih paket **Free**, main branch `main`, dan metode analisis **With GitHub
   Actions / CI-based analysis**. Matikan **Automatic Analysis** untuk project
   ini agar tidak terjadi analisis ganda. Coverage Go memerlukan CI-based analysis.
4. Salin **Organization Key** dan **Project Key** dari tutorial konfigurasi project.
   Nama tampilan organisasi/project belum tentu sama dengan key-nya.
5. Buat personal access token dari **My Account → Security → Generate Tokens**
   menggunakan akun yang memiliki **Execute Analysis** pada project. Paket Free
   memakai personal token; jangan menaruh token di source code atau chat.
6. Pada GitHub repo, buka **Settings → Secrets and variables → Actions** dan isi:

| Jenis | Nama | Nilai |
| --- | --- | --- |
| Repository variable | `SONAR_ORGANIZATION` | Organization Key dari SonarQube Cloud |
| Repository variable | `SONAR_PROJECT_KEY` | Project Key dari SonarQube Cloud |
| Repository secret | `SONAR_TOKEN` | Personal access token yang dapat menjalankan analisis |
| Repository variable | `SONAR_ENABLED` | `true` setelah tiga konfigurasi di atas siap |

Ini konfigurasi **repository**, bukan environment `dev`/`prd`. Sonar tidak
membutuhkan credential PostgreSQL, Supabase, SMTP, Render, atau GHCR. Host EU
dikunci di workflow. Jika memilih region US pada SonarQube Cloud, sesuaikan host/
region scanner sesuai dokumentasi Sonar sebelum mengaktifkan flag.

## Aktivasi dan baseline pertama

Sebelum konfigurasi tersedia, biarkan `SONAR_ENABLED` kosong atau `false`.
Job Sonar dilewati, tetapi unit test, PostgreSQL integration dan build tetap
berjalan. Scan nonaktif tidak berarti kode sudah lulus SonarQube Quality Gate.

Setelah perubahan pipeline masuk main dan konfigurasi tersedia:

1. Set `SONAR_ENABLED=true`.
2. Buka GitHub **Actions → Backend CI → Run workflow**, pilih branch **main**,
   lalu jalankan workflow baru. Ini membuat baseline main tanpa publish image
   release atau deployment. Jangan memakai rerun release lama untuk bootstrap.
3. Periksa dashboard project SonarQube Cloud. Bila Quality Gate gagal, perbaiki
   temuan/coverage sebelum mengandalkan gate pada release.
4. Buka atau update PR dengan target main; job **SonarQube Cloud quality gate**
   akan memeriksa perubahan setelah unit test lulus. GitHub App yang sudah terikat
   dapat menampilkan status/dekorasi PR.
5. Setelah scan pertama berhasil, tambahkan check Sonar sebagai required check
   pada branch protection/ruleset main sesuai kebijakan tim. Status job CI yang
   sudah dilewati sebelum aktivasi bukan bukti baseline Sonar yang sukses.

## Alur dan quality gate

```text
Format / vet / kontrak API
→ Unit test + race + coverage.out
→ PostgreSQL integration dan SonarQube berjalan paralel
→ Build hanya setelah keduanya lulus (atau Sonar memang nonaktif/dilewati)
→ Release image / Render melalui pipeline yang ada
```

Job Sonar mengambil artifact coverage dari **run yang sama**, checkout history
lengkap, menormalisasi path module Go ke file dalam checkout, kemudian menjalankan
action resmi Sonar yang dikunci ke commit tertentu. Scanner menunggu Quality Gate
maksimal 300 detik. Gagal scan, token/konfigurasi salah, timeout, atau Quality Gate
merah membuat CI gagal dan menahan build/publish/deployment pada jalur release.
Tidak ada `continue-on-error` untuk analisis.

Release meneruskan hanya `SONAR_TOKEN` ke reusable CI. Fork PR dan workflow
Dependabot dilewati agar token tidak diberikan kepada kode eksternal. Workflow
menggunakan `pull_request`, bukan `pull_request_target`, dan tidak menjalankan
analisis ber-secret melalui `workflow_run`. PR eksternal tetap memperoleh test
dan build; kebijakan review maintainer perlu memperhitungkan Sonar yang dilewati.

## Cakupan analisis

- Production Go: `cmd` dan `internal`.
- Go tests diklasifikasikan terpisah, termasuk test migrasi di `migrations`.
- Coverage memakai unit report `go test -race -coverprofile=coverage.out ./...`.
  Coverage integration PostgreSQL tidak digabung atau diklaim sebagai unit coverage.
- Tidak mengecualikan file produksi agar angka coverage tampak lebih tinggi.
- Tidak mengubah Quality Profile atau Quality Gate server secara otomatis;
  gunakan bawaan Sonar way yang tersedia pada paket Free.
- Script CI, dokumentasi, dan SQL tidak termasuk scope scanner Go ini; mereka
  tetap diperiksa oleh Python tests, actionlint, ShellCheck, serta migration tests.

`scripts/cicd/prepare_sonar.py` membuat `coverage-sonar.out` tanpa mengubah report
asli atau counter hit. Record kosong/rusak, source hilang, atau path di luar
checkout ditolak agar import coverage tidak diam-diam menghasilkan hasil keliru.
Report dan `.scannerwork` diabaikan oleh Git.

Tidak ada perubahan endpoint, model bisnis, OpenAPI/Bruno, SQL, atau relasi ERD.
Live scan baru dapat diverifikasi setelah project, key, dan token disiapkan.

Referensi: [GitHub Actions](https://docs.sonarsource.com/sonarqube-cloud/analyzing-source-code/ci-based-analysis/github-actions-for-sonarcloud),
[Go coverage](https://www.sonarsource.com/blog/how-sonarqube-coverage-reporting-works/),
[Paket Free/OSS](https://docs.sonarsource.com/sonarqube-cloud/administering-sonarcloud/managing-subscription/subscription-plans).
