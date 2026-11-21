# UI `/app` Ruang Sanad — Implementation Plan (Todo)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Menyajikan SPA milik Ruang Sanad di `/app` (branding + analisis chat CS), di-embed ke binary GOWA, dengan akses admin dan CS.

**Architecture:** React SPA di `web/` di-build Vite ke `src/ui/webapp/dist`, lalu di-embed dengan `go:embed` dan dilayani di `{base}/app/*`. Endpoint analisis baru berada di `/api/app/*`, mengikuti lapisan domain → infrastructure (query SQLite read-only) → usecase → ui/rest, sehingga `IChatStorageRepository` tidak berubah. Perhitungan waktu respons dilakukan di Go dengan fungsi murni yang menghitung hanya menit di dalam jam layanan.

**Tech Stack:** Go 1.26, Fiber v3, SQLite (`database/sql`); React 19, Vite, TypeScript, Tailwind 4, shadcn/ui, TanStack Query, Vitest.

**Spec:** `docs/plans/2026-11-21-ui-app-ruang-sanad.md`

## Global Constraints

- Jangan menyalin kode, komponen, atau aset dari `aldinokemal/gowa-ui` (tidak berlisensi). Semua file di `web/` ditulis baru.
- UI upstream di `/` dan mekanisme `uiasset` **tidak diubah**.
- Jam layanan CS default: `08:00-15:00` dan `19:00-21:00`, zona `Asia/Jakarta`, berlaku setiap hari. Bisa diubah lewat env `CS_SERVICE_HOURS` dan `CS_SERVICE_TZ`.
- CS hanya 1 orang. Tidak ada analisis per agen. Atribusi hanya membedakan sumber kirim: `app` / `ai` / `phone`.
- Setiap data chat yang dibaca wajib dibatasi pada `device_id` penyimpanan (`deviceIDFromContext` di `src/usecase/chat.go:263`).
- Rentang analisis maksimum 90 hari; default 7 hari.
- Chat grup (`@g.us`), `status@broadcast`, dan newsletter (`@newsletter`) dikecualikan dari analisis.
- Kode Go baru ditaruh di file baru bila memungkinkan, supaya merge dari upstream tetap mudah.
- Perintah Go dijalankan dari `src/`. Perintah frontend dijalankan dari `web/`.

## Review Focus

1. **Pesan masuk di luar jam layanan** (mis. pukul 22:00, dibalas pukul 08:05 esoknya). Waktu respons yang diharapkan adalah 5 menit, bukan 10 jam. Diuji di Task 5 dan Task 6.
2. **Akun CS meminta device lain** lewat `X-Device-Id`. Yang diharapkan adalah 403 `DEVICE_NOT_ALLOWED`, bukan data device lain. Diuji di Task 3.
3. **Rentang tanggal terbalik atau lebih dari 90 hari.** Yang diharapkan adalah 400 dengan pesan jelas, bukan query berat. Diuji di Task 7.
4. **Chat yang pesan pertamanya dari kita** (outbound duluan, pelanggan membalas, lalu kita membalas lagi). Hanya giliran pelanggan yang dihitung. Diuji di Task 6.
5. **Deep link `/app/analytics` dimuat ulang di browser.** Yang diharapkan adalah `index.html` (SPA fallback), bukan 404. Diuji di Task 2.

---

## FASE 1 — Fondasi dan branding

### Task 1: Scaffold SPA `web/` dan pipeline build

**Files:**
- Create: `web/package.json`, `web/vite.config.ts` (`base: '/app/'`, `build.outDir: '../src/ui/webapp/dist'`, `emptyOutDir: true`), `web/tsconfig*.json`, `web/index.html`, `web/src/main.tsx`, `web/src/App.tsx`, `web/src/lib/api.ts`
- Create: `src/ui/webapp/dist/.gitkeep` (supaya `go:embed` selalu punya direktori; `go build` tanpa Node tetap jalan)
- Modify: `.gitignore`: tambah `src/ui/webapp/dist/*` dan `!src/ui/webapp/dist/.gitkeep`, sehingga hasil build tidak pernah ter-commit.
- Modify: `docker/golang.Dockerfile`: stage `FROM node:22-alpine AS web` (`npm ci && npm run build`), lalu `COPY --from=web /web-dist ./ui/webapp/dist` di stage builder sebelum `go build`.

