# M2 Authorization Readiness — ScopeGuard

## Scope

M2 mencakup role management, role permission management, dan membership role
assignment dalam lingkungan multi-tenant.

## Security invariants

1. Seluruh authorization mutation harus menggunakan tenant lock sebelum
   mengevaluasi hak akses atau memodifikasi data.
2. Role dan membership harus berada dalam organisasi yang sama.
3. Aktor tidak boleh mendelegasikan permission yang tidak dimilikinya.
4. Role `owner` tidak boleh diberikan atau dilepas melalui membership role
   assignment API.
5. Membership tidak boleh kehilangan role terakhirnya melalui role removal API.
6. Read endpoint harus dilindungi permission guard yang sesuai.
7. Perubahan permission efektif harus diperiksa terhadap PostgreSQL, bukan
   semata-mata JWT.
8. Akses tulis langsung ke tabel RBAC harus dibatasi agar tidak melewati aturan
   transaksi aplikasi.

## Cakupan security review

Pengujian tambahan ada di
[`m2_authorization_hardening_test.go`](../apps/api/tests/integration/m2_authorization_hardening_test.go)
dan melengkapi suite authorization yang sudah ada.

| Pengujian | Bukti yang diperiksa |
| --- | --- |
| `TestM2ConcurrentDifferentRoleRemovalsPreserveLastRole` | Dua penghapusan role berbeda menunggu tenant lock; tepat satu berhasil, satu menghasilkan `ErrLastMembershipRole`, dan satu role tersisa. |
| `TestM2RoleReadDoesNotRequirePermissionRead` | Daftar permission role tetap dapat dibaca dengan `role:read` tanpa `permission:read`, sementara katalog permission menghasilkan `403`. |
| `TestM2MemberReadDoesNotRequireRoleAssign` | Daftar role membership tetap dapat dibaca dengan `member:read` tanpa `member:role_assign`, sementara assignment menghasilkan `403`. |
| `TestM2CompositeFKRejectsCrossTenantAssignments` | INSERT langsung yang mencampur organisasi dengan membership atau role tenant lain ditolak PostgreSQL dengan SQLSTATE `23503`, termasuk constraint yang diperiksa saat COMMIT. |

Tes konkurensi memakai `runConcurrentRolePermissionMutations`. Helper menahan
baris organisasi dengan `FOR UPDATE` dan memeriksa rantai blocking PostgreSQL
hingga kedua operasi menunggu lock sebelum melepasnya. Ini melengkapi tes
penghapusan role yang sama, yang memeriksa idempotency dan serialisasi.

Suite authorization yang sudah ada tetap dijalankan untuk memeriksa delegasi
permission, proteksi owner, aktor aktif, isolasi tenant, dan perubahan permission
efektif. Review privilege database production, perlindungan branch, dan required
CI checks memerlukan verifikasi konfigurasi deployment dan repository.

## Hasil pengujian

Validasi lokal dilakukan pada 14 November 2026, pada working tree berbasis
commit `4e4becc` dengan tambahan tes hardening dan langkah CI M2. Lingkungan
pengujian memakai Go 1.27.2, PostgreSQL 17.11, dan Goose 3.29.0 pada macOS arm64.
Pengujian database memakai database sementara dengan seluruh 11 migrasi sudah
diterapkan.

| Pemeriksaan | Hasil |
| --- | --- |
| Unit tests dengan race detector | Lulus: `go test -race ./... -count=1`. |
| PostgreSQL integration tests lengkap | Lulus: 122 tes dan 43 subtes, tanpa tes yang dilewati. |
| M2 hardening dengan race detector | Lulus: empat tes dan dua subtes, tanpa laporan data race. |
| OpenAPI contract test | Lulus, termasuk dalam `go test -race ./... -count=1`. |
| Go vet dan formatting | Lulus: `go vet ./...` dan pemeriksaan `gofmt` seluruh kode API. |
| Build API | Lulus: binary dihasilkan di `/tmp/scopeguard-m2-api`. |
| Validasi migrasi dan rollback | Lulus: validasi, penerapan seluruh migrasi, lalu `goose down` dan `goose up` untuk migrasi terakhir `20261025000757`. |
| Konfigurasi langkah CI | YAML valid; suite integration lengkap diikuti race M2 dan rollback migrasi. |

Kelulusan lokal harus dilengkapi hasil CI pada commit final sebelum menutup
milestone. CI tetap memakai Goose 3.27.3 sesuai workflow. Kebijakan privilege
database production, perlindungan branch, dan required CI checks belum
diverifikasi dalam validasi lokal ini.

Dari direktori `apps/api`, jalankan dengan `TEST_DATABASE_URL` yang menunjuk
database test dengan seluruh migrasi sudah diterapkan:

```sh
go test -race ./... -count=1
go test -tags=integration ./tests/integration/... -count=1 -v
go test -race -tags=integration ./tests/integration/... \
  -run '^TestM2' -count=1 -v
go test ./internal/bootstrap/api -run '^TestOpenAPIContract$' -count=1
go vet ./...
test -z "$(gofmt -l .)"
go build -o /tmp/scopeguard-m2-api ./cmd/api
```

Job **PostgreSQL & Migration Tests** pada
[`api-ci.yml`](../.github/workflows/api-ci.yml) menjalankan suite integration
lengkap, lalu mengulang hanya `TestM2` dengan race detector sebelum menguji
rollback migrasi (`goose down` lalu `goose up`).

## Release acceptance

- [ ] Unit tests dan race detector lulus.
- [ ] PostgreSQL integration tests lulus.
- [ ] Concurrent removal terhadap dua role berbeda menjaga minimal satu role.
- [ ] Database menolak relasi role atau membership lintas tenant.
- [ ] Regression test authorization guard lulus.
- [ ] OpenAPI contract test lulus.
- [ ] Build API dan migration rollback lulus.
- [ ] Kebijakan akses database production sudah ditinjau.
- [ ] Perlindungan branch dan required CI checks sudah dikonfigurasi.

Milestone dapat dinyatakan siap ditutup setelah seluruh acceptance criteria yang
relevan terpenuhi dan diverifikasi pada commit final. Checklist ini mencatat
penutupan pada commit final; hasil lokal dicatat terpisah di atas.
