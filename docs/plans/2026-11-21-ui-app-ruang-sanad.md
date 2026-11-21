# Rencana Teknis: UI Custom `/app` untuk GOWA Ruang Sanad

Status: draf untuk direview, belum ada kode yang diubah.
Tanggal: 2026-11-21

## 1. Keputusan yang sudah diambil

| Topik | Keputusan | Sumber |
|---|---|---|
| Pengguna | Admin dan CS | jawaban user |
| Kontrol | Penuh; tidak bergantung pada update UI upstream | jawaban user |
| Lokasi analisis | Di dalam GOWA | jawaban user |
| Arsitektur | **Hybrid**: UI upstream tetap di `/`, UI milik sendiri di `/app` | jawaban user (setelah temuan lisensi) |

### Alasan hybrid (temuan lisensi)
- `aldinokemal/gowa-ui` **tidak punya file lisensi** (dicek pada commit 2026-11-20). Default hukumnya "all rights reserved".
- Repo `ruangsanadindonesia-art/go-whatsapp-web-multidevice` **publik**, jadi menyalin source UI ke sini sama dengan redistribusi tanpa izin.
- Memakai file rilis HTML-nya apa adanya (mekanisme `uiasset` saat ini) tetap boleh. Karena itu `/` dibiarkan.
- `/app` ditulis 100% dari nol. Semua kode di sana milik Ruang Sanad. **Jangan menyalin file atau komponen dari gowa-ui**, termasuk dengan cara copy-paste lalu diubah.
- Bila nanti penulis memberi lisensi (mis. MIT), `/app` bisa mengambil alih `/` sepenuhnya.

Ini penilaian risiko berdasarkan aturan default hak cipta, bukan nasihat hukum.

## 2. Kondisi kode saat ini (fakta)

- UI `/` diunduh saat runtime dari GitHub Release `aldinokemal/gowa-ui`: `src/cmd/rest.go:318-367`, `src/infrastructure/uiasset/`, `src/config/settings.go:31-37`.
- `AGENTS.md` masih menyebut `src/views/` (Vue 3). Folder itu sudah tidak ada, jadi dokumennya **usang** dan perlu diperbaiki di fase 1.
- Halaman custom yang sudah ada: `/integrations` (HTML dirender Go, `src/cmd/integrations_page.go`) dan `/cs-ai` (proxy ke `hermes-cs-bridge`).
- Auth: login web dan session cookie (`src/cmd/web_auth.go`). Akun dibagi dua:
  - Admin: akun di `APP_BASIC_AUTH` tanpa scope.
  - Akun terbatas: didaftarkan di `APP_BASIC_AUTH_DEVICES`, hanya bisa mengakses device tertentu dan endpoint messaging (`src/cmd/device_scope.go`). **Akun ini cocok untuk CS.**
- Data chat: SQLite tabel `chats` dan `messages` (`id, chat_jid, device_id, sender, content, timestamp, is_from_me, media_type, ...`), dengan index di `device_id`, `timestamp`, `chat_jid` (`src/infrastructure/chatstorage/sqlite_repository.go:2901-2973`).
- Deploy: `deploy/deploy.sh` mem-build Docker image di server dari checkout repo (`docker/golang.Dockerfile`, saat ini hanya ada stage Go).
- Data lead/CRM ada di Dashboard Ruang Sanad. Yang diketahui sejauh ini hanya endpoint **ingest** (`DASHBOARD_URL/integrations/cs-ai/events`). Belum diketahui apakah ada **read API** untuk GOWA.

## 3. Arsitektur target

```
Browser ──> GOWA (Fiber)
             ├── /            UI upstream (tidak diubah)
             ├── /app/*       SPA Ruang Sanad (embed, go:embed)
             ├── /api/app/*   endpoint baru khusus /app (analisis, inbox, template)
             ├── /send/*, /chat/*, ... endpoint lama (dipakai ulang oleh /app)
             └── /integrations, /cs-ai (tetap)
```