**Interfaces:**
- Produces: `api.ts` mengekspor `apiGet<T>(path: string, deviceId?: string): Promise<T>`. Path relatif terhadap base server (dihitung dari `import.meta.env.BASE_URL` dengan membuang akhiran `app/`), dengan `credentials: 'same-origin'` dan header `X-Device-Id` bila ada.

- [ ] **Step 1:** Scaffold Vite React-TS. Pasang Tailwind 4 dan shadcn/ui (init baru, bukan salinan). Gunakan React Router dengan `basename` dari `import.meta.env.BASE_URL`.
- [ ] **Step 2:** Tulis test Vitest `api.test.ts`. Kasus: `apiGet('/chats','dev1')` memanggil `fetch` dengan URL `/chats` (BASE_URL `/app/`) atau `/wa/chats` (BASE_URL `/wa/app/`) dan header `X-Device-Id: dev1`.
- [ ] **Step 3:** Jalankan `npm test`. Harus FAIL dulu, lalu implementasikan `api.ts`, lalu PASS.
- [ ] **Step 4:** Jalankan `npm run build`. Expected: `src/ui/webapp/dist/index.html` berisi referensi aset `/app/assets/...`.
- [ ] **Step 5:** Jalankan `docker build -f docker/golang.Dockerfile .` dari root. Expected: sukses.
- [ ] **Step 6:** Commit `feat(web): scaffold /app SPA and docker build stage`.

### Task 2: Sajikan `/app` dari Go (embed + SPA fallback)

**Files:**
- Create: `src/ui/webapp/embed.go` (`package webapp`; `//go:embed all:dist`; `func Register(router fiber.Router)`)
- Create: `src/ui/webapp/embed_test.go`
- Modify: `src/cmd/rest.go`: panggil `webapp.Register(apiGroup)` setelah `registerCSAIProxy(apiGroup)` dan **sebelum** `headerDeviceGroup`. `TestDeviceGroupIsRegisteredLast` harus tetap lulus.
- Modify: `src/config/settings.go`: `AppWebAppEnabled = true`. `src/cmd/root.go`: env `APP_WEBAPP_ENABLED` dan flag `--webapp`. `src/.env.example`: dokumentasikan.

**Interfaces:**
- Produces: `webapp.Register(router fiber.Router)` melayani `GET /app`, `GET /app/*`. Bila `index.html` tidak ada di FS (belum di-build), yang dikirim adalah HTML placeholder dari konstanta Go: "UI /app belum di-build. Jalankan `npm run build` di `web/`." Path yang cocok dengan file di `dist` dilayani apa adanya: `assets/*` dengan `Cache-Control: public, max-age=31536000, immutable`, selain itu `index.html` dengan `Cache-Control: no-cache`. Header keamanan: `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`.

- [ ] **Step 1:** Tulis test `TestWebAppServesIndexForDeepLink` (GET `/app/analytics` → 200, `Content-Type` text/html), `TestWebAppServesAssetWithLongCache` (memakai `fstest.MapFS` lewat fungsi internal `register(router, fsys fs.FS)`), `TestWebAppRejectsPathTraversal` (GET `/app/../go.mod` → bukan isi file), dan `TestWebAppPlaceholderWhenNotBuilt` (FS kosong → 200 berisi teks placeholder).
- [ ] **Step 2:** Jalankan `go test ./ui/webapp/...`. Expected: FAIL.
- [ ] **Step 3:** Implementasikan `Register` dan `register`. Gunakan `fs.Sub(distFS, "dist")` dan `path.Clean`.
- [ ] **Step 4:** Jalankan `go test ./ui/webapp/... ./cmd/...`. Expected: PASS, termasuk `TestDeviceGroupIsRegisteredLast`.
- [ ] **Step 5:** Commit `feat(webapp): embed and serve /app SPA`.

### Task 3: Endpoint identitas `/api/app/me` dan akses akun CS

