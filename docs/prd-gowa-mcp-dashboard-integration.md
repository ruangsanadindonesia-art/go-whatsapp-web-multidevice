# PRD GoWA — MCP Agent Gateway dan Integrasi Dashboard Ruang Sanad

| Metadata | Nilai |
|---|---|
| Status | **PROPOSED / READY FOR REVIEW** — belum implementasi |
| Dibuat | 28 Oktober 2026 |
| Repository owner | ruangsanadindonesia-art/go-whatsapp-web-multidevice |
| Baseline source main | 3031d9a4163f5013963b7d52e8e442f65bd48890 |
| PRD pasangan Dashboard | [Integrasi WhatsApp Dashboard–GoWA MCP](https://github.com/dinastran/ruang-sanad/blob/main/docs/prd/prd-integrasi-whatsapp-dashboard-gowa-mcp.md) |
| PRD existing CS Bridge | [CS AI Bridge Outbound](./prd-cs-bridge-outbound.md) |
| Referensi auth existing | [MCP OAuth](./mcp-oauth.md) |
| Status runtime | **NOT VERIFIED**; inspeksi source code, bukan smoke test produksi |
| Perubahan yang disetujui dokumen ini | Perencanaan fitur GoWA saja; tidak mengotorisasi coding, perubahan credential, merge, atau deployment |

**Aturan dokumen:** Existing code = telah terlihat di source main. Existing PRD = spesifikasi sebelumnya yang tidak otomatis terbukti berjalan. Proposed = fitur yang perlu dibuat. Gunakan source code terbaru pada implementasi jika telah berubah dari baseline PRD.

## 1. Latar belakang dan posisi GoWA dalam ekosistem

GoWA adalah gateway WhatsApp multi-device. Saat ini memiliki REST API, MCP, shared sending usecases, chat storage, scheduler, serta UI GoWA terpisah. Ruang Sanad ingin:

1. Memakai MCP GoWA dari Claude Web/Desktop/Code dan Hermes untuk berinteraksi dengan seluruh kategori tools WhatsApp yang tersedia, sesuai permission.
2. Mengirim pesan ke nomor atau grup, termasuk teks, gambar, video, audio, file/dokumen, dan scheduled messages.
3. Memakai media dari URL atau lampiran yang diberikan dalam lingkungan AI.
4. Melihat hasil komunikasi dan jadwal dari UI GoWA, plus identitas agent, audit, serta antrean approval grup.
5. Tetap mendukung Dashboard Ruang Sanad yang mengelola follow-up Keuangan, Admin Kelas, Koordinator Guru dan CS AI.

Keputusan tanggung jawab:

- **GoWA:** otentikasi MCP GoWA, otorisasi device/tool/action, pengiriman transport, direct scheduler, group approval direct-MCP, media staging, audit transport, device status dan UI GoWA.
- **Dashboard:** validitas data santri/guru/tagihan/kelas, role staf, template dan approval bisnis, jadwal bisnis, wa_outbox, serta state hasil tindak lanjut.
- **Hermes CS Bridge:** pengiriman melalui nomor CS AI dan sinkronisasi konteks percakapan Hermes untuk workflow CRM; kontrak existing tidak diambil alih oleh MCP direct.

PRD GoWA ini bukan duplikat PRD Dashboard. Kontrak lintas aplikasi saling dirujuk, tetapi implementasi dan acceptance criteria di sini fokus pada repository GoWA dan dependensinya.

## 2. Temuan baseline (existing) dan gap

| Area | Existing code/sumber | Gap terhadap target |
|---|---|---|
| MCP tool registration | [src/ui/mcp/server.go](../src/ui/mcp/server.go) menyediakan whatsapp_send, whatsapp_schedule, whatsapp_message, whatsapp_chat, whatsapp_group, whatsapp_app | Belum ada granular server policy per principal/device/action |
| MCP sending | [src/ui/mcp/send.go](../src/ui/mcp/send.go), [schema](../src/ui/mcp/schemas.go) | Media MCP memakai URL; belum ada end-to-end attachment staging lintas klien atau pending group approval |
| MCP group control | [src/ui/mcp/group.go](../src/ui/mcp/group.go) | Info dan mutasi admin grup menggunakan satu tool dengan sub-action; harus dipisahkan permission per sub-action |
| MCP app/session | [src/ui/mcp/app.go](../src/ui/mcp/app.go) | Status dan logout/login/reconnect tidak boleh berbagi izin yang sama |
| MCP OAuth | [src/cmd/mcp_oauth.go](../src/cmd/mcp_oauth.go), [docs OAuth](./mcp-oauth.md) | OAuth disabled by default; restricted APP_BASIC_AUTH_DEVICES user ditolak MCP karena keseluruhan tools belum dibatasi per-device |
| Device resolution | [src/ui/mcp/device.go](../src/ui/mcp/device.go), [src/ui/mcp/route.go](../src/ui/mcp/route.go) | device_id dan X-Device-Id tidak boleh menjadi sumber privilege |
| Shared send usecase | [src/usecase/send.go](../src/usecase/send.go) | Pesan sukses disimpan asynchronous; histori chat dapat tertunda/gagal; origin/principal belum terhubung sebagai audit activity baru |
| Shared scheduler | [src/usecase/schedule.go](../src/usecase/schedule.go), [REST scheduler](../src/ui/rest/schedule.go) | Job scheduled langsung tidak punya approval per group occurrence dan provenance agent sesuai requirement baru |
| Chatstorage | [src/domains/chatstorage/interfaces.go](../src/domains/chatstorage/interfaces.go) | Perlu audit/idempotency/approval durable yang selaras dengan device scope |
| UI GoWA | [aldinokemal/gowa-ui](https://github.com/aldinokemal/gowa-ui) | Repo UI terpisah dan diambil sebagai release; fitur AI Activity dan Approvals belum dibuktikan ada |
| CS Bridge | [docs/prd-cs-bridge-outbound.md](./prd-cs-bridge-outbound.md) | Tidak boleh di-bypass. Business outbound dari device cs-ai harus mengikuti existing bridge |

**Tidak boleh menyatakan OAuth Claude/Hermes, upload langsung, scoped authorization, atau approval grup production PASS hanya karena desain ini ada.** Verifikasi client/version dan runtime merupakan pekerjaan fase implementasi.

## 3. Product decisions yang dikunci

- Seluruh device GoWA masuk cakupan platform, dengan **pemberian izin per principal/agent/device**, bukan unrestricted access.
- Semua kategori tools tersedia bagi agent yang memang memiliki izin; sub-action berisiko dibatasi atau memerlukan otorisasi tambahan.
- Direct personal send dapat otomatis **hanya** setelah policy, recipient dan domain routing lolos.
- Semua pengiriman ke grup melalui **GoWA MCP agent/direct flow** memerlukan persetujuan manusia. Approval juga berlaku pada scheduled/recurring occurrence.
- Media diterima melalui URL maupun adapter file/chat upload dengan fallback secure upload.
- Permintaan dari Dashboard yang merupakan proses bisnis memakai **Dashboard wa_outbox → GoWA REST**, bukan memakai akses Claude/Hermes agar mengambil alih business policy.
- Semua outbound terkait lead dari device cs-ai melalui Hermes CS Bridge, termasuk follow-up yang berasal dari Dashboard.
- Pertahankan REST/MCP yang sudah ada sejauh kompatibel; perubahan kontrak harus diberi versioning/compatibility plan.

### Keputusan terbuka (bukan persetujuan otomatis)

- Apakah pesan bisnis non-CS dari Dashboard otomatis atau memerlukan review staf: **default human approval**; keputusan akhir ada di PRD Dashboard.
- Delegasi approver grup selain Super Admin: **default Super Admin** sampai role management disepakati.
- Ownership build/fork UI GoWA: perlu ditetapkan sebelum menambah halaman.
- Rate limits, quiet hours, audit/media retention per device: ditetapkan pada security review sebelum produksi.
- Perluasan Bridge CS untuk outbound media/grup: **di luar MVP** dan membutuhkan kontrak terpisah.

## 4. Arsitektur target

    Claude / Hermes Operator
            |
       Remote MCP HTTPS
            |
    MCP authentication context
            |
    Principal + Device + Action + Target Policy
            |
       Request validation
            |
      Origin / purpose router
        /           \
    Personal       Group
      |             |
    Direct     Durable approval
      |             |
      +---> Safe dispatcher <--- Dashboard REST (non-CS)
                   |
             Existing usecase
                   |
          WhatsApp device manager
                   |
          WhatsApp personal/group
                   |
       Message/schedule/audit state

    Dashboard CRM follow-up
              |
      Dashboard wa_outbox
              |
       Hermes CS Bridge
              |
        GoWA REST cs-ai
              |
           WhatsApp

**Important:** memblokir seluruh akses REST menuju device cs-ai akan merusak Bridge. Pembatasan harus mengizinkan service identity terpercaya milik Bridge untuk menjalankan existing kontrak CS, sambil menolak actor lain yang mencoba direct-send cs-ai lewat MCP atau REST tidak berwenang. Kebijakan ini perlu diuji end-to-end terhadap authenticator, device mapping, dan profile Bridge.

### Security boundary

- OAuth human-facing MCP dan service credential Dashboard/Bridge adalah identitas yang berbeda.
- Kode model tidak boleh memilih principal sendiri lewat request arguments.
- Setiap jalur pengiriman yang dapat dipakai oleh agent harus mengalir melalui policy enforcement yang konsisten; tidak cukup hanya membatasi satu tool handler jika direct REST masih dapat dipakai dengan kredensial yang sama.
- UI GoWA yang dioperasikan manusia boleh mempertahankan workflow existing menurut role; jangan memaksakan perubahan perilaku UI REST tanpa analisis kompatibilitas.

## 5. Scope dan non-goals

### MUST (MVP GoWA)

1. AuthZ granular per principal, device, tool/sub-action, tujuan (personal/group), source dan risiko.
2. Identity audit dan revocation, termasuk OAuth-compatible clients yang diuji.
3. Direct personal sending melalui MCP, dan group sending dengan durable human approval.
4. Scheduler GoWA untuk direct MCP yang melakukan recheck permission dan approval pada waktu eksekusi.
5. Media URL hardening + private attachment staging/fallback.
6. Replay/idempotency protection, ambiguity-safe statuses, per-device rate limit dan audit.
7. Menjaga kontrak REST Dashboard non-CS dan CS Bridge existing.
8. Endpoint/manajemen minimum untuk AI Activity, Approvals dan identity policy; UI integration plan yang mempertimbangkan separate UI repo.
9. Test negatif untuk bypass auth, device, approval, CS route dan SSRF.

### OUT OF SCOPE

- Membuat ulang workflow tagihan, santri, kelas, guru atau database Dashboard.
- Memigrasikan bisnis scheduler Dashboard menjadi GoWA scheduled_send.
- Membuka direct MCP universal ke device cs-ai.
- Memodifikasi logic follow-up Script 30 Hari atau mengganti CS Bridge.
- Mewajibkan seluruh operator menggunakan UI GoWA; business UI tetap berada di Dashboard.
- Mengaktifkan bulk messaging/promotional automation tanpa persetujuan penerima dan kebijakan volume.
- Mengubah server produksi, credential, proxy, Cloudflare, DNS, atau mengirim pesan nyata dalam proses penulisan PRD.

## 6. Actor dan permission model

| Principal | Device scope | Permission awal | Batas |
|---|---|---|---|
| GoWA Super Admin manusia | Seluruh device yang dimiliki organisasi | Admin/manage | Approval/administrasi diaudit; workflow CS tetap lewat Bridge |
| Claude operator | Device eksplisit yang di-grant | READ_CHAT, SEND_PERSONAL, SCHEDULE sesuai grant | SEND_GROUP meminta approval; no privileged group/session admin secara default |
| Hermes operator | Device eksplisit yang di-grant | Sama, profile terpisah | Unattended cron tidak dapat approve grup otomatis |
| Dashboard service | Device non-CS yang dimapping | SEND_PERSONAL dan operasi transport yang diperlukan | Request berasal dari business outbox yang sudah tervalidasi |
| Hermes CS Bridge service | Device cs-ai | Hanya capability yang diperlukan existing Bridge | Tidak bisa dipakai Claude/Hermes operator untuk bypass |
| Read-only agent | Device scoped | READ_STATUS dan read ops sesuai izin | Tidak boleh send, edit, revoke, logout, atau group mutation |

### Permission categories (proposed)

- DEVICE_STATUS; DEVICE_LOGIN; DEVICE_LOGOUT; DEVICE_RECONNECT.
- CHAT_LIST; CHAT_READ; CONTACT_READ; CHAT_ARCHIVE.
- MESSAGE_SEND_PERSONAL; MESSAGE_SEND_GROUP_REQUEST; MESSAGE_SCHEDULE_PERSONAL; MESSAGE_SCHEDULE_GROUP_REQUEST.
- MESSAGE_REACT; MESSAGE_EDIT; MESSAGE_REVOKE; MESSAGE_DELETE; MESSAGE_MEDIA_DOWNLOAD.
- GROUP_READ; GROUP_CREATE; GROUP_JOIN_LEAVE; GROUP_PARTICIPANT_MANAGE; GROUP_ADMIN_MANAGE.
- SCHEDULE_READ; SCHEDULE_PAUSE; SCHEDULE_RESUME; SCHEDULE_CANCEL.
- APPROVAL_REVIEW; INTEGRATION_ADMIN; AUDIT_READ.

Policy mengevaluasi **tool + sub-action**. Contoh: whatsapp_group(action=info) bukan grant remove_participants; whatsapp_app(action=status) bukan grant logout. File download dan contact read diberi scope tersendiri. Policy tidak boleh menerima full-device fallback dari parameter/header saat principal sebenarnya restricted.

### Authentication design

- Reuse OAuth 2.1 existing untuk human-connected remote MCP, termasuk DCR/PKCE dan token hashing existing.
- Lakukan targeted audit untuk memastikan identity yang tersedia dapat dipetakan ke principal, granted devices, actions dan OAuth client tanpa mengubah keamanan login REST/UI secara tidak sengaja.
- User restricted APP_BASIC_AUTH_DEVICES saat ini ditolak pada MCP. **Jangan menghapus penolakan itu terlebih dahulu** sebelum policy per-device/action benar-benar enforced serta negative tests lulus.
- Service agents membutuhkan identitas tersendiri dengan scope/rotation/revoke; jangan memakai administrator Basic credentials atau token Dashboard rsai_* sebagai bypass.
- Auth dan policy perlu context propagation ke worker scheduler. Token expiration/revocation harus diuji pada dispatch yang terjadi kemudian.

## 7. Functional requirements

| ID | Requirement | Priority |
|---|---|---|
| G-FR-01 | Semua MCP operations terikat server-side principal + client + device + action + target + origin | P0 |
| G-FR-02 | Deny access device lain meski explicit device_id atau X-Device-Id diganti | P0 |
| G-FR-03 | Restricted Basic account tidak mendapat implicit MCP privileges | P0 |
| G-FR-04 | OAuth issuance/revoke/expiry dihubungkan ke authorization scopes | P0 |
| G-FR-05 | whatsapp_send mendukung seluruh tipe existing, dengan validasi target dan media | P0 |
| G-FR-06 | Agent direct personal send boleh auto hanya setelah policy dan routing lolos | P0 |
| G-FR-07 | Direct MCP group send selalu masuk PENDING_APPROVAL tanpa mengirim | P0 |
| G-FR-08 | Approval dibuat oleh human identity di trusted UI, tidak oleh tool agent sendiri | P0 |
| G-FR-09 | Approval mengikat device, target, payload/media hash, jadwal dan occurrence | P0 |
| G-FR-10 | Mutasi isi/tujuan/media/waktu invalidates approval | P0 |
| G-FR-11 | Scheduled/recurring group send harus mendapat approval sah per occurrence | P0 |
| G-FR-12 | Semua sub-action whatsapp_group/message/chat/app/schedule punya policy granular | P0 |
| G-FR-13 | CS AI device direct send ditolak kecuali service Bridge trusted yang memiliki route existing | P0 |
| G-FR-14 | REST Dashboard non-CS tetap mendukung basic/device-scoped service send | P0 |
| G-FR-15 | Idempotency key unik untuk logical send dan occurrence; retry tidak double send | P0 |
| G-FR-16 | Timeout ambigu menghasilkan UNCERTAIN, bukan blind resend | P0 |
| G-FR-17 | URL media tervalidasi saat fetch termasuk redirect/DNS/network boundary | P0 |
| G-FR-18 | Private attachment upload/staging aman, TTL cleanup, fallback bila client tidak mendukung binary transfer | P0 |
| G-FR-19 | Schedule owner dibedakan direct GoWA vs business Dashboard, tanpa double-job | P0 |
| G-FR-20 | Origin, principal, action, device, target category, status, request ID dan message ID diaudit | P0 |
| G-FR-21 | UI Chats/Scheduled tetap kompatibel; AI Activity dan Approvals ditambahkan via owned UI build | P1 |
| G-FR-22 | Approver/revoke/deny/rate-limit/media errors dapat diinspeksi tanpa secret leakage | P0 |
| G-FR-23 | Kill switch memungkinkan stop direct AI sends tanpa mematikan Bridge CS | P0 |
| G-FR-24 | Group dan device management tidak otomatis diizinkan karena SEND_PERSONAL grant | P0 |

## 8. Flow end-to-end dan state machine

### 8.1 Direct personal text/media

1. Claude/Hermes mengirim tool call dengan target personal, jenis pesan dan device yang diminta.
2. Gateway membangun identity dari token, mengevaluasi device/action/target, dan memeriksa apakah request termasuk business workflow atau device CS.
3. Untuk media, proses validasi/preparation menghasilkan media_ref immutable.
4. Dispatcher mengklaim idempotency key, memanggil shared usecase GoWA, lalu mencatat hasil WA message ID.
5. Hasil dikembalikan ke client sebagai SENT atau error/state yang akurat. Chat storage existing menyimpan pesan secara asynchronous; histori bisa tertunda.

**Tidak boleh** mengklaim DELIVERED atau READ hanya dari HTTP/MCP success. Jika content hasil send tidak masuk chat storage, activity tetap memuat transport outcome dan catatan storage failure.

### 8.2 Direct group message

1. Resolve group JID dari device yang diizinkan; periksa membership.
2. Ambil snapshot exact recipient, device, pesan, caption, jenis media, immutable file hash dan schedule.
3. Request dibuat PENDING_APPROVAL. **Tidak memanggil transport send**.
4. Approver manusia berwenang melihat preview, boleh REJECT/CANCEL atau APPROVE.
5. Dispatcher melakukan recheck actor policy, target, connected device, payload hash, approval TTL dan idempotency sebelum satu send.
6. Audit mencatat requester, approver, timestamp, keputusan dan message ID.
7. Jika request expired/edited/rejected, **tidak ada pengiriman**.

### 8.3 Schedule personal

MCP agent yang diberi MESSAGE_SCHEDULE_PERSONAL dapat membuat scheduled send lewat GoWA scheduler. Status dan pause/resume/cancel tetap memakai GoWA Scheduled. Pada saat due, worker mengevaluasi ulang policy, target, media dan token revocation; jangan hanya percaya permission saat jadwal dibuat. Jika tidak diizinkan, job ditahan/dibatalkan dengan status terjelaskan dan audit.

### 8.4 Recurring group

Recurring group request disimpan sebagai jadwal direct milik GoWA, tetapi tiap occurrence menunggu approval manusia yang mengikat payload dan waktu occurrence. Jika approval belum ada atau expired saat due, occurrence tidak dikirim; tidak ada silent fallback menjadi auto-send. Pause/cancel menyangkut seluruh schedule hanya oleh principal yang memiliki izin.

### 8.5 Dashboard business transport

Dashboard Finance/Admin Kelas/Koordinator melakukan domain eligibility, approval, dispatch window, idempotency dan jadwal pada wa_outbox. Ketika due, Dashboard melakukan **immediate send** ke GoWA REST device non-CS dengan service identity scoped; GoWA tidak membuat second ScheduledSend untuk request yang sama. GoWA menyimpan transport audit/WA message ID dan memberi respons untuk rekonsiliasi Dashboard. Nomor penerima dan konten yang dikeluarkan dari Dashboard tetap divalidasi di boundary GoWA tanpa menduplikasi aturan tagihan/kelas/guru.

### 8.6 Dashboard CS via Bridge

CS workflow mengikuti kontrak existing Dashboard → Hermes CS Bridge POST /api/outbound → GoWA device cs-ai. Permintaan existing adalah **text personal**. Bridge mencatat/menyisipkan pesan outbound ke konteks Hermes dan menghasilkan event message.out ke Dashboard. Jangan membuat direct MCP raw-send ke cs-ai, jangan memasukkan request bridge ke human group approval yang akan merusak alur existing, dan jangan mengubah signature bridge tanpa koordinasi kedua repository.

### 8.7 Core state

    DRAFT -> PENDING_APPROVAL -> APPROVED -> QUEUED/SCHEDULED -> SENDING -> SENT
                |                   |                          |
                +-> REJECTED        +-> EXPIRED                 +-> UNCERTAIN
                +-> CANCELLED       +-> CANCELLED               +-> FAILED

APPROVED bukan SENT; SENT tidak berarti delivered/read. Status UNCERTAIN muncul jika send mungkin sudah dikirim tetapi hasil tidak terkonfirmasi. Tidak retry otomatis sebelum rekonsiliasi.

## 9. Proposal interface dan kontrak data

**PROPOSED**; harus direkonsiliasi dengan REST/MCP DTO existing sebelum menulis kode.

### 9.1 SendRequest metadata

| Field | Keterangan |
|---|---|
| request_id | UUID correlation lintas client/GoWA/Dashboard |
| idempotency_key | Stable key untuk satu logical send atau occurrence |
| principal_id / client_id | Diisi server dari auth context, bukan input bebas |
| origin | claude / hermes / dashboard_finance / dashboard_admin_kelas / dashboard_koordinator / bridge_cs / gowa_ui |
| purpose | direct_message / group_message / tagihan / reminder_kelas / tindak_lanjut_guru / lead_followup / payment_verified |
| owner_system | gowa atau dashboard |
| device_id | Device yang diizinkan policy |
| recipient_jid / type | Target tervalidasi, personal/group |
| payload_type / payload_hash | Jenis pesan dan snapshot immutable |
| media_ref | Temporary private media identity (jika ada) |
| scheduled_at / timezone / occurrence_id | Jadwal hanya jika owner_system=gowa |
| approval_ref | Wajib ketika group MCP direct |
| status / error_code | State machine sesuai dispatcher |
| wa_message_id | Diisi saat transport sukses |

- Metadata dari request harus divalidasi terhadap principal; caller tidak boleh memalsukan origin=bridge_cs atau owner_system=dashboard untuk lolos approval.
- Dashboard dan Bridge harus memiliki credential/service identities berbeda; scope device bridge hanya cs-ai dan scope Dashboard non-CS hanya mapping yang sah.
- Jika existing endpoints tidak dapat menerima semua metadata, buat internal adapter yang kompatibel; jangan mematahkan REST payload yang telah dipakai Dashboard/Bridge tanpa versioning.
- Nama endpoint tambahan di sini belum dikunci. Bisa berupa MCP prepare/status tools atau protected REST endpoints untuk approval/media; pilih setelah audit kontrak.

### 9.2 Proposed MCP extensions

Existing six consolidated tools tetap dipertahankan. Fitur tambahan disarankan:

- Prepare/dry-run send: resolve device, target, type, media metadata, policy decision dan approval requirement sebelum dispatch.
- Send status: cek request id, pending approval, actual send result dan error.
- Media attachment staging: dapat media_ref yang dipakai oleh whatsapp_send tanpa mengekspos path OS.
- Jika tool baru ditambah, pastikan tool annotations, schema, scope dan response error konsisten dengan existing, tidak membuat helper tool untuk self-approval agent.

### 9.3 Audit metadata dan penyimpanan

Audit minimal mencatat principal, client, action, resolved device, target category, request_id, origin, purpose, approval actor (bila ada), status, WA message ID, latency, error category dan timestamp. Nomor/konten dapat dimasking atau dibatasi retensinya. Jangan menyimpan token, password, QR device, private download URL, atau raw credential pada log/audit.

- Gunakan durable store agar pending approval tetap aman saat restart.
- Idempotency record dan approval record harus atomic pada transition; tidak cukup in-memory map untuk send penting.
- Perubahan repository chatstorage perlu kontrak domain, implementasi SQLite, dan wrapper sesuai AGENTS.md. Buat migration additive dengan rollback plan; jangan menghapus histori chat.
- Store identity bukan selalu user-facing device alias; perhatikan GoWA device-scoped chat storage key.

## 10. Authorization dan approval enforcement

### 10.1 Entry points yang harus dianalisis

- MCP send (langsung dan scheduled).
- MCP schedule pause/resume/cancel.
- MCP group/message/app/admin sub-actions.
- REST send untuk Dashboard dan Bridge.
- REST send dan scheduler yang mungkin dipakai UI GoWA.
- Scheduled worker yang memanggil underlying send usecase.
- Device resolution melalui explicit device_id, X-Device-Id, session defaults, dan storage context.

**Kunci desain:** check pada trust boundary yang sesuai. Jangan memblokir secara membabi buta semua pesan grup REST existing jika requirement approval hanya untuk direct AI agent; tetapi jangan memberi agent credential REST yang dapat bypass MCP approval. Gunakan immutable trusted auth context (bukan client-provided source label) untuk memilih policy.

### 10.2 Approval invariants

- Request PENDING_APPROVAL tidak memanggil whatsapp.SendMessage.
- Approver adalah identitas manusia berwenang, bukan agent requester.
- Approval terikat snapshot device/JID/payload/media/schedule/occurrence.
- Payload berubah => approval invalid.
- Approval expired atau absent => no send, termasuk setelah restart.
- Tidak ada replay yang membuat pengiriman ganda.
- Sedikitnya per-request audit siapa meminta, siapa menyetujui, kapan dan untuk grup apa.
- Admin dapat menolak dan mematikan integration/agent tanpa disconnect WhatsApp device.

## 11. Media security dan attachment handling

### URL-based media

GoWA sudah menerima URL pada tool MCP. Perlu hardening server-side:

- Skema terbatas ke HTTPS/HTTP yang diizinkan dan URL parsing ketat.
- Blok localhost, private/link-local/metadata IP ranges, IPv6 special ranges, raw IP bypass, redirect internal, DNS rebinding.
- Revalidate DNS+target socket pada setiap redirect dan koneksi; gunakan bounded HTTP client.
- Batasi ukuran, MIME berdasarkan magic bytes, durasi dan jumlah file, dan validasi format media.
- Pre-fetch/lock immutable media untuk approval grup agar konten remote tidak berubah setelah review.
- Return error tanpa membocorkan internal network/credentials.

### Client-upload media

User meminta file langsung dari chat Claude/Hermes **dan** via URL. Kemampuan client membawa attachment binary tidak boleh diasumsikan otomatis. Lakukan compatibility matrix dan live test untuk Claude Web, Claude Desktop/Code, dan Hermes version yang dipakai. Jika tidak ada attachment-to-MCP path, sediakan private upload page/service adapter yang memberi short-lived media_ref. File harus:

1. Dibuat dan diakses dengan per-principal/device authorization.
2. Memiliki checksum/hash konten, MIME/size validation, TTL dan cleanup.
3. Tidak dapat diambil agent lain atau lewat public static path.
4. Tidak mengembalikan absolute local filesystem path.
5. Diuji untuk oversized file, malformed MIME, path traversal, expired ref, dan replay.

## 12. Scheduler ownership dan compatibility

GoWA ScheduledSend existing menjadi source of truth **hanya** untuk direct GoWA/MCP jobs. Dashboard wa_outbox tetap owner tagihan, reminder kelas, guru dan CRM follow-up.

- REST request immediate dari Dashboard tidak membuat GoWA ScheduledSend kedua.
- GoWA scheduler harus mengetahui policy source dan check ulang sebelum due send.
- Recurrence group membutuhkan approval per occurrence; pending approval menahan occurrence dan memberi status yang jelas.
- Pause/resume/cancel juga terikat principal dan device scope.
- Timezone UTC internal, IANA timezone eksplisit pada input/output/preview; business send window tetap kebijakan Dashboard.
- Jika schedule sudah dibuat sebelum permission agent dicabut, due send harus ditolak atau ditahan.
- Pastikan scheduler restart/recovery tidak mengeksekusi pending/rejected approval atau menggandakan occurrence.

## 13. UI GoWA dan strategi release

UI GoWA saat audit adalah [aldinokemal/gowa-ui](https://github.com/aldinokemal/gowa-ui), diambil sebagai packaged HTML release dengan cache/runtime update. Oleh karena itu penambahan halaman baru membutuhkan **keputusan ownership UI** lebih dahulu, bukan hanya mengedit file dalam repo backend GoWA.

### Proposed UI

- **AI Activity:** asal Claude/Hermes/Dashboard/Bridge/UI, principal, device, recipient type, pesan/media summary yang diizinkan, request ID, status, schedule, WA message ID dan error.
- **Approvals:** daftar PENDING_APPROVAL, detail request, preview immutable, expiry, tombol Approve/Reject untuk role manusia yang memiliki izin.
- **Integrations/Principals:** daftar client, grant device/action, token status, revoke, kill switch dan audit summary.
- **Existing Chats dan Scheduled:** tetap berjalan, dengan filter/source badge bila metadata sudah tersedia. Halaman Chats hanya dapat menjamin pesan yang tersimpan oleh chat storage; async storage failure bisa membuat pesan sukses belum tampil.

### UI build decision gate

Sebelum implementasi pilih satu sumber UI yang dikelola organisasi, misalnya fork privat/owned release yang dipin oleh konfigurasi APP_UI_REPO/APP_UI_ASSET_SHA256; jangan bergantung pada upstream auto-update untuk custom halaman. Verifikasi runtime yang sebenarnya menggunakan UI release mana. Perubahan repo UI dapat membutuhkan PR tersendiri, sehingga backend GoWA tidak boleh diklaim selesai UI-nya sebelum release/asset test PASS.

## 14. Keamanan, risiko dan observability

| Risiko | Pencegahan dan evidence |
|---|---|
| CS context split | Seluruh business CS outbound melalui trusted Bridge identity, tests Bridge event dan prompt context |
| Device escalation | Scope enforced pada token, tool handler, resolver dan dispatch; negative override tests |
| Approval bypass | Agent tidak punya credential REST direct untuk escape; group MCP direct pending until human-approved |
| Duplicate send | Durable idempotency + occurrence key, no blind retry untuk ambiguous send |
| Scheduler race | Transactional status transitions, policy recheck sebelum due |
| Data leak | Scoped chat/contact read, masked audit, separate Dashboard PII redaction |
| SSRF media | Network-bound fetch validation, redirect/DNS rechecks, egress rules |
| Compromised token | Short access expiry, revocation, client/device scope, rate limits, audit |
| Divergent business state | GoWA tidak menentukan sudah/belum lunas dan jadwal kelas, Dashboard mengecek sendiri |
| UI release drift | Owned/pinned UI build dan smoke test versi yang terpasang |
| Ban/rate-limit WhatsApp | Per-device cap, target policy, opt-out bisnis, kill switch, pemantauan volume |

Observability minimum: MCP calls by principal/action; denied auth, pending approval age, approval outcomes, direct send SENT/FAILED/UNCERTAIN, retry/dedup count, scheduled jobs due/paused, device connection, media failed uploads/fetches, async chatstorage errors, dan Bridge event health. HTTP 200 maupun tool result sukses tidak boleh dianggap pesan dibaca.

## 15. Acceptance criteria GoWA

| ID | Test | PASS evidence |
|---|---|---|
| G-AC-01 | Claude remote MCP | Connect dengan scoped principal dan tool list yang sesuai |
| G-AC-02 | Hermes remote MCP | Profile terpisah dari CS, auth dan scope sesuai grant |
| G-AC-03 | MCP device override | Mengubah device_id/X-Device-Id tidak dapat akses device di luar grant |
| G-AC-04 | MCP sub-action | whatsapp_app.status allowed tidak memberi logout; group.info tidak memberi remove_participants |
| G-AC-05 | OAuth restriction regression | Restricted Basic account tidak berubah menjadi global MCP access |
| G-AC-06 | Personal text send | Satu pesan uji, WA message ID + principal origin audit |
| G-AC-07 | Personal image/video/file | URL media valid; source result dan storage benar |
| G-AC-08 | Secure upload | Client upload/fallback diizinkan, ref scoped, TTL cleanup PASS |
| G-AC-09 | Group pre-approval | whatsapp.SendMessage tidak dipanggil sebelum approval |
| G-AC-10 | Group reject/expiry | Tidak terkirim setelah reject/expiry, termasuk restart |
| G-AC-11 | Group content mutation | Device/JID/message/caption/media/schedule berubah invalidates approval |
| G-AC-12 | Group recurring | Tiap occurrence tanpa approval tidak terkirim |
| G-AC-13 | Agent self-approval | Tidak bisa approve via tool/token agent lain yang dikendalikan agent sama |
| G-AC-14 | Dashboard REST non-CS | Scoped credential keuangan/admin-kelas/koord-guru mengirim ke device yang tepat |
| G-AC-15 | Bridge CS regression | Existing bridge tetap kirim cs-ai dan event/konteks CS tidak terganggu |
| G-AC-16 | CS direct bypass | MCP operator dan Dashboard non-CS tidak bisa raw send dari cs-ai |
| G-AC-17 | No double scheduling | Business Dashboard job tidak menghasilkan independent GoWA scheduled job |
| G-AC-18 | Replay/idempotency | Repeated request sama hanya satu send |
| G-AC-19 | Timeout uncertainty | UNCERTAIN tidak auto retry tanpa rekonsiliasi |
| G-AC-20 | SSRF/media negative | Private network/redirect, over-size dan expired media ditolak |
| G-AC-21 | UI source/release | Owned UI package menampilkan AI Activity dan Approvals; existing Chats/Scheduled tidak regresi |
| G-AC-22 | Audit and revocation | Per-action audit, revoke token dan kill switch bekerja tanpa secret leak |
| G-AC-23 | Multi-device test | Beberapa device aktif dengan scope berbeda tetap terisolasi |
| G-AC-24 | Deployment smoke | **Hanya** setelah approval deployment, uji ke nomor dan grup yang dikelola tim |

**Klasifikasi:** PASS hanya dengan direct test evidence; NOT VERIFIED bila belum diuji. Build PASS tidak membuktikan deployment dan chatstorage updated; HTTP 200 tidak membuktikan delivery/read.

## 16. Rencana implementasi bertahap

| Phase | Perubahan utama | Scope implementasi | Verification gate |
|---|---|---|---|
| 0 — Focused audit | Source/route inventory minimal, auth/CS role/REST mapping, storage & scheduler, UI repo ownership, client attachment capability | docs + targeted inspection | File/symbol map, kontrak existing, acceptance baseline |
| 1 — Policy foundation | Auth principal, grants device/tool/action/target, CS boundary | src/cmd, src/ui/mcp, auth services | Negative auth/device tests |
| 2 — Direct dispatch/approval | Pending group approvals, dispatcher/idempotency, send status/audit | MCP/usecase/storage | Approval + replay + restart tests |
| 3 — Scheduler safe | Recheck permissions and per-occurrence approval, no double schedule | scheduler/storage | Cron/recurrence/permission revoke tests |
| 4 — Media | URL security + attachment staging/fallback | send and media services | SSRF/media edge tests |
| 5 — UI | GoWA UI owned build, AI Activity, Approvals and principal administration | UI fork/release + backend APIs | Browser E2E + packaging tests |
| 6 — Cross-repo integration | Dashboard non-CS REST, CS Bridge regression, source metadata, status reconciliation | GoWA + Dashboard | Integration suite with safe test devices |
| 7 — Production | Backup, staged rollout, alerts, smoke, rollback | Ops | **Separate approval required** |

### Engineering rules

- Utamakan reuse shared GoWA usecases; jangan membuat engine pengiriman paralel.
- Transport parsing pada ui/, orchestration usecase/, DTO/interface domains/, SQLite infrastructure.
- Jika repository interfaces berubah, update domain chatstorage interface, SQLite implementation dan WhatsApp wrapper sesuai AGENTS.md.
- Hindari refactor unrelated, generic dependency upgrade, dan modifikasi existing bridge contract tanpa PR lintas repo.
- Satu PR terarah per fase; verifikasi syntax/static → targeted tests → integration → build → production smoke hanya bila deploy.
- Untuk perubahan Markdown saja: review diff dan tautan cukup, tidak perlu menjalankan app atau menghubungkan WhatsApp real.

### Rollback

Feature gate direct agent send, group approval dan custom UI terpisah; ketika rollback, hentikan jalur baru tanpa memutus CS Bridge. Backup metadata approval/audit sebelum migrasi. Jangan hapus sejarah atau mengirim ulang pesan status UNCERTAIN. Rollback kode tidak membatalkan pesan WhatsApp yang sudah benar-benar terkirim.

## 17. Dependensi dan checkpoint lintas repository

**GoWA bergantung pada Dashboard untuk:** mapping device bisnis, status lead/tagihan/kelas/guru, approval bisnis, scheduler bisnis dan outbox. Rujukan normatif: [PRD Dashboard](https://github.com/dinastran/ruang-sanad/blob/main/docs/prd/prd-integrasi-whatsapp-dashboard-gowa-mcp.md).

**Dashboard bergantung pada GoWA untuk:** REST transport device-scoped, WA message ID, penanganan error/uncertainty, audit transport dan rekonsiliasi. Kedua sistem tidak berbagi kredensial MCP dan tidak menyalin business logic.

**Bridge CS tetap memiliki kontrak independen:** [PRD Bridge](./prd-cs-bridge-outbound.md). D5/D6/D7 pada PRD Dashboard CRM tidak dibatalkan oleh PRD GoWA baru.

### Current checkpoint

- Objective: PRD khusus GoWA yang kompatibel dengan PRD Dashboard, MCP Claude/Hermes dan CS Bridge.
- Evidence: baseline source dari branch main; aplikasi, UI release aktual dan runtime belum diverifikasi.
- Completed: scope GoWA, matrix permission, routing, approval, scheduling, media, audit, 24 FR, 24 AC, ownership, fase implementasi.
- Changes: dokumentasi ini saja pada branch docs.
- Locked: semua device terkelola dengan policy scoped; group sends dari agent butuh approval; kedua metode media; CS bridge tidak bypass; pemilik scheduler tunggal.
- Open: OD-01 kebijakan auto-send business non-CS; approver grup; ownership UI build; operational cap/retention.
- Next action: review kedua PRD sebagai kontrak silang; lakukan Phase 0 focused technical audit sebelum coding; merge dan deployment tetap dua approval terpisah.
