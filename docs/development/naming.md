# Konvensi penamaan Go dan API Dikasa

## Folder dan file

- Folder package memakai huruf kecil, singkat, dan singular berdasarkan feature:
  `auth`, `user`, `server`. Gunakan `shared` hanya untuk komponen lintas feature.
- File Go memakai `snake_case.go`: `repository_gorm.go`, `service_test.go`.
- Test berada di package yang sama dengan nama `<file>_test.go`.
- Migration memakai urutan, aksi, dan arah:
  `000001_create_identity_tables.up.sql` dan pasangan `.down.sql`.

## Simbol Go

- Exported type/function memakai `PascalCase` dan wajib memiliki Go doc comment.
- Unexported function/variable memakai `camelCase`.
- Interface dinamai berdasarkan kemampuan, misalnya `TokenIssuer` dan
  `Repository`; hindari awalan `I`.
- Constructor memakai `New<Type>`, misalnya `NewService` dan `NewHandler`.
- Sentinel error memakai `Err<Name>`, misalnya `ErrNotFound`.

## PostgreSQL dan JSON

- Tabel PostgreSQL memakai plural `snake_case`: `users`, `roles`, `outlets`.
- Kolom dan foreign key memakai `snake_case`: `password_hash`, `role_id`.
- Field JSON memakai `snake_case`: `access_token`, `phone_number`.
- Jangan kirim entity GORM langsung sebagai response. Gunakan DTO agar password
  hash dan field internal tidak pernah terekspos.

## Path API

- Prefix wajib: `/dikasa/v1`.
- Resource memakai noun plural: `/users/me`, `/products`, `/transactions/{id}`.
- Gunakan HTTP method untuk aksi CRUD. Endpoint use case seperti `/login` boleh
  memakai kata kerja karena bukan CRUD resource biasa.