**Files:**
- Create: `src/cmd/webapp_api.go`: `func registerWebAppAPI(router fiber.Router)`
- Create: `src/cmd/webapp_api_test.go`
- Modify: `src/cmd/device_scope.go:92` `scopedRoute`: izinkan `GET /app`, `GET /app/*`, `GET /api/app/me`, dan `GET /api/app/analytics/*` untuk akun terbatas. `/api/app/analytics/*` mengikuti pemeriksaan device lewat header, sama seperti `/chats`.
- Modify: `src/cmd/rest.go`: panggil `registerWebAppAPI(apiGroup)` di samping `webapp.Register`.

**Interfaces:**
- Produces: `GET /api/app/me` → `{"username": string, "role": "admin"|"cs", "devices": []string}`. `role` = `"cs"` bila `isRestrictedAccount(username)`. `devices` = `config.AppBasicAuthDevices[username]` untuk CS, atau `[]` untuk admin (artinya semua device; UI mengambilnya dari `/devices`).

- [ ] **Step 1:** Tulis test `TestWebAppMeReturnsRoleForRestrictedAccount` (akun `cs` ber-scope `dev1` → `role=cs`, `devices=["dev1"]`), `TestWebAppMeAdmin`, dan di `device_scope_test.go` tambah `TestDeviceScopeAllowsWebAppForRestricted` (GET `/app/x` dan `/api/app/me` → lolos) serta `TestDeviceScopeBlocksOtherDeviceAnalytics` (GET `/api/app/analytics/summary` dengan `X-Device-Id: dev2` → 403 `DEVICE_NOT_ALLOWED`).
- [ ] **Step 2:** Jalankan `go test ./cmd -run 'WebApp|DeviceScope'`. Expected: FAIL.
- [ ] **Step 3:** Implementasikan. Test harus serial dan memulihkan `config.AppBasicAuthDevices` (pola yang sama dengan test yang ada).
- [ ] **Step 4:** Jalankan `go test ./cmd/...`. Expected: PASS.
- [ ] **Step 5:** Commit `feat(webapp): identity endpoint and CS account access`.

### Task 4: App shell, branding, halaman status

**Files:**
- Create: `web/src/components/layout/app-shell.tsx`, `device-picker.tsx`, `web/src/pages/home.tsx`, `web/src/pages/not-found.tsx`, `web/src/i18n/id.ts` (semua teks UI berbahasa Indonesia), `web/src/assets/logo-ruang-sanad.svg` (aset dari tim Ruang Sanad)
- Modify: `AGENTS.md` root: ganti paragraf "Embedded browser UI ... `src/views`" dengan deskripsi `/` (gowa-ui runtime) dan `/app` (`web/` dan `src/ui/webapp`). Tambah `web/AGENTS.md` singkat berisi perintah build/test dan larangan menyalin gowa-ui.

- [ ] **Step 1:** Shell: sidebar (Beranda, Analisis, tautan "Kelola device" ke `/`). Menu admin disembunyikan bila `role=cs`. Pemilih device: CS dengan 1 device tidak melihat pemilih. Pilihan device disimpan di `localStorage` (dibungkus try/catch).
- [ ] **Step 2:** Beranda: status device dari `GET /devices/{id}/status`.
- [ ] **Step 3:** Test Vitest `app-shell.test.tsx`: role `cs` tidak merender menu admin; role `admin` merendernya.
- [ ] **Step 4:** Jalankan `npm run lint && npm run typecheck && npm test && npm run build`. Expected: semua lulus.
- [ ] **Step 5:** Verifikasi manual: `go run . rest` dengan `APP_BASIC_AUTH=admin:x,cs:y` dan `APP_BASIC_AUTH_DEVICES=cs:<device>`, buka `/app` sebagai keduanya.
- [ ] **Step 6:** Commit `feat(web): app shell, branding, device status`.

> **Logo dan warna resmi Ruang Sanad dibutuhkan dari user sebelum Task 4.**

---

## FASE 2 — Analisis chat CS

### Task 5: Konfigurasi jam layanan dan fungsi waktu respons (Go murni)

**Files:**
- Create: `src/domains/analytics/analytics.go`: DTO dan interface
- Create: `src/pkg/servicehours/servicehours.go` + `servicehours_test.go`
- Modify: `src/config/settings.go`: `CSServiceHoursRaw = "08:00-15:00,19:00-21:00"`, `CSServiceTZ = "Asia/Jakarta"`. `src/cmd/root.go`: env `CS_SERVICE_HOURS`, `CS_SERVICE_TZ`. `src/.env.example`.

