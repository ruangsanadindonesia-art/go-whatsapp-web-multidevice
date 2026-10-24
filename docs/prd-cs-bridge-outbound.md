# PRD — Bridge CS AI: kirim pesan outbound dari Dashboard (follow-up & konfirmasi)

| | |
|---|---|
| Status | Siap diimplementasikan |
| Repo | `ruangsanadindonesia-art/go-whatsapp-web-multidevice`, folder `integrations/hermes-cs-bridge/` |
| Pasangan | Dashboard `dinastran/ruang-sanad` Fase D (sudah merge): `app/services/cs_bridge.go`, `app/services/wa_outbox.go` |
| Dasar | PRD Dashboard `docs/prd/prd-crm-leads-cs-ai.md` bagian 4.3, 10, 11.3, Lampiran B.3; kontrak `docs/panduan/bridge-outbound-api.md` |
| Pembaca | Claude Code / developer yang mengimplementasikan |
| Tanggal | Oktober 2026 |

> **Cara memakai dokumen ini:**
> 1. Baca `AGENTS.md` repo ini. Semua perubahan ada di `integrations/hermes-cs-bridge/` (modul Go sendiri, `package main`), ditambah dokumentasi di `docs/cs-ai-hermes.md`. Jangan menyentuh `src/`.
> 2. Kerjakan sebagai **satu PR**. Jalankan `go test ./...` dan `go vet ./...` di folder bridge sebelum PR.
> 3. Bagian bertanda 🔒 adalah aturan keamanan yang tidak boleh dilonggarkan.
> 4. Jangan mengubah kontrak request/response di bagian 4 tanpa mengubah Dashboard juga.

---

## 1. Latar belakang

Dashboard sekarang punya antrean follow-up yang disetujui manusia (Script 30 Hari) dan konfirmasi pembayaran. Semua pesan ke calon santri **wajib keluar dari nomor CS lewat bridge**, bukan langsung dari GOWA (keputusan D6). Alasannya:

1. Hermes menyimpan riwayat percakapan per sesi (`wa-cs-<nomor>`). Pesan yang dikirim Dashboard langsung lewat GOWA tidak masuk ke riwayat itu, sehingga saat lead membalas, CS AI tidak tahu konteksnya.
2. Akun GOWA `dashboard` sengaja **tidak** punya akses ke device `cs-ai`. Isolasi nomor tetap terjaga.
3. Satu pengirim per nomor membuat jeda dan anti-blokir bisa dikendalikan.

Yang **sudah ada** di bridge, dan tidak perlu dibuat ulang:
- Pengirim event ke Dashboard: `dashboard.go` (`DashboardOutbox`, `emit`, `leadFor`, `eventIDFor`).
- Penanda pesan milik sendiri: `rememberSent` / `isOwnSent`. Pesan yang dikirim bridge tidak dianggap "admin membalas dari HP".
- Jeda chat: `Store.PausedUntil` / `SetPause` (`state.go`).
- Prompt sistem per giliran: `buildSystemPrompt` (`prompt.go`), dipanggil di `flush` (`bridge.go`).

Yang **belum ada**: endpoint untuk menerima perintah kirim dari Dashboard, dan konteks "pesan terakhir dari admin/Dashboard" di prompt.

## 2. Tujuan dan non-tujuan

**Tujuan**
1. Endpoint `POST /api/outbound` yang mengirim teks ke chat lead lewat GOWA device `cs-ai`.
2. Pesan outbound tercatat di Dashboard sebagai `message.out` (`sender=followup`), lengkap dengan `reference_id` dan `purpose`.
3. Saat lead membalas, CS AI tahu isi dan waktu pesan outbound terakhir.
4. Aman: hanya Dashboard (token terpisah, alamat internal) yang bisa memanggil, dan tidak ada kiriman ganda saat Dashboard mengulang.

**Non-tujuan**
- Penjadwalan, jam kirim, cap harian, dan aturan Script 30 Hari. Semuanya sudah ditangani Dashboard. Bridge **tidak** mengulang logika itu.
- Kirim gambar/dokumen. Outbound hanya teks.
- Broadcast atau kirim ke nomor yang belum pernah chat.