- **Frontend:** folder baru `web/` di root repo. Stack: React + Vite + TypeScript + Tailwind + shadcn/ui (semuanya berlisensi MIT). Dipilih karena tim sudah familiar dari UI upstream, bukan karena kodenya disalin. Base path Vite `/app/`.
- **Build:** hasil `web/dist` disalin ke `src/ui/webapp/dist`, lalu di-embed dengan `//go:embed dist`. Di repo hanya ada placeholder `dist/index.html` ("UI belum di-build"), supaya `go build` dan `go test` tetap jalan tanpa Node.
- **Routing:** `/app/*` mengembalikan `index.html` (SPA fallback). Aset statis dilayani dengan hash dan cache panjang. Route dipasang setelah middleware auth yang sama dengan `/`.
- **Docker:** tambah stage `node:22-alpine` di `docker/golang.Dockerfile` (`npm ci && npm run build`), lalu `COPY` hasilnya ke stage Go sebelum `go build`.
- **CI:** `deploy.yml` sudah menjalankan `go build` dan `go test` tanpa Node, jadi placeholder sudah cukup. Tambahkan job `web` (lint, typecheck, vitest) mengikuti pola `integrations-page-ci.yml`.

## 4. Fase

### Fase 1: Fondasi dan branding (perkiraan 3–5 hari kerja)
- Scaffold `web/` dengan app shell: sidebar, pemilih device, tema, bahasa Indonesia, logo dan warna Ruang Sanad.
- Embed dan route `/app` di Go, tambah stage Node di Dockerfile, perbarui `AGENTS.md` (hapus referensi `src/views`).
- Endpoint `GET /api/app/me`: username, peran (`admin`/`cs`), dan device yang diizinkan, diturunkan dari `isRestrictedAccount` dan scope device.
- Halaman awal: status device (memakai endpoint yang sudah ada).
- **Selesai bila:** `/app` tampil di server, login admin dan akun CS berfungsi, akun CS hanya melihat device-nya, `go test ./...` lulus, image Docker ter-build.

### Fase 2: Analisis chat CS (perkiraan 4–6 hari)
Endpoint `GET /api/app/analytics/*` (query SQLite read-only, wajib scope `device_id`):
- Volume pesan masuk/keluar per hari dan per jam (heatmap).
- **Waktu respons pertama:** untuk setiap pesan masuk yang memulai "giliran" pelanggan, hitung jarak ke pesan `is_from_me` berikutnya di chat yang sama. Tampilkan median dan p90, bukan rata-rata, karena distribusinya miring.
- Daftar chat belum dibalas: pesan terakhir dari pelanggan dan umurnya.
- Chat baru per periode (kontak yang pertama kali chat).

**Keterbatasan data (penting):**
1. `is_from_me` tidak membedakan **siapa** yang membalas: CS manusia lewat HP, CS lewat UI, atau AI `hermes-cs-bridge`. Analisis per agen dan per AI-vs-manusia **tidak mungkin** dengan data saat ini. Solusinya ada di fase 3 (atribusi kirim).
2. Data hanya selengkap riwayat yang tersinkron ke SQLite. Chat lama sebelum device login bisa tidak ada.
3. Jam kerja dan libur belum diperhitungkan. Waktu respons di malam hari akan terlihat buruk. Perlu konfigurasi jam layanan.
4. Query agregat di SQLite produksi: perlu batas rentang tanggal (mis. maks. 90 hari) dan diuji pada salinan database dengan volume nyata sebelum rilis.

### Fase 3: Fitur operasional (perkiraan 1–2 minggu, tergantung lingkup)
- **Inbox CS:** daftar chat, tampilan pesan, dan kirim teks/media (memakai `/chat/*` dan `/send/*`), dengan pembaruan realtime lewat websocket yang sudah ada.
- **Atribusi kirim:** tabel baru `app_send_log(message_id, device_id, username, source, created_at)`. Setiap kirim dari `/app` mencatat username. Bridge AI juga diberi penanda `source=ai`. Ini membuka analisis per agen.
  - Perubahan repository harus mencapai 3 lapisan (`domains/chatstorage/interfaces.go`, `sqlite_repository.go`, `chatstorage_wrapper.go`) sesuai `AGENTS.md`.
