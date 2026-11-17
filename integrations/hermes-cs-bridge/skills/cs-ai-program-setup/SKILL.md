---
name: cs-ai-program-setup
description: Ubah landing page (LP), brosur, poster, caption, atau catatan program baru Ruang Sanad menjadi isian siap tempel untuk halaman /cs-ai (CS AI WhatsApp, hermes-cs-bridge) — kode paket & pembayaran, Knowledge Produk, dan blok Instruksi CS — plus checklist review dan pesan uji. Gunakan skill ini setiap kali user mengirim link LP/post.ruangsanad.com, brosur, atau teks program/kelas/event/kajian/daurah baru dan ingin CS AI bisa menjawab dan menerima pendaftarannya, termasuk kalimat seperti "buatkan paket dan knowledge dari LP ini", "masukkan program ini ke CS AI", "setup cs-ai untuk program baru", "update knowledge produk", atau "ada program baru, siapkan untuk CS", walaupun user tidak menyebut nama skill ini.
---

# Setup Program Baru untuk CS AI Ruang Sanad

Tugas: dari satu sumber program (LP, brosur, teks), hasilkan isian yang bisa langsung
ditempel admin ke halaman **/cs-ai** tanpa diedit ulang, dan tanpa merusak perilaku CS
yang sudah berjalan (Tahsin Reguler, paket `1x`/`2x`).

Kenapa ini butuh hati-hati: CS AI membaca isian ini sebagai *sumber kebenaran*. Fakta
yang salah atau klaim yang dikarang akan diulang ke ratusan calon santri. Jadi lebih baik
menulis "belum ada di sumber, konfirmasi admin" daripada mengisi celah dengan tebakan.

Sebelum menulis output, baca `references/cs-ai-fields.md` (struktur kolom, batas karakter,
cara CS AI merakit prompt, dan aturan tetap yang tidak bisa diubah). Untuk contoh hasil
lengkap, lihat `references/contoh-sepekan-al-fatihah.md`.

## Langkah kerja

### 1. Ambil seluruh isi sumber

- Link: buka dan baca seluruh halaman (LP Ruang Sanad sering panjang; pastikan bagian
  jadwal, harga, FAQ, syarat, dan tombol daftar ikut terbaca). Kalau halaman gagal dibuka,
  minta user menempelkan teksnya — jangan menyusun dari ingatan atau dari judul URL.
- Gambar/PDF/brosur: baca teksnya; sebut bagian yang tidak terbaca.

### 2. Susun lembar fakta (internal, lalu tampilkan ringkas)

Kumpulkan hanya yang tertulis di sumber. Untuk tiap butir, bedakan **fakta tertulis**
dari **kesimpulan Anda**.

Checklist yang dicari:
- Nama program, penyelenggara, pengajar, angkatan/batch
- Sasaran peserta dan prasyarat (mis. "sudah bisa membaca", ikhwan/akhwat)
- Format (Zoom/offline), jumlah sesi, tanggal & jam tiap sesi, durasi, ukuran kelas
- Materi/kurikulum, fasilitas yang didapat
- Harga per opsi (catat istilahnya: "infak", "biaya", "SPP"), diskon/early bird + batasnya
- Tanggal tutup pendaftaran, kuota (hanya kalau tertulis)
- Syarat sertifikat/ijazah/sanad, kebijakan izin/tidak hadir
- Alur daftar (data apa yang diminta), nomor/teks prefill tombol WhatsApp
- Program lanjutan yang ditawarkan
- FAQ, klaim (sanad, testimoni, angka capaian), dalil yang dikutip

Lalu **cek konsistensi sumber**, karena brosur sering salah hitung dan CS AI akan
mengulang kesalahannya: jumlah sesi vs hari & rentang tanggal di kalender, hari vs
tanggal (mis. "Sabtu, 21 Nov"), label musim ("Ramadhan Edition") vs tanggal sebenarnya,
batas early bird vs tanggal tutup, harga di beberapa bagian halaman. Hitung, jangan
mengira-ngira. Temuan masuk ke bagian 0 dan checklist; di instruksi, suruh AI tidak
menyebut detail yang bertentangan dan mengalihkannya ke admin.