## 3. Arsitektur

```
Dashboard worker (tiap 1 menit, maks 1 pesan)
   │  POST http://172.17.0.1:8650/api/outbound   (Bearer OUTBOUND_TOKEN)
   ▼
hermes-cs-bridge ──► GOWA /send/message (device cs-ai) ──► WhatsApp lead
   │  rememberSent(id)          state.json: LastOutbound[chat]
   │  emit message.out (sender=followup, reference_id, purpose)
   ▼
DashboardOutbox (antrean JSONL yang sudah ada) ──► Dashboard /integrations/cs-ai/events

Lead membalas ──► webhook ──► flush ──► buildSystemPrompt(+ "Pesan terakhir dari admin: …") ──► Hermes
```

Topologi: bridge memakai jaringan host dan listen di `LISTEN_ADDR` (produksi `172.17.0.1:8650`). Dashboard berjalan di host dan memanggil alamat itu langsung.
🔒 Proxy GOWA hanya meneruskan `{AppBasePath}/cs-ai` dan `/cs-ai/*` ke bridge (`src/cmd/csai_proxy.go`), sehingga route ini **wajib** di luar prefix `/cs-ai` supaya tidak terjangkau dari `wa.ruangsanad.com`.

## 4. Kontrak API (jangan diubah)

### 4.1 Request

```
POST /api/outbound
Authorization: Bearer <OUTBOUND_TOKEN>
Content-Type: application/json

{"chat_id":"6281234567890@s.whatsapp.net","text":"Assalamu'alaikum Kak …","reference_id":123,"purpose":"followup"}
```

| Field | Wajib | Aturan |
|---|---|---|
| `chat_id` | ya | JID chat lead, sama dengan `chat_id` di event ingest. Berakhiran `@s.whatsapp.net` atau `@lid` |
| `text` | ya | 1–1000 karakter (rune) setelah trim. Teks final yang sudah disetujui manusia |
| `reference_id` | tidak | `lead_followups.id` untuk `purpose=followup`; `0` untuk tujuan lain |
| `purpose` | ya | `followup` atau `payment_verified`. Nilai lain → 400 |

Body maksimal 16 KB. Field yang tidak dikenal diabaikan.

### 4.2 Response

| Kondisi | Status | Body |
|---|---|---|
| Terkirim | `200` | `{"wa_message_id":"3EB0…"}` |
| Pengulangan request yang sudah terkirim (6.4) | `200` | `{"wa_message_id":"<id lama>","duplicate":true}` |
| Fitur mati (`OUTBOUND_TOKEN` kosong) | `404` | `{"error":"disabled"}` |
| Token salah / tidak ada | `401` | `{"error":"unauthorized"}` |
| JSON rusak, field tidak valid, chat grup/status/admin | `400` | `{"error":"invalid: <alasan>"}` |
| Teks memuat `[[` | `400` | `{"error":"marker"}` |
| Chat sedang dijeda (admin menangani) | `409` | `{"error":"paused"}` |
| Lead baru saja menulis / giliran AI sedang berjalan (6.3) | `409` | `{"error":"busy"}` |
| GOWA gagal / timeout | `502` | `{"error":"send failed"}` |

**Cara Dashboard menafsirkan** (`app/services/cs_bridge.go`, sudah jadi): 2xx berarti terkirim. 409, 429, 5xx, timeout, dan gagal koneksi dianggap sementara, jadi dicoba lagi setelah 1 dan 5 menit; gagal ke-3 masuk "Gagal kirim". 400, 401, 403, 404, dan lainnya dianggap permanen dan langsung "Gagal kirim". Karena itu **jangan** membalas 4xx selain 409 untuk kondisi yang bisa pulih sendiri.

## 5. Konfigurasi

Tambah ke `Config` (`config.go`) dan `.env.example` bridge:

| Env | Default | Keterangan |
|---|---|---|
| `OUTBOUND_TOKEN` | kosong | 🔒 Sama dengan `CS_BRIDGE_OUTBOUND_TOKEN` di `.env` Dashboard. Kosong = endpoint mati (404). Bila diisi wajib ≥ 32 karakter (pola validasi sama dengan `DASHBOARD_INGEST_TOKEN`). **Harus berbeda** dari `DASHBOARD_INGEST_TOKEN`; `loadConfig` menolak bila sama |
| `OUTBOUND_QUIET_AFTER_INBOUND` | `2m` | Jeda minimum sejak pesan masuk terakhir lead sebelum outbound boleh dikirim (6.3) |