- **Template balasan:** CRUD template per device.
- **Label atau penugasan chat** (opsional, perlu dirinci).
- **Broadcast:** tidak saya rekomendasikan sebagai fitur bebas (risiko nomor diblokir WhatsApp). Kalau tetap dibutuhkan, batasi ke kontak yang sudah pernah berinteraksi, dengan rate limit yang sama (30/menit).
- **Akses:** endpoint baru untuk akun CS harus ditambahkan secara eksplisit ke allowlist akun terbatas. Fitur admin (device, API key, webhook) tetap tertutup untuk CS.

### Fase 4: Panel lead/konversi (bergantung pada Dashboard Ruang Sanad)
- **Prinsip:** GOWA **menampilkan** data dari Dashboard, tidak menghitung ulang funnel. Satu sumber data mencegah angka yang berbeda di dua tempat.
- **Prasyarat yang belum ada:** read API di Dashboard (mis. `GET /integrations/gowa/leads?phone=` dan ringkasan funnel) dengan token server-to-server. GOWA memanggilnya dari backend (proxy di `/api/app/leads/*`), jadi token tidak pernah sampai ke browser.
- Fitur di inbox: panel samping "info lead" (tahap, label, PIC) untuk nomor chat yang sedang dibuka. Di halaman analisis: ringkasan funnel dari Dashboard.
- Kalau read API tidak bisa dibuat, fase ini ditunda. Menyalin database Dashboard ke GOWA **tidak** disarankan.

## 5. Risiko dan trade-off

| Risiko | Dampak | Mitigasi |
|---|---|---|
| Dua UI (`/` dan `/app`) | Pengguna bingung, ada fitur ganda | `/app` jadi pintu utama CS. `/` hanya untuk admin (QR, device). Tambah tautan di `/app` ke `/`. |
| Endpoint upstream berubah saat merge dari `aldinokemal` | `/app` rusak | `/app` memakai endpoint lama lewat satu lapisan klien API. Endpoint `/api/app/*` milik sendiri lebih stabil. Ada test kontrak. |
| Konflik merge dengan upstream backend | Biaya maintenance | Kode baru di file dan folder baru (`cmd/webapp*.go`, `ui/webapp/`, `web/`), minim mengubah file upstream. |
| Image Docker lebih besar atau build lebih lama | Deploy lebih lambat | Multi-stage build: Node hanya di stage build, image akhir tidak berubah kecuali beberapa ratus KB aset. |
| Query analisis membebani SQLite | Pengiriman melambat | Batas rentang, index tambahan bila perlu, timeout per query, uji pada salinan data. |
| Lingkup fase 3 melebar | Molor | Rinci fitur operasional sebelum fase 3 dimulai. |

## 6. Hal yang masih perlu data atau keputusan

Sudah dijawab (2026-11-21):
- Jam layanan CS: 08:00–15:00 dan 19:00–21:00. Asumsi: WIB (`Asia/Jakarta`) dan berlaku setiap hari; bisa diubah lewat konfigurasi.
- Jumlah CS: 1 orang. Analisis per agen tidak diperlukan; atribusi cukup membedakan `app` / `ai` / `phone`.

Masih terbuka:
1. Target SLA waktu respons, bila ada.
2. Daftar fitur operasional prioritas di fase 3 (inbox, template, label).
3. Apakah Dashboard Ruang Sanad bisa menyediakan read API (fase 4), dan siapa yang mengerjakannya.
4. Logo dan warna resmi Ruang Sanad (fase 1).
5. Opsional: minta lisensi ke `aldinokemal/gowa-ui`. Bila diberikan, `/app` bisa menggantikan `/` sepenuhnya.

Todo implementasi: `docs/plans/2026-11-21-ui-app-implementation.md`.

## 7. Verifikasi per fase
- `go test ./...`, `go vet ./...`, `go build` di `src/`.
- `npm run lint && npm run typecheck && npm test && npm run build` di `web/`.
- Test Go untuk endpoint baru: scope device (akun CS tidak bisa membaca device lain) dan perhitungan waktu respons, memakai SQLite sementara.
- Uji manual `/app` di server staging atau produksi, oleh admin dan akun CS.