### 3. Tanyakan yang kritis sebelum menulis

Tanyakan ke user (pakai tool tanya-jawab bila tersedia, maksimal 4 pertanyaan sekaligus)
hanya bila salah satu ini kosong atau ambigu, karena tanpanya output akan salah di WA:
- Harga tiap opsi, atau apakah ada lebih dari satu opsi/tier
- Tanggal tutup pendaftaran atau tanggal pelaksanaan
- Kode paket yang sudah terpakai di /cs-ai selain `1x` dan `2x` (agar tidak bentrok)
- Data apa yang harus dikumpulkan saat daftar, bila sumber tidak menyebutnya

Bila tidak ada cara bertanya (mis. dijalankan otomatis), jangan berhenti: tulis
pertanyaannya di bagian paling atas output dengan judul "Perlu konfirmasi admin",
lanjutkan dengan asumsi paling aman, dan tandai setiap asumsi dengan ⚠️ di tempat
dipakainya.

Hal yang tidak kritis (testimoni, profil guru lengkap, rekening) tidak perlu ditanyakan;
cukup dicatat sebagai "tidak ada di sumber". Rekening memang berasal dari teks
pembayaran global di /cs-ai, bukan dari skill ini.

### 4. Tulis output dengan format tetap di bawah

## Format output