**Interfaces:**
- Produces (`package servicehours`):
  - `type Window struct{ Start, End time.Duration }` (offset dari tengah malam)
  - `type Schedule struct{ Windows []Window; Loc *time.Location }`
  - `func Parse(raw, tz string) (Schedule, error)`: tolak jendela tumpang tindih, `Start >= End`, format bukan `HH:MM-HH:MM`.
  - `func (s Schedule) Elapsed(from, to time.Time) time.Duration`: jumlah durasi di dalam jendela layanan antara `from` dan `to`. Hasilnya 0 bila `to <= from`. Algoritma: iterasi per hari kalender di `s.Loc` dari `from` sampai `to`; untuk setiap jendela hari itu tambahkan `max(0, min(to, winEnd) - max(from, winStart))`.
  - `func (s Schedule) InService(t time.Time) bool`

- [ ] **Step 1:** Tulis test (semua dalam WIB):
  - `TestElapsedInsideWindow`: 09:00 → 09:10 = 10m
  - `TestElapsedAcrossGap`: 14:50 → 19:10 = 20m
  - `TestElapsedOvernight`: 22:00 → 08:05 esok = 5m
  - `TestElapsedBeforeOpen`: 07:00 → 08:30 = 30m
  - `TestElapsedMultiDay`: Senin 20:00 → Rabu 08:00 = 1h (Senin) + 9h (Selasa) = 10h
  - `TestParseRejectsOverlap`: `"08:00-15:00,14:00-16:00"` → error
  - `TestParseRejectsBadFormat`: `"8-15"` → error
- [ ] **Step 2:** Jalankan `go test ./pkg/servicehours/...`. Expected: FAIL.
- [ ] **Step 3:** Implementasikan.
- [ ] **Step 4:** Jalankan lagi. Expected: PASS.
- [ ] **Step 5:** Commit `feat(analytics): CS service-hours schedule`.

### Task 6: Perhitungan giliran dan waktu respons (fungsi murni)

**Files:**
- Create: `src/usecase/analytics_compute.go` + `analytics_compute_test.go`

**Interfaces:**
- Consumes: `servicehours.Schedule.Elapsed`
- Produces (di `domains/analytics`):
  - `type MessageRow struct{ ChatJID string; Timestamp time.Time; IsFromMe bool }`
  - `type Turn struct{ ChatJID string; AskedAt time.Time; RepliedAt *time.Time; Response time.Duration }` (`Response` = durasi dalam jam layanan; 0 bila belum dibalas)
- Produces (di `usecase`): `func computeTurns(rows []MessageRow, s servicehours.Schedule) []Turn`. `rows` sudah terurut per `(ChatJID, Timestamp)`. Giliran dimulai pada pesan masuk pertama setelah pesan keluar (atau di awal chat) dan berakhir pada pesan keluar berikutnya. Pesan masuk beruntun tergabung dalam satu giliran.
- Produces: `func summarize(turns []Turn, now time.Time, s servicehours.Schedule) analytics.Summary` dengan `Summary{ Inbound, Outbound int; Turns, Answered int; MedianResponse, P90Response time.Duration; Unanswered []UnansweredChat }`. `UnansweredChat{ChatJID string; WaitingSince time.Time; WaitingService time.Duration}` diurutkan dari yang paling lama. Persentil dihitung dengan metode nearest-rank.

- [ ] **Step 1:** Tulis test:
  - `TestTurnsGroupConsecutiveInbound`: masuk 09:00, masuk 09:02, keluar 09:10 → 1 giliran, respons 10m
  - `TestTurnsIgnoreLeadingOutbound`: keluar 09:00, masuk 09:05, keluar 09:06 → 1 giliran, 1m
  - `TestTurnsOvernightUsesServiceTime`: masuk 22:00, keluar 08:05 → 5m
  - `TestUnansweredLastInbound`: masuk 10:00 tanpa balasan, now=10:30 → `Unanswered[0].WaitingService=30m`
  - `TestSummaryPercentiles`: respons 1,2,3,...,10 menit → median 5m, p90 9m (nearest-rank)
