# Menggunakan ScopeGuard API di Apidog

Spesifikasi OpenAPI 3.0.3 mencakup seluruh 14 operasi HTTP aplikasi yang sudah
terdaftar, termasuk CreateRole, Grant permission, Revoke permission,
AssignRoleToMember, dan RemoveRoleFromMember. Kontrak memuat request body,
response, parameter organisasi, membership, role, dan kode permission, serta
autentikasi Bearer JWT.

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
   organisasi yang dapat diakses akun tersebut. Untuk Grant atau Revoke
   permission, isi juga `role_id` dengan UUID custom role di organisasi yang
   sama. Revoke menerima `permission_code` sebagai parameter path.
7. Untuk AssignRoleToMember atau RemoveRoleFromMember, isi `membership_id`
   dengan UUID keanggotaan target dan `role_id` dengan UUID role dari organisasi
   yang sama. `membership_id` berbeda dari ID user.

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
| DELETE | `/v1/organizations/{organization_id}/roles/{role_id}/permissions/{permission_code}` | `role:update` dan permission yang akan dicabut |
| POST | `/v1/organizations/{organization_id}/memberships/{membership_id}/roles/{role_id}` | `member:role_assign` dan seluruh permission pada role target |
| DELETE | `/v1/organizations/{organization_id}/memberships/{membership_id}/roles/{role_id}` | `member:role_assign` dan seluruh permission pada role target |

## Mencoba role, permission, dan assignment membership

Gunakan Bearer token dari alur login di atas. Untuk membuat organisasi, membuat
role, dan Grant permission, atur `Content-Type: application/json`. Body
CreateRole dan Grant permission dibatasi 16 KiB; field yang tidak dikenal dan
beberapa nilai JSON dalam satu body ditolak. Revoke permission,
AssignRoleToMember, dan RemoveRoleFromMember tidak memerlukan body maupun header
Content-Type. Identitas aktor diambil dari JWT.

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

### 4. Cabut permission dari role

Kirim `DELETE /v1/organizations/{organization_id}/roles/{role_id}/permissions/{permission_code}`.
Gunakan `organization_id` dan `role_id` yang sama, lalu isi parameter
`permission_code` dengan `member:invite`. Request ini tidak memerlukan body
maupun header Content-Type.

Aktor tetap harus memiliki `role:update` dan `member:invite` pada organisasi
tersebut. Permission hanya dicabut dari custom role yang dituju.

| Kondisi | HTTP status | Field `removed` |
| --- | --- | --- |
| Assignment berhasil dicabut | `200 OK` | `true` |
| Assignment sudah tidak ada | `200 OK` | `false` |

Contoh response untuk pencabutan yang menghapus assignment:

```json
{
  "role_id": "420f90b8-91e2-46e8-8a46-ac58c57f56aa",
  "permission_code": "member:invite",
  "removed": true
}
```

Ulangi request yang sama untuk memperoleh `removed: false`. Kedua hasil
memerlukan otorisasi yang sama; assignment yang sudah tidak ada tetap
memerlukan permission terdaftar dan hak aktor yang lengkap.

### 5. Assign role ke membership

Kirim `POST /v1/organizations/{organization_id}/memberships/{membership_id}/roles/{role_id}`
tanpa body. Gunakan role yang sudah dibuat dan membership yang sudah ada di
organisasi tersebut. Ambil `membership_id` dari data uji atau administrasi
organisasi; nilainya adalah ID keanggotaan, bukan ID user. Operasi ini menambah
role pada membership yang sudah ada.

Aktor harus memiliki `member:role_assign` dan seluruh permission pada role
target dalam organisasi yang sama. Jika role memiliki `member:invite`, aktor
juga harus memiliki permission tersebut. Aturan ini tetap diperiksa untuk
assignment berulang.

Custom role dan system role `member` dapat diberikan melalui endpoint ini.
Assignment role `owner` ditolak dengan `409 Conflict`.

| Kondisi | HTTP status | Field `assigned` |
| --- | --- | --- |
| Role baru dipasangkan ke membership | `201 Created` | `true` |
| Membership sudah memiliki role tersebut | `200 OK` | `false` |

