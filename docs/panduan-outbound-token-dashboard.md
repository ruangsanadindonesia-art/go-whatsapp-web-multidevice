# Panduan outbound token Dashboard ke CS AI Bridge

Panduan ini untuk menghubungkan **Dashboard Ruang Sanad** (pengirim follow-up yang disetujui manusia) dengan **GoWA Hermes CS Bridge**. Endpoint internal: `POST http://172.17.0.1:8650/api/outbound`.

**Jangan mengirim token melalui chat, issue, screenshot, GitHub, atau commit.** File `.env` hanya ada di server. Token outbound berbeda dari token ingest.

## 1. Persiapan

- Pastikan perubahan PR outbound bridge sudah **di-deploy dan diverifikasi** sebelum mengaktifkan pengiriman Dashboard.
- Ketahui lokasi `.env` Dashboard yang benar sesuai metode deploy di VPS. Jangan mengasumsikan foldernya.
- Pastikan Dashboard dapat mengakses IP bridge `172.17.0.1:8650` melalui jaringan internal. Jangan membuka port 8650 ke publik.
- Pastikan akun GOWA khusus bridge dapat mengirim dari device `cs-ai`; akun Dashboard **tidak** perlu diberi akses device tersebut.
- Simpan salinan aman file konfigurasi sebelum mengubahnya. Jangan membagikan isi file.

## 2. Buat satu token outbound

Jalankan di terminal VPS yang aman:

```bash
umask 077
openssl rand -hex 32
```

Perintah menghasilkan **64 karakter heksadesimal** (32 byte acak). Salin hasilnya secara aman dan gunakan **nilai yang sama** untuk `OUTBOUND_TOKEN` bridge serta `CS_BRIDGE_OUTBOUND_TOKEN` Dashboard.

Jangan gunakan token yang sama dengan `DASHBOARD_INGEST_TOKEN` / `CS_BRIDGE_INGEST_TOKEN`. Bridge menolak start jika keduanya identik, dan menolak token outbound yang panjangnya kurang dari 32 karakter.

## 3. Pasang di .env bridge

Edit file konfigurasi bridge **di server**, bukan template atau repository:

```bash
sudoedit /opt/gowa-deploy/cs-bridge.env
```

Tambahkan atau sesuaikan baris:

```dotenv
OUTBOUND_TOKEN=<TOKEN_ACAK_DARI_LANGKAH_2>
OUTBOUND_QUIET_AFTER_INBOUND=2m
```

Jangan mengganti `DASHBOARD_INGEST_TOKEN` yang sudah digunakan untuk event masuk ke Dashboard. Cek agar tidak ada nama variabel yang terduplikasi.

## 4. Pasang di .env Dashboard

Buka **file .env yang benar-benar dibaca proses Dashboard**. Path bergantung pada deployment Dashboard. Isi:

```dotenv
CS_BRIDGE_URL=http://172.17.0.1:8650
CS_BRIDGE_OUTBOUND_TOKEN=<TOKEN_ACAK_YANG_SAMA_DENGAN_OUTBOUND_TOKEN_BRIDGE>
```

Parameter di atas berbeda dari konfigurasi `CS_BRIDGE_INGEST_TOKEN`, yang dipakai untuk arah **bridge → Dashboard**. Pastikan tidak menukar keduanya.

Jangan menaruh token di URL, source code, JavaScript browser, perintah `curl` yang disimpan ke shell history, atau log Dashboard.

## 5. Restart bertahap setelah approval deployment

Konfigurasi tidak aktif sebelum service yang bersangkutan memuat ulang `.env`:
1. Deploy build bridge yang mengandung route outbound, **setelah** test dan approval produksi.
2. Muat ulang container bridge dengan prosedur deploy yang berlaku (contoh instalasi Compose pada server yang telah diverifikasi: `docker compose -f /opt/gowa-deploy/compose.yml up -d --no-deps --force-recreate hermes_cs_bridge`). Jangan melakukan restart saat sesi CS kritis tanpa jadwal.
3. Jalankan `curl -fsS http://172.17.0.1:8650/healthz` dari host. Respons harus memuat `"outbound":true`. Status ini hanya membuktikan fitur diaktifkan, belum membuktikan pengiriman berhasil.
4. Restart service Dashboard sesuai process manager/deploy yang terverifikasi. Jangan mengasumsikan `systemctl`, PM2, atau Docker sebelum memeriksa instalasinya.

## 6. Smoke test yang aman

Gunakan nomor WhatsApp uji yang dimiliki/dikendalikan tim dan **sudah pernah mengirim pesan ke nomor CS**. Chat aktif, dijeda, atau baru saja mengirim pesan dapat menghasilkan `409 busy` atau `409 paused` secara normal.

Pengujian `POST /api/outbound` akan **benar-benar mengirim WhatsApp**, jadi hanya jalankan setelah approval pengiriman ke nomor uji. Cara paling aman adalah mengaktifkan label `uji-followup` di Dashboard sesuai `docs/panduan/crm-followup.md`, menyetujui satu pesan, lalu menjalankan satu kiriman. Pastikan:
- Pesan terkirim dari nomor CS yang benar.
- Event `message.out` tampil sebagai **Follow-up**, beserta referensi dan purpose.
- Dashboard menandai antrean **Terkirim**.
- Jika nomor uji membalas, konteks balasan Hermes sesuai follow-up terakhir.
- Mengulang request dengan `reference_id` yang sama tidak mengirim duplikat.
- Request dari proxy publik `/cs-ai/api/outbound` tidak tersedia.

Jika perlu pengujian HTTP manual, jalankan pada host menggunakan token dari konfigurasi aman tanpa mencetaknya di log atau terminal yang direkam. Gunakan `reference_id` uji yang unik dan payload `purpose=followup`. Jangan pernah mengirim ke lead riil tanpa persetujuan.

## 7. Masalah umum

| Gejala | Pemeriksaan |
|---|---|
| 404 `disabled` atau endpoint tidak ada | `OUTBOUND_TOKEN` belum terpasang, konfigurasi belum dimuat, atau bridge belum di-deploy |
| 401 `unauthorized` | Dua nilai token tidak sama atau header Authorization tidak terkirim |
| 400 `invalid` | JID harus personal dan pernah terlihat, teks 1–1000 karakter, purpose hanya `followup` / `payment_verified` |
| 400 `marker` | Hapus marker seperti `[[FORM]]` dari teks final yang disetujui |
| 409 `paused` | Admin mengambil alih chat; tunggu sampai jeda berakhir |
| 409 `busy` | Ada pesan baru, debounce, atau giliran Hermes aktif; Dashboard akan retry |
| 502 `send failed` | Periksa kesehatan GOWA dan nomor CS, lalu status idempotensi jika request terputus |
| `/healthz` outbound=false | Variabel belum terpasang/dimuat ulang |

Untuk diagnosis cukup cek status/keberadaan nilai seperti `TOKEN_SET=YES`. Jangan menampilkan nilai token. Bila token diduga bocor, lakukan rotasi **kedua sisi** secara terkoordinasi, hentikan sementara worker follow-up, dan uji lagi.

## Referensi

- `docs/prd-cs-bridge-outbound.md`
- `docs/cs-ai-hermes.md` bagian Langkah 7
- Dashboard: `docs/panduan/bridge-outbound-api.md` dan `docs/panduan/crm-followup.md`