- [ ] **Step 2:** Jalankan `go test ./usecase -run 'Turns|Summary|Unanswered'`. Expected: FAIL.
- [ ] **Step 3:** Implementasikan.
- [ ] **Step 4:** Jalankan. Expected: PASS.
- [ ] **Step 5:** Commit `feat(analytics): turn and response-time computation`.

### Task 7: Query SQLite analisis + usecase

**Files:**
- Create: `src/infrastructure/analytics/sqlite.go` + `sqlite_test.go` (SQLite sementara, skema dibuat lewat `chatstorage.NewStorageRepository(db).InitializeSchema()`)
- Create: `src/usecase/analytics.go`
- Modify: `src/cmd/root.go`: buat `analyticsUsecase = usecase.NewAnalyticsService(analytics.NewSQLiteReader(chatStorageDB), schedule)` setelah `chatStorageRepo`.

**Interfaces:**
- Produces (`domains/analytics`):
  - `type Reader interface { MessagesInRange(ctx context.Context, deviceID string, from, to time.Time) ([]MessageRow, error); HourlyVolume(ctx context.Context, deviceID string, from, to time.Time) ([]HourBucket, error) }`
  - `type HourBucket struct{ Day string /* YYYY-MM-DD di zona layanan */; Hour int; Inbound, Outbound int }`
  - `type IAnalyticsUsecase interface { Summary(ctx context.Context, req RangeRequest) (Summary, error); Volume(ctx context.Context, req RangeRequest) ([]HourBucket, error) }`
  - `type RangeRequest struct{ From, To time.Time }`
- Query `MessagesInRange`: `SELECT chat_jid, timestamp, is_from_me FROM messages WHERE device_id=? AND timestamp>=? AND timestamp<? AND chat_jid NOT LIKE '%@g.us' AND chat_jid NOT LIKE '%@newsletter' AND chat_jid <> 'status@broadcast' ORDER BY chat_jid, timestamp`. Supaya giliran yang mulai sebelum `from` tidak salah hitung, ambil juga 1 pesan terakhir per chat sebelum `from` (subquery) sebagai konteks; konteks ini tidak ikut dihitung di Inbound/Outbound.
- Bucket jam dihitung di Go (konversi ke `CS_SERVICE_TZ`), bukan dengan `strftime` SQLite, supaya format timestamp tersimpan tidak menjadi masalah.
- Usecase memvalidasi `To > From` dan `To-From <= 90*24h`; bila gagal, kembalikan `pkgError.ValidationError`. Device diambil dari `deviceIDFromContext(ctx)`.

- [ ] **Step 1:** Tulis test infrastruktur: `TestMessagesInRangeScopedToDevice` (pesan device lain tidak muncul), `TestMessagesInRangeExcludesGroups`, `TestMessagesInRangeIncludesPriorContext`. Test usecase: `TestAnalyticsRejectsInvertedRange`, `TestAnalyticsRejectsOver90Days`.
- [ ] **Step 2:** Jalankan `go test ./infrastructure/analytics/... ./usecase -run Analytics`. Expected: FAIL.
- [ ] **Step 3:** Implementasikan.
- [ ] **Step 4:** Jalankan. Expected: PASS.
- [ ] **Step 5:** Ukur performa: jalankan `MessagesInRange` 90 hari pada **salinan** `storages/` produksi (bukan file live). Catat durasinya. Bila lebih dari 1 detik, tambah index `(device_id, timestamp)` lewat migrasi baru di `sqlite_repository.go` sesuai `src/infrastructure/chatstorage/AGENTS.md`.
- [ ] **Step 6:** Commit `feat(analytics): sqlite reader and usecase`.

### Task 8: REST `/api/app/analytics/*`

**Files:**
- Create: `src/ui/rest/analytics.go` + `analytics_test.go`
- Modify: `src/cmd/rest.go`: `rest.InitRestAnalytics(r, analyticsUsecase)` di dalam `registerDeviceScopedRoutes` (butuh `DeviceMiddleware`).

