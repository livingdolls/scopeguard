# Menggunakan ScopeGuard API di Apidog

Spesifikasi OpenAPI 3.0.3 mencakup seluruh 11 operasi HTTP aplikasi yang sudah
terdaftar, termasuk CreateRole dan Grant permission. Kontrak memuat request body,
response, parameter organisasi dan role, serta autentikasi Bearer JWT.

## Ekspor file

Jalankan dari root proyek:

```sh
make api-export
```

File hasil ekspor ada di `dist/openapi.json`. Perintah ini tidak memerlukan API
yang sedang berjalan, database, maupun konfigurasi `.env`.

Di Apidog, buka proyek lalu pilih **Settings → Import Data**, masukkan file
`dist/openapi.json`, periksa preview endpoint, lalu konfirmasi impor. Apidog
mengenali format OpenAPI secara otomatis. Untuk impor ulang, pilih **Merge**
atau **Overwrite** sesuai perubahan yang ingin diterapkan.

## Impor dari URL

Setelah database, migrasi, dan `.env` API siap, jalankan:

```sh
make api-run
```

Di dialog impor Apidog, masukkan:

```text
http://localhost:8080/openapi.json
```

Sesuaikan port jika `HTTP_ADDR` berbeda. URL ini mengembalikan dokumen OpenAPI
langsung dan tidak memerlukan login. Gunakan Apidog desktop untuk mengakses
server lokal; jika memakai aplikasi web, akses URL jaringan lokal memerlukan
ekstensi browser Apidog.

Untuk sinkronisasi berkala, buka **Settings → Import Data → Scheduled Import
(Bind Data Sources)** dan tambahkan URL tersebut. Pilih **Local Client** untuk
server lokal; aplikasi desktop dan proyek Apidog harus tetap terbuka agar impor
berjalan. Sumber Git repository juga dapat diarahkan ke file spesifikasi proyek.

## Base URL dan autentikasi

1. Atur base URL environment Apidog ke `http://localhost:8080` atau alamat API
   yang digunakan. Path endpoint sudah menyertakan `/v1`, jadi jangan tambahkan
   `/v1` lagi ke base URL.
2. Panggil `POST /v1/auth/register` dengan email dan password minimal 15 karakter.
3. Ambil token dari email verifikasi, lalu kirim ke `POST /v1/auth/verify-email`.
   Pengiriman email memerlukan email worker dan konfigurasi SMTP yang aktif.
4. Panggil `POST /v1/auth/login`, lalu salin `access_token` dari response.
5. Pilih **Bearer Token** pada autentikasi request yang dilindungi dan isi token
   tersebut. Token berlaku selama 15 menit.
6. Untuk operasi authorization, isi parameter `organization_id` dengan UUID
   organisasi yang dapat diakses akun tersebut. Untuk Grant permission, isi juga
   `role_id` dengan UUID custom role di organisasi yang sama.

## Endpoint authorization

Seluruh endpoint berikut memerlukan akun aktif dengan email terverifikasi dan
membership pada organisasi yang dipilih. Permission efektif diperiksa pada
PostgreSQL untuk setiap request.

| Method | Path | Permission aktor pada organisasi |
| --- | --- | --- |
| GET | `/v1/organizations/{organization_id}/permissions` | `permission:read` |
| GET | `/v1/organizations/{organization_id}/roles` | `role:read` |
| POST | `/v1/organizations/{organization_id}/roles` | `role:create` |
| POST | `/v1/organizations/{organization_id}/roles/{role_id}/permissions` | `role:update` dan permission yang akan diberikan |

## Mencoba CreateRole dan Grant permission

Gunakan Bearer token dari alur login di atas. Untuk request POST, atur
`Content-Type: application/json`. Body CreateRole dan Grant permission dibatasi
16 KiB; field yang tidak dikenal dan beberapa nilai JSON dalam satu body ditolak.
Identitas aktor diambil dari JWT.

### 1. Buat organisasi

Kirim `POST /v1/organizations` dengan slug organisasi yang unik:

```json
{
  "name": "Apidog Role Test",
  "slug": "apidog-role-test"
}
```

Response `201 Created` memuat `id` organisasi. Gunakan nilai tersebut sebagai
`organization_id`. Akun pembuat menjadi owner dengan permission awal untuk
membuat role dan memberikan permission.

### 2. Buat custom role

Kirim `POST /v1/organizations/{organization_id}/roles`:

```json
{
  "name": "Content Editor",
  "slug": "content_editor"
}
```

Response `201 Created` memuat role dengan `is_system: false`. Gunakan field `id`
dari response sebagai `role_id` pada langkah berikutnya. Custom role baru belum
memiliki permission.