Contoh response untuk assignment baru; UUID mengikuti parameter path:

```json
{
  "membership_id": "c2df2916-9d17-4d4c-8925-f647777a895c",
  "role_id": "420f90b8-91e2-46e8-8a46-ac58c57f56aa",
  "assigned": true
}
```

### 6. Cabut role dari membership

Kirim `DELETE /v1/organizations/{organization_id}/memberships/{membership_id}/roles/{role_id}`
tanpa body. Gunakan `membership_id` dan `role_id` yang sama dengan langkah
assignment. Operasi ini mencabut pasangan membership dan role; hak efektif user
setelahnya dihitung dari role yang masih dimilikinya.

Aktor harus memiliki `member:role_assign` dan seluruh permission pada role target
dalam organisasi tersebut. Role `owner` dilindungi, dan penghapusan assignment
yang merupakan role terakhir pada membership ditolak dengan `409 Conflict`.
Siapkan membership yang memiliki role lain sebelum mencoba penghapusan.

| Kondisi | HTTP status | Field `removed` |
| --- | --- | --- |
| Assignment role berhasil dicabut | `200 OK` | `true` |
| Assignment role sudah tidak ada | `200 OK` | `false` |

Contoh response untuk penghapusan assignment:

```json
{
  "membership_id": "c2df2916-9d17-4d4c-8925-f647777a895c",
  "role_id": "420f90b8-91e2-46e8-8a46-ac58c57f56aa",
  "removed": true
}
```

Request berulang tetap memeriksa hak aktor, kepemilikan tenant, keberadaan role
dan membership, serta proteksi owner. `removed: false` berlaku ketika resource
target valid dan assignment sudah tidak ada. Role atau membership yang tidak
ditemukan pada tenant menghasilkan `404`.

## Membaca respons error

Error dikembalikan sebagai objek JSON dengan field `error`.

| HTTP status | CreateRole atau Grant permission | Revoke permission |
| --- | --- | --- |
| `400 Bad Request` | UUID, JSON, field, slug role, atau format kode permission tidak valid. | UUID atau format kode permission pada path tidak valid. |
| `401 Unauthorized` | Bearer token tidak tersedia atau tidak valid, atau akun tidak aktif. | Sama. |
| `403 Forbidden` | Aktor bukan anggota tenant atau tidak memiliki permission yang diperlukan. | Sama. |
| `404 Not Found` | Target role tidak ditemukan pada tenant atau permission tidak terdaftar. Bisa juga terjadi jika organisasi terhapus setelah pemeriksaan akses. | Target role tidak ditemukan pada tenant, termasuk role milik tenant lain; permission tidak terdaftar; atau organisasi terhapus setelah pemeriksaan akses. |
| `409 Conflict` | CreateRole: slug sudah digunakan dalam tenant. Grant permission: target merupakan system role `owner` atau `member`. | Target merupakan system role `owner` atau `member`. |
| `413 Request Entity Too Large` | Body melebihi 16 KiB. | Tidak digunakan; Revoke tidak membaca body. |
| `415 Unsupported Media Type` | Content-Type bukan `application/json`. | Tidak digunakan; Revoke tidak memerlukan Content-Type. |
| `503 Service Unavailable` | Pemeriksaan akses atau layanan pendukung gagal. | Sama. |

Untuk AssignRoleToMember dan RemoveRoleFromMember:

| HTTP status | Penyebab |
| --- | --- |
| `400 Bad Request` | UUID organisasi, membership, atau role tidak valid. |
| `401 Unauthorized` | Bearer token tidak tersedia atau tidak valid, atau akun tidak aktif. |
| `403 Forbidden` | Aktor bukan anggota tenant, tidak memiliki `member:role_assign`, atau tidak memiliki seluruh permission pada role target. |
| `404 Not Found` | Membership atau role tidak ditemukan pada tenant, termasuk target dari tenant lain; atau organisasi terhapus setelah pemeriksaan akses. |
| `409 Conflict` | AssignRoleToMember: target assignment adalah role `owner`. RemoveRoleFromMember: target adalah role `owner` atau assignment yang akan dihapus merupakan role terakhir pada membership. |
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