**Interfaces:**
- Produces:
  - `GET /api/app/analytics/summary?from=YYYY-MM-DD&to=YYYY-MM-DD` → `utils.ResponseData{Results: Summary}`. Durasi dikirim dalam **detik** (`median_response_s`, `p90_response_s`, `waiting_service_s`).
  - `GET /api/app/analytics/volume?from=&to=` → `[]HourBucket`
  - `from`/`to` ditafsirkan sebagai tanggal di `CS_SERVICE_TZ`, dengan `to` inklusif. Default: 7 hari terakhir.
  - Juga mengembalikan `service_hours: "08:00-15:00,19:00-21:00"` agar UI bisa menampilkannya.

- [ ] **Step 1:** Tulis test dengan stub usecase: `TestAnalyticsSummaryParsesDates`, `TestAnalyticsSummaryBadDate` (→ 400), `TestAnalyticsDefaultsToLast7Days`.
- [ ] **Step 2:** Jalankan `go test ./ui/rest -run Analytics`. Expected: FAIL.
- [ ] **Step 3:** Implementasikan.
- [ ] **Step 4:** Jalankan `go test ./...` dan `go vet ./...`. Expected: PASS.
- [ ] **Step 5:** Commit `feat(rest): analytics endpoints for /app`.

### Task 9: Halaman Analisis di `/app`

**Files:**
- Create: `web/src/pages/analytics.tsx`, `web/src/features/analytics/{kpi-row,volume-heatmap,unanswered-list,range-picker}.tsx`, `web/src/features/analytics/format.ts` + `format.test.ts`

- [ ] **Step 1:** Test `format.test.ts`: `formatDuration(65)` → `"1m 5d"`; `formatDuration(3700)` → `"1j 1m"`; `formatDuration(0)` → `"0d"`.
- [ ] **Step 2:** KPI: pesan masuk, pesan keluar, giliran terjawab/total, median dan p90 waktu respons (label: "dalam jam layanan 08–15 & 19–21 WIB").
- [ ] **Step 3:** Heatmap volume hari × jam, dengan jam layanan ditandai.
- [ ] **Step 4:** Daftar chat belum dibalas, diurutkan dari yang menunggu paling lama. Klik membuka `/` (sementara; inbox `/app` baru ada di fase 3).
- [ ] **Step 5:** Catatan keterbatasan tampil di halaman: "Balasan AI dan balasan dari HP ikut dihitung sebagai balasan CS sampai fitur atribusi aktif."
- [ ] **Step 6:** Jalankan `npm run lint && npm run typecheck && npm test && npm run build`, lalu verifikasi manual dengan data nyata (salinan database).
- [ ] **Step 7:** Commit `feat(web): CS analytics page`.

---

## FASE 3 — Operasional (todo kasar; dirinci menjadi plan terpisah sebelum dimulai)

- [ ] **T10 Atribusi kirim:** tabel `app_send_log(message_id, device_id, source /* app|ai|phone */, created_at)`. Kirim dari `/app` → `app`; `hermes-cs-bridge` → `ai` (lewat header atau field baru, perlu desain); selebihnya `phone`. Perubahan repository melewati 3 lapisan (`domains/chatstorage/interfaces.go`, `sqlite_repository.go`, `chatstorage_wrapper.go`). Analisis memisahkan waktu respons manusia vs AI.
- [ ] **T11 Inbox CS di `/app`:** daftar chat, tampilan pesan, kirim teks/media, realtime via websocket yang sudah ada.
- [ ] **T12 Template balasan** per device (CRUD + sisip ke kotak kirim).
- [ ] **T13 Label chat** (opsional; tentukan apakah memakai label WhatsApp Business atau label lokal).
- **Perlu keputusan user:** urutan prioritas T11–T13, dan apakah broadcast benar-benar dibutuhkan (default: tidak dibuat).

## FASE 4 — Panel lead (bergantung pada Dashboard Ruang Sanad)

- [ ] **T14 Read API di Dashboard Ruang Sanad** (di luar repo ini): `GET lead by phone` + ringkasan funnel, dengan token server-to-server.
- [ ] **T15 Proxy `/api/app/leads/*` di GOWA:** token dari env, tidak pernah dikirim ke browser, timeout dan cache singkat.
- [ ] **T16 Panel "Info lead" di inbox + ringkasan funnel di halaman Analisis.**
- **Perlu keputusan user:** siapa yang mengerjakan T14.