Nama role dipangkas dari whitespace di awal dan akhir, lalu harus berisi 1–80
karakter Unicode. Slug harus cocok dengan `^[a-z][a-z0-9_]{0,62}$` dan unik dalam
organisasi. Slug `owner` dan `member` digunakan oleh system role dan ditolak untuk
custom role. Slug custom role yang sama boleh digunakan di organisasi lain.

### 3. Berikan permission pada role

Ambil kode permission yang terdaftar melalui
`GET /v1/organizations/{organization_id}/permissions`. Kemudian kirim
`POST /v1/organizations/{organization_id}/roles/{role_id}/permissions`:

```json
{
  "permission_code": "member:invite"
}
```

Aktor harus memiliki `role:update` dan `member:invite` pada organisasi tersebut.
Target harus berupa custom role milik organisasi yang sama.

| Kondisi | HTTP status | Field `added` |
| --- | --- | --- |
| Assignment baru dibuat | `201 Created` | `true` |
| Assignment sudah ada | `200 OK` | `false` |

Contoh response untuk assignment baru; nilai `role_id` mengikuti UUID role yang
dikirim pada path:

```json
{
  "role_id": "420f90b8-91e2-46e8-8a46-ac58c57f56aa",
  "permission_code": "member:invite",
  "added": true
}
```

Request berulang tetap memeriksa hak aktor sebelum mengembalikan `added: false`.

## Membaca respons error

Error dikembalikan sebagai objek JSON dengan field `error`.

| HTTP status | Penyebab pada CreateRole atau Grant permission |
| --- | --- |
| `400 Bad Request` | UUID, JSON, field, slug role, atau format kode permission tidak valid. |
| `401 Unauthorized` | Bearer token tidak tersedia atau tidak valid, atau akun tidak aktif. |
| `403 Forbidden` | Aktor bukan anggota tenant atau tidak memiliki permission yang diperlukan. |
| `404 Not Found` | Target role tidak ditemukan pada tenant atau permission tidak terdaftar. Bisa juga terjadi jika organisasi terhapus setelah pemeriksaan akses. |
| `409 Conflict` | CreateRole: slug sudah digunakan dalam tenant. Grant permission: target merupakan system role `owner` atau `member`. |
| `413 Request Entity Too Large` | Body melebihi 16 KiB. |
| `415 Unsupported Media Type` | Content-Type bukan `application/json`. |
| `503 Service Unavailable` | Pemeriksaan akses atau layanan pendukung gagal. |

JWT mengidentifikasi user; permission pada PostgreSQL menjadi sumber otorisasi.
Jika `role:create` dicabut setelah token diterbitkan, token yang sama tetap bisa
mengautentikasi akun, tetapi request CreateRole berikutnya mendapat `403`.
Pencabutan permission berlaku tanpa menunggu token kedaluwarsa.

Mutasi authorization pada repository mengikuti konvensi lock organisasi untuk
menjaga konsistensi per tenant. Penulisan langsung ke `role_permissions` yang
melewati lock tersebut dapat melewati serialisasi. Saat deployment, batasi
privilege user database aplikasi dan kendalikan seluruh jalur penulisan
authorization agar mengikuti konvensi yang sama.

## Memperbarui spesifikasi

Sumber kontrak ada di
[`apps/api/internal/platform/openapi/openapi.json`](../apps/api/internal/platform/openapi/openapi.json).
File ini dikelola bersama kode API; ekspor menyalinnya, bukan membangkitkan schema
secara otomatis dari handler Go. Saat endpoint atau DTO berubah, perbarui file
ini dan jalankan `make test`. Untuk memeriksa kontrak OpenAPI saja dari direktori
API:

```sh
cd apps/api
go test ./internal/bootstrap/api -run '^TestOpenAPIContract$' -count=1
```

Test tersebut memeriksa method dan path terhadap router produksi, keunikan
`operationId`, dan referensi schema. Test kontrak tidak membutuhkan database
yang aktif. Setelah lolos, jalankan `make api-export` dari root proyek dan impor
ulang `dist/openapi.json` ke Apidog.

Dokumen yang sama di-embed ke binary API. Setelah mengubahnya, restart atau build
ulang API agar `/openapi.json` menyajikan versi terbaru.

Referensi resmi: [Manual Import](https://docs.apidog.com/en/manual-import-633884m0),
[Import Options](https://docs.apidog.com/en/import-options-633930m0), dan
[Scheduled Import](https://docs.apidog.io/scheduled-import-bind-data-sources-633932m0).
