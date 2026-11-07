# Menggunakan ScopeGuard API di Apidog

Spesifikasi OpenAPI 3.0.3 mencakup seluruh 9 endpoint HTTP yang sudah terdaftar,
termasuk request body, response, parameter organisasi, dan autentikasi Bearer JWT.

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
6. Untuk daftar role atau permission, isi parameter `organization_id` dengan UUID
   organisasi yang dapat diakses akun tersebut.

## Memperbarui spesifikasi

Sumber kontrak ada di
[`apps/api/internal/platform/openapi/openapi.json`](../apps/api/internal/platform/openapi/openapi.json).
File ini dikelola bersama kode API; ekspor menyalinnya, bukan membangkitkan schema
secara otomatis dari handler Go. Saat endpoint atau DTO berubah, perbarui file
ini, jalankan `make test`, lalu ekspor atau impor ulang. Test membandingkan method
dan path spesifikasi dengan router aplikasi agar endpoint tidak terlewat.

Dokumen yang sama di-embed ke binary API. Setelah mengubahnya, restart atau build
ulang API agar `/openapi.json` menyajikan versi terbaru.

Referensi resmi: [Manual Import](https://docs.apidog.com/en/manual-import-633884m0),
[Import Options](https://docs.apidog.com/en/import-options-633930m0), dan
[Scheduled Import](https://docs.apidog.io/scheduled-import-bind-data-sources-633932m0).