Buat token: `openssl rand -hex 32`.

## 6. Perilaku

### 6.1 Urutan pemeriksaan di handler

File baru `outbound.go`, route didaftarkan di `Routes()` **hanya bila** `cfg.OutboundToken != ""`:

1. Token: `Authorization: Bearer …` dibandingkan dengan `subtle.ConstantTimeCompare`. Gagal → 401. Jangan log nilai token.
2. Baca body dengan `io.LimitReader` (16 KB) lalu parse JSON. Gagal → 400.
3. Validasi field (4.1). `ignoredChat(chat_id)` (grup/status/broadcast) → 400. Chat milik nomor admin (`isAdmin(settings, normalizePhone(chat_id))`) → 400.
4. `strings.Contains(text, "[[")` → 400 `marker`. Pemeriksaan ini dilakukan sebelum hal lain yang menyentuh WhatsApp.
5. Idempotensi (6.4): bila sudah pernah terkirim → 200 `duplicate`.
6. `store.PausedUntil(chat_id, now)` aktif → 409 `paused`.
7. Sibuk (6.3) → 409 `busy`.
8. Kirim (6.2).

### 6.2 Kirim dan catat

- Ambil `c.work` milik chat (buat entri `chat` bila belum ada, seperti `enqueue`) supaya outbound **tidak pernah** bersamaan dengan giliran AI di chat yang sama. Pakai `TryLock`. Gagal → 409 `busy`; jangan menunggu, karena giliran AI bisa sampai `HERMES_TIMEOUT`.
- Kirim lewat `b.gowa.SendText(ctx, chat_id, text)` dengan timeout 30 detik. Jangan pakai `b.send` karena fungsi itu menelan error. Error atau id kosong → 502.
- `b.rememberSent(id)` agar gema `is_from_me` (bila suatu saat GOWA mengirimnya) tidak dianggap admin mengambil alih chat.
- Simpan `LastOutbound` (6.5) dan idempotensi (6.4) di `state.json` lewat `Store`.
- `b.emit(ref, "message.out", eventIDFor("out", id), now, data)` dengan `ref := b.leadFor(chat_id)` dan:
  ```json
  {"wa_message_id":"<id>","body":"<text>","sender":"followup","reference_id":123,"purpose":"followup"}
  ```
  Dashboard hanya menandai follow-up terkirim bila `purpose` kosong atau `followup`. Untuk `payment_verified`, `reference_id` = 0.
- Log `info` cukup berisi chat, purpose, reference_id, dan wa_message_id. 🔒 Jangan log isi teks.
- Balas 200.

### 6.3 Aturan "sibuk"

Tujuannya mencegah follow-up memotong percakapan yang sedang berjalan. Dashboard membatalkan follow-up saat menerima `message.in`, tapi event itu bisa tertahan beberapa detik di antrean bridge. Kondisi sibuk (→ 409 `busy`) bila salah satu benar:
- chat punya `pending` atau `timer` debounce aktif (lead sedang mengetik beruntun);
- `c.work` sedang dipegang (giliran AI berjalan);
- pesan masuk terakhir lead lebih baru dari `OUTBOUND_QUIET_AFTER_INBOUND`. Tambahkan `lastInbound time.Time` di struct `chat`, diisi di `handleEvent` untuk pesan non-admin.

Dashboard akan mengulang. Pada saat itu event `message.in` biasanya sudah membatalkan follow-up di Dashboard, sehingga pengulangan tidak terjadi.

### 6.4 Idempotensi

Dashboard bisa mengulang request yang sebenarnya sudah terkirim, mis. timeout jaringan setelah GOWA sukses. Bridge menyimpan kunci di `Store` (map baru `Outbound`, entri berumur > 7 hari dibersihkan saat menulis):
- `purpose=followup` dan `reference_id > 0` → kunci `followup:<reference_id>`.
- Selain itu → kunci `<purpose>:<chat_id>:<sha256(text) 16 hex pertama>`, berlaku 24 jam.