Tulis dalam Bahasa Indonesia. Setiap nilai yang akan ditempel diletakkan di dalam blok
kode (```) tersendiri supaya admin cukup klik salin. Gunakan judul persis seperti ini:

```
# Setup CS AI: <Nama Program>

## 0. Ringkasan sumber
- Sumber: <URL/berkas>, dibaca <tanggal>
- Fakta inti: <5–8 poin: tanggal, format, harga, syarat, tutup daftar>
- Tidak ditemukan di sumber: <daftar>
- Inkonsistensi/temuan saya (bukan fakta sumber): <daftar, atau "tidak ada">
- Bahasa promosi yang sengaja tidak dipakai: <mis. "kuota terbatas", "ribuan alumni">
- Asumsi yang saya buat: <daftar ⚠️, atau "tidak ada">

## 1. Tab "Paket & Pembayaran" → + Tambah paket
(satu sub-bagian per opsi harga)
### Paket: <nama>
Kode → ```<kode>```
Nama paket → ```<nama>```
Harga → ```<RpX.XXX>```
Keterangan → ```<keterangan>```
Penanda yang akan dipakai AI: [[REKENING:<kode>]]

## 2. Tab "Knowledge Produk" → + Tambah produk
Kode unik produk → ```...```
Nama program → ```...```
Cocok untuk siapa? → ```...```
Situasi/kebutuhan calon santri → ```...```
Big Why: nilai Al-Qur'an dan alasan personal → ```...```
Manfaat yang dapat dijelaskan → ```...```
Mekanisme/proses belajar → ```...```
Fakta/bukti terverifikasi → ```...```
Batasan klaim dan hal yang perlu konfirmasi admin → ```...```
Kode paket terkait → ```...```
Status → biarkan **nonaktif (draft)** sampai checklist di bagian 4 selesai.

## 3. Tab "Instruksi CS" → tempel di PALING BAWAH instruksi yang ada
```<blok instruksi program>```

## 4. Checklist review sebelum mengaktifkan
## 5. Catatan teknis & risiko
## 6. Pesan uji setelah aktif
```

### Aturan isi tiap bagian

**Paket (bagian 1)**
- Kode: huruf kecil/angka/`-`/`_`, maks 32 karakter, diawali huruf/angka, unik. Pakai
  singkatan program, bukan generik: `fatihah-nov26`, bukan `paket1` atau `1x` (bentrok).
  Sertakan bulan/tahun bila program per angkatan, supaya angkatan berikutnya tidak menimpa.
- Nama paket muncul di pesan rekening sebagai `{PAKET}`, jadi tulis nama yang jelas bagi
  calon peserta: nama program + angkatan/tanggal.
- Harga persis format `Rp49.000` (titik ribuan, tanpa spasi, tanpa ",-"). Harga ini
  dikirim apa adanya sebagai `{NOMINAL}`.
- Keterangan: isi singkat pembeda opsi (jumlah sesi, durasi, istilah "infak"), ≤ 1 baris.
  Pakai istilah biaya dari sumber; bila sumber tidak menyebut, pakai "biaya".
- Diskon/early bird yang punya batas waktu: buat paket terpisah (kode berakhiran mis.
  `-eb`) dan jelaskan di instruksi kapan masing-masing berlaku: AI memilih paket menurut
  "Waktu sekarang" saat mengirim `[[REKENING]]`; kasus perbatasan (info rekening early
  bird sudah dikirim tetapi transfer setelah batas) dialihkan ke admin, bukan diputuskan
  AI. Di catatan admin, sarankan menghapus paket early bird setelah batasnya, karena
  paket tidak punya tombol nonaktif. Jangan mengarang potongan harga yang tidak tertulis.
- Bila harga program sama dengan paket lain (mis. `1x` juga Rp149.000), tegaskan di
  instruksi bahwa peserta program ini hanya ditagih dengan kode paket program ini.

**Knowledge Produk (bagian 2)** — baca rincian kolom di `references/cs-ai-fields.md`.
- Tiap kolom ≤ 3000 karakter; usahakan 300–900 karakter agar prompt tidak bengkak
  (total semua produk dibatasi 35.000 karakter).
- Sasaran, Big Why, Manfaat, Metode wajib terisi — /cs-ai menolak mengaktifkan produk
  tanpa empat kolom ini.
- Big Why harus customer-centric dan tidak menakut-nakuti. Jika LP mengutip dalil
  (mis. "tidak ada salat bagi yang tidak membaca Al-Fatihah"), boleh dirujuk sebagai
  nilai keutamaan, tetapi tulis eksplisit bahwa AI tidak boleh memakainya untuk
  menyiratkan salat seseorang tidak sah atau berdosa karena bacaannya. Aturan tetap
  CS AI melarang itu, dan klaim hukum fikih dialihkan ke admin.
- Fakta/bukti: tulis "menurut LP" untuk klaim yang belum diverifikasi lembaga (sanad
  berantai, jumlah alumni, testimoni). Jangan menambah bukti yang tidak ada.
- Batasan: tulis apa yang *tidak boleh dijanjikan* (lulus/ijazah otomatis, hasil
  tertentu) dan apa yang harus dikonfirmasi admin (penempatan kelas, link Zoom, izin
  tidak hadir, pertanyaan fikih).

**Blok Instruksi (bagian 3)** — ini bagian yang paling menentukan perilaku di WA.
Instruksi CS adalah satu teks global; blok ini ditambahkan, bukan mengganti. Isi blok:
1. Judul `## Program: <Nama> (<periode>)` dan satu kalimat kapan blok ini berlaku.
2. **Cara mengenali peminat**: kata kunci dan teks prefill tombol WA dari LP, supaya AI
   tidak menawarkan Tahsin Reguler kepada orang yang datang dari LP program ini.
3. **Fakta singkat yang boleh disebut**: tanggal/jam sesi, format, harga, fasilitas,
   syarat sertifikat. Tulis ringkas; detail ada di Knowledge Produk.
4. **Jadwal tetap**: jika program punya tanggal sesi yang sudah pasti, nyatakan bahwa
   tanggal itu fakta program, bukan pilihan dari "Jadwal kelas yang dibuka"; AI tidak
   menanyakan level, tidak mencocokkan 1×/2× sepekan, dan tidak menulis `[[JADWAL:...]]`
   untuk program ini. (Alasannya: aturan jadwal tetap di CS AI dirancang untuk kelas
   reguler; tanpa penegasan ini AI bisa menolak menyebut tanggal sesi.)
5. **Alur daftar khusus**: AI tidak memakai `[[FORM]]` (form global berisi pertanyaan
   kelas Reguler). AI menanyakan langsung data yang diminta sumber (mis. nama dan
   ikhwan/akhwat), maksimal dua pertanyaan per pesan, merangkum, minta konfirmasi, lalu
   menulis `[[REKENING:<kode>]]`. Setelah bukti transfer, ikuti aturan pembayaran
   yang sudah ada (diteruskan ke admin).
6. **Batas waktu**: tulis tanggal-jam tutup dengan zona WIB. Instruksikan AI
   membandingkan dengan "Waktu sekarang" di konteks chat; setelah lewat, jangan kirim
   `[[REKENING]]` untuk paket ini, sampaikan pendaftaran sudah ditutup, tawarkan
   dicatat untuk angkatan berikutnya atau program lain yang relevan. Tanggal tutup asli
   dari sumber boleh disebut sebagai fakta; dilarang menambah urgensi ("tinggal sedikit",
   "segera sebelum kehabisan") yang tidak tertulis.
7. **Jawaban untuk pertanyaan yang sering muncul**, diambil dari FAQ sumber, dalam gaya
   WA Ruang Sanad (lihat `references/cs-ai-fields.md` bagian gaya).
8. **Kapan handoff** (`[[HANDOFF: alasan | ringkasan]]`): pertanyaan fikih, permintaan
   keringanan/gratis, izin tidak hadir, belum bisa membaca huruf hijaiyah bila sumber
   mengarahkan ke admin, hal di luar fakta.
9. **Hubungan dengan program lain**: bila sumber menyebut program lanjutan (mis. Kelas
   Tahsin Reguler), jelaskan kapan boleh disebut — biasanya hanya bila ditanya, tanpa
   menggeser fokus dari program yang diminati.
10. **Setelah program selesai**: satu baris pengingat untuk admin bahwa blok ini, paket,
   dan knowledge dinonaktifkan/dihapus setelah tanggal terakhir.

Panjang blok: target 1.500–4.000 karakter; sampai ±6.000 masih wajar untuk program
dengan beberapa tier harga. Ini target, bukan batas keras — batas kerasnya total
instruksi 40.000 karakter, dan setiap karakter dibaca AI di setiap giliran chat, jadi
pangkas pengulangan dari Knowledge Produk sebelum memangkas aturan.

**Checklist (bagian 4)**: butir yang benar-benar perlu manusia: klaim sanad/pengajar,
dalil yang dikutip, harga & tanggal, nama paket di pesan rekening, kode paket tidak
bentrok, apakah teks pembayaran global cocok dengan istilah program (mis. "infak" vs
"pendaftaran"), urutan aktivasi (simpan paket dulu → knowledge → instruksi → aktifkan).

**Catatan teknis (bagian 5)**: sebutkan secara jujur keterbatasan yang tidak bisa
diselesaikan lewat isian, misalnya: pesan rekening memakai satu teks global; galeri
gambar perlu diunggah manual bila ingin AI mengirim poster (`[[GAMBAR:<kode>]]`) —
usulkan kode dan "kapan dikirim" bila sumber punya poster; aturan tetap jadwal tetap
berlaku sehingga perilaku perlu diuji.

**Pesan uji (bagian 6)**: 4–6 pesan WhatsApp realistis untuk dicoba admin setelah aktif,
masing-masing dengan perilaku yang diharapkan. Wajib mencakup: teks prefill dari LP,
tanya harga, keberatan/ragu, pertanyaan yang harus di-handoff, dan pesan setelah tanggal
tutup (bila bisa diuji).

## Prinsip yang tidak boleh dilanggar

- Semua angka (harga, tanggal, jam, jumlah sesi, kapasitas) berasal dari sumber atau
  jawaban user. Jika tidak ada, tulis "konfirmasi admin".
- Jangan menulis nomor rekening, link Zoom, nomor WA admin, atau data pribadi di isian
  mana pun; rekening hanya berasal dari teks pembayaran global, dan AI sudah berbicara
  dari nomor CS sehingga tidak perlu nomor WA.
- Jangan menyalin gaya hiperbola LP ke instruksi. CS AI Ruang Sanad memakai gaya hangat,
  jujur, tidak menekan: tanpa janji hasil, tanpa rasa takut/dosa, tanpa kuota/urgensi palsu.
- Pertahankan istilah lembaga: santri/peserta, ikhwan/akhwat, infak bila sumber memakainya.
