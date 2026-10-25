# GoWA Web Login dengan sesi 14 hari

Status: perubahan kode pada branch `feature/gowa-ui-web-login-14d`. Belum aktif di production sebelum PR di-merge dan release di-deploy.

## Tujuan
Ganti popup HTTP Basic Auth saat membuka `https://wa.ruangsanad.com` dengan halaman login biasa (`/login`), tanpa mengubah kredensial `APP_BASIC_AUTH` dan pembatasan `APP_BASIC_AUTH_DEVICES`.

## Cara kerja
- Navigasi tanpa sesi yang valid ke `/` atau `/cs-ai` diarahkan ke `/login`, bukan menerima `WWW-Authenticate` (popup browser).
- Login memakai akun yang SUDAH tercantum di `APP_BASIC_AUTH`; tidak membuat akun atau password baru.
- Server mengeluarkan token sesi acak yang dikirim lewat cookie `__Host-gowa_session`, `Secure`, `HttpOnly`, `SameSite=Lax`, `Path=/`.
- Token disimpan dalam database SQLite hanya sebagai SHA-256 hash, bukan plaintext. Credential proof digabung dengan token sehingga sesi invalid setelah akun dihapus atau password diubah.
- **Masa sesi 14 hari tetap** dari login terakhir, bukan rolling extension pada aktivitas.
- Database sesi `storages/ui-sessions.db` berada di volume storages GoWA yang sudah persisten. Logout `/logout` lalu tombol `Keluar` menghapus sesi di server, bukan hanya browser.
- Login memiliki pembatasan 10 kegagalan per IP per 15 menit (in-memory, reset saat restart). Semua tindakan cookie berbasis sesi memeriksa origin same-origin; login/logout juga memvalidasi token CSRF.
- Basic Auth tetap berlaku pada API otomatis (Dashboard, Hermes Bridge), WebSocket query Basic lama, dan integrasi lain. OAuth untuk MCP terpisah dan tidak diubah.
- Middleware `APP_BASIC_AUTH_DEVICES` dan `/cs-ai` tetap menggunakan identitas akun yang sama untuk login cookie maupun Basic API.
- Frontend `gowa-ui` yang diunduh otomatis tidak perlu menerima atau menyimpan password baru. Saat login, informasi koneksi browser yang lama dihapus agar UI memakai same-origin cookie pada permintaan API dan WebSocket.

## Persyaratan deployment
1. Backup konfigurasi dan volume `storages` GoWA sebelum perubahan auth yang sensitif.
2. Pastikan URL publik melalui **HTTPS**: `https://wa.ruangsanad.com`. Karena cookie `Secure`, pengujian login melalui HTTP lokal biasa tidak mengirim cookie.
3. Pastikan reverse proxy mengarahkan `/login`, `/auth/login`, `/logout`, `/auth/logout`, `/cs-ai`, `/ws`, dan API lain ke service GoWA yang sama. Jika Nginx memiliki Basic Auth *sendiri*, fitur ini tidak dapat menghilangkan popup milik Nginx sampai aturan tersebut ditinjau dan dipindahkan secara aman.
4. Pertahankan `APP_BASIC_AUTH` dan `APP_BASIC_AUTH_DEVICES` tanpa memuat isinya di log, issue, atau PR.
5. Jangan melayani domain ini langsung melalui port GoWA tanpa reverse proxy/TLS publik. Jangan mengubah mode SSL Cloudflare tanpa approval khusus.
6. Setelah deployment, lakukan smoke test khusus dengan akun admin yang berwenang.

## Smoke test aman (non-destructive)
- Buka jendela incognito ke `https://wa.ruangsanad.com`; harus pindah ke `/login` **tanpa popup browser**.
- Username/password salah menampilkan pesan aman dan tidak membuat sesi; 10 percobaan salah harus dibatasi (cukup diuji di staging, bukan sengaja lockout production).
- Login benar → halaman perangkat dapat memuat data; buka `/cs-ai` dan pastikan pengaturan terbuka sesuai permission.
- Cek WhatsApp connection/WebSocket (status realtime) tanpa kredensial Basic baru dalam URL.
- Uji login ulang setelah proses GoWA restart menggunakan lingkungan staging untuk membuktikan persistensi DB.
- Uji pembatasan user `APP_BASIC_AUTH_DEVICES` untuk device lain: harus tetap ditolak.
- Klien Dashboard/Hermes yang memakai Basic Auth harus tetap dapat mengakses endpoint yang memang diizinkan.
- Buka `https://wa.ruangsanad.com/logout`, klik keluar, lalu uji URL root dan API tanpa Basic kembali ditolak. Cookie 14 hari tidak boleh tetap memberi akses setelah logout.
- Pastikan webhook yang sebelumnya publik, endpoint health, dan MCP OAuth tetap berfungsi.

## Risiko dan rollback
- Jika GoWA memakai **Cloudflare Flexible**, Secure cookie melindungi browser→Cloudflare namun koneksi Cloudflare→origin belum terenkripsi end-to-end. Perubahan mode TLS adalah keputusan production tersendiri.
- In-memory rate limiter bersifat per-instance. Deployment multipod butuh mekanisme shared rate limit sebelum dipakai untuk login publik berskala tinggi.
- Rollback cukup mengembalikan build GoWA sebelumnya; Basic Auth lama tetap tersedia. Database `ui-sessions.db` boleh dibiarkan (jangan hapus data tanpa approval).
- Sesi yang dibuat pada build baru tidak digunakan oleh build lama setelah rollback; pengguna login lagi lewat Basic Auth lama.

## Status
- Source code: menunggu review/merge.
- Go targeted verification: lihat tautan GitHub Actions di PR.
- VPS/production: NOT VERIFIED; tidak ada restart atau deployment yang dilakukan.