Kunci ada → balas 200 dengan `wa_message_id` lama dan `"duplicate":true`, tanpa mengirim ulang dan tanpa emit ulang.

### 6.5 Konteks untuk CS AI saat lead membalas

Hermes tidak melihat pesan outbound di riwayat sesinya, jadi konteksnya disisipkan lewat prompt sistem.
- `Store.LastOutbound map[string]OutboundNote` (`chat_id` → `{Text, Purpose, At}`), disimpan atomik seperti field lain di `state.json`. Teks dipotong 600 karakter. Entri > 30 hari diabaikan dan dibersihkan.
- `buildSystemPrompt` mendapat parameter tambahan (mis. `last *OutboundNote`). Di bagian `## Konteks chat ini` ditambahkan:
  - `purpose=followup`:
    `Pesan terakhir yang dikirim admin ke pengirim ini (follow-up, <tanggal WIB>): "<teks>". Pengirim mungkin sedang membalas pesan itu. Lanjutkan dengan wajar dari sana.`
  - `purpose=payment_verified`:
    `Admin (bukan Anda) sudah mengirim konfirmasi pembayaran pada <tanggal WIB>: "<teks>". Anda boleh menanggapi ucapan terima kasih, tetapi tetap jangan membuat pernyataan baru tentang status pembayaran; pertanyaan pembayaran tetap dialihkan ke admin.`
- `flush` mengambil catatan ini dari store saat membangun prompt. Catatan **tidak** dihapus setelah dipakai. Catatan baru menimpa yang lama.
- 🔒 Teks outbound berasal dari staf yang sudah menyetujui. Tetap tulis di dalam tanda kutip dan perlakukan sebagai data. Aturan di `prompts/locked.md` tetap menang.
- Pratinjau prompt di `/cs-ai` (`apiPromptPreview`) memanggil `buildSystemPrompt` dengan `nil`.

### 6.6 Interaksi dengan fitur yang ada

| Fitur | Perilaku |
|---|---|
| CS AI dimatikan di `/cs-ai` (`Settings.Enabled=false`) | Outbound **tetap** dikirim (sudah disetujui manusia). Balasan lead tidak dijawab AI, sama seperti sekarang |
| Rate limit per jam (`hits`) | Outbound **tidak** dihitung sebagai pesan lead |
| `AI OFF <nomor>` / admin membalas dari HP | Chat dijeda, outbound → 409 `paused` sampai jeda habis |
| `/healthz` | Tambah `"outbound": true/false` (fitur aktif atau tidak). Jangan tampilkan token |
| Halaman `/cs-ai` status | Opsional: jumlah outbound terkirim hari ini |

## 7. Keamanan

- 🔒 Token outbound terpisah dari token ingest, ≥ 32 karakter, dibandingkan constant-time, hanya ada di `.env`.
- 🔒 Route di luar `/cs-ai` agar tidak diteruskan proxy GOWA ke internet. Tambahkan test yang memastikan `GET/POST /cs-ai/api/outbound` **tidak** ada.
- 🔒 Tolak marker `[[`, tolak chat grup/status/admin, batasi panjang teks.
- 🔒 Jangan log isi teks maupun token.
- Endpoint tidak mengizinkan nomor sembarang: `chat_id` wajib JID. Dashboard hanya mengirim ke chat yang pernah masuk sebagai lead. Bridge tidak perlu memverifikasi riwayat, tapi **boleh** menolak `chat_id` yang belum pernah terlihat (`store.Phones` / `chats`) dengan 400 sebagai pengaman tambahan. Rekomendasi: aktifkan.

## 8. Test (wajib, di `outbound_test.go`, pakai `fakeSender`/`fakeAgent` yang sudah ada)

1. `OUTBOUND_TOKEN` kosong → route tidak terdaftar (404).
2. Token salah atau tanpa header → 401, `fakeSender` tidak dipanggil.
3. JSON rusak, `chat_id` grup, `chat_id` admin, teks kosong, teks > 1000, `purpose` asing → 400.
4. Teks `"Isi [[FORM]] ya"` → 400 `marker`.
5. Chat dijeda (`SetPause`) → 409 `paused`.
6. Lead baru menulis (< 2 menit) atau debounce aktif → 409 `busy`. Setelah jam palsu maju > 2 menit → 200.
7. Sukses `followup` → `fakeSender` menerima teks persis; event `message.out` berisi `sender=followup`, `reference_id`, `purpose`; `state.json` berisi `LastOutbound` dan kunci idempotensi.
8. Request yang sama diulang → 200 `duplicate:true`, `fakeSender` tetap dipanggil sekali, event tidak bertambah.
9. `payment_verified` → event dengan `reference_id=0`, `purpose=payment_verified`.
10. Lead membalas setelah follow-up → `fakeAgent` menerima prompt sistem yang memuat teks follow-up (6.5).
11. GOWA error → 502, tidak ada event, tidak ada kunci idempotensi (boleh dicoba lagi).
12. `GET /healthz` memuat `"outbound": true`. `POST /cs-ai/api/outbound` → bukan 200.
13. `loadConfig`: token < 32 karakter ditolak; token sama dengan `DASHBOARD_INGEST_TOKEN` ditolak.

## 9. Dokumentasi

Di `docs/cs-ai-hermes.md`, tambah **Langkah 7 — Follow-up dari Dashboard**:
1. Buat token: `openssl rand -hex 32`.
2. Isi `OUTBOUND_TOKEN=<token>` di `.env` bridge, lalu restart bridge. Cek `curl -s http://172.17.0.1:8650/healthz` menunjukkan `"outbound":true`.
3. Isi `CS_BRIDGE_URL=http://172.17.0.1:8650` dan `CS_BRIDGE_OUTBOUND_TOKEN=<token yang sama>` di `.env` Dashboard, lalu restart Dashboard.
4. Uji tanpa Dashboard ke nomor uji milik sendiri yang sudah pernah chat:
   ```
   curl -s -X POST http://172.17.0.1:8650/api/outbound \
     -H "Authorization: Bearer $OUTBOUND_TOKEN" -H 'Content-Type: application/json' \
     -d '{"chat_id":"62812xxxx@s.whatsapp.net","text":"Tes follow-up","reference_id":0,"purpose":"payment_verified"}'
   ```
5. Lanjutkan sesuai panduan Dashboard `docs/panduan/crm-followup.md`: label `uji-followup`, nyalakan saklar, satu kiriman uji.

Tambah juga baris `OUTBOUND_TOKEN` dan `OUTBOUND_QUIET_AFTER_INBOUND` di tabel env dokumen itu.

## 10. Acceptance criteria

- Semua test bagian 8 lulus. `go vet ./...` bersih.
- Dari Dashboard (Fase D, saklar menyala, label uji):
  - follow-up yang disetujui terkirim dari nomor CS;
  - muncul di transkrip lead sebagai "Follow-up";
  - status di Dashboard menjadi Terkirim.
- Lead membalas follow-up, dan balasan CS AI nyambung dengan isi follow-up.
- Saat admin sedang menangani chat (dijeda), follow-up tidak terkirim. Dashboard mencoba lagi; bila masih dijeda setelah 3 percobaan, muncul di "Gagal kirim".
- Bridge restart di antara dua pengulangan request yang sama tidak menghasilkan pesan ganda.
- `/api/outbound` tidak bisa diakses lewat `https://wa.ruangsanad.com/...`.

## 11. Risiko

| Risiko | Mitigasi |
|---|---|
| Pesan ganda saat Dashboard mengulang | Idempotensi 6.4 disimpan di `state.json` |
| Follow-up memotong percakapan aktif | Aturan sibuk 6.3 + pembatalan follow-up oleh Dashboard saat `message.in` |
| Endpoint terekspos publik | Route di luar `/cs-ai`, listen di alamat internal, token terpisah, test 12 |
| CS AI salah paham konteks follow-up | Catatan konteks 6.5; review transkrip mingguan (PRD Dashboard 15.6) |
| Nomor CS diblokir | Pembatasan di Dashboard (persetujuan manusia, cap harian, jeda, jam kirim, saklar default mati). Bridge tidak menambah volume |
