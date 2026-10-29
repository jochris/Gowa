<div align="center">

# Gowa

Modular and high-performance WhatsApp bot written in Go.

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev/)
[![Library](https://img.shields.io/badge/Library-whatsmeow-25D366?style=flat-square&logo=whatsapp&logoColor=white)](https://github.com/tulir/whatsmeow)
[![Database](https://img.shields.io/badge/Database-SQLite%20(Pure%20Go)-003B57?style=flat-square&logo=sqlite&logoColor=white)](https://modernc.org/sqlite)
[![Platform](https://img.shields.io/badge/Platform-Linux%20%7C%20macOS%20%7C%20Windows-555555?style=flat-square&logo=linux&logoColor=white)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue?style=flat-square)](LICENSE)

</div>

---

## Daftar Isi

- [Tentang Proyek](#tentang-proyek)
- [Fitur Utama](#fitur-utama)
- [Struktur Direktori](#struktur-direktori)
- [Daftar Perintah](#daftar-perintah)
- [Panduan Instalasi](#panduan-instalasi)
- [Konfigurasi](#konfigurasi)
- [Pengembangan](#pengembangan)
- [Lisensi](#lisensi)

---

## Tentang Proyek

**Gowa** adalah bot WhatsApp berbasis Go yang dirancang dengan fokus pada efisiensi resource, stabilitas eksekusi di grup dengan volume chat tinggi, serta kemudahan modularitas kode. Menggunakan library [whatsmeow](https://github.com/tulir/whatsmeow) sebagai antarmuka protokol WhatsApp dan driver SQLite murni (`modernc.org/sqlite`) yang tidak memerlukan toolchain CGO.

Respon perintah menggunakan format teks minimalis tanpa dekorasi berlebihan, memudahkan pembacaan pesan dan meminimalkan ukuran payload jaringan.

---

## Fitur Utama

- **Autentikasi Pairing Code**: Mendukung login akun menggunakan kode pairing 8-digit ke nomor telepon tanpa memerlukan pemindaian QR code.
- **Arsitektur Modular**: Perintah diorganisir dalam paket-paket terpisah (`cmd/<kategori>/`) dan didaftarkan otomatis melalui mekanisme `init()`.
- **Manajemen Beban & Concurrency**:
  - Filter prefix berbasis byte check untuk mengabaikan chat non-perintah secara instan.
  - Bounded worker pool untuk membatasi jumlah goroutine konkruen saat lalu lintas pesan tinggi.
  - Pre-warming cache informasi grup dan daftar anggota saat inisialisasi awal.
- **Penyimpanan SQLite Pure-Go**: Berjalan tanpa dependensi CGO dengan konfigurasi Write-Ahead Logging (WAL) dan in-memory page cache.
- **Mode Operasi (Self / Public)**: Bot dapat diatur untuk melayani publik atau hanya merespon nomor pemilik (owner).
- **Eksekusi Perintah Shell**: Mendukung eksekusi perintah terminal sistem secara langsung oleh owner terotorisasi.
- **Diagnostik Sistem**: Menyediakan pemantauan real-time status memori proses (RSS, Go Alloc, GC) dan informasi perangkat server.

---

## Struktur Direktori

```text
.
├── cmd/                          # Modul perintah aplikasi
│   ├── general/                  # Perintah umum (getpp, menu, ping)
│   ├── owner/                    # Perintah khusus owner (exec, mode)
│   ├── system/                   # Perintah diagnosa sistem (os, rss)
│   └── loader.go                 # Loader registrasi paket perintah
├── internal/                     # Komponen internal bot
│   ├── bot/                      # Engine bot, event handler, dan worker pool
│   ├── command/                  # Struct context, cache, dan command registry
│   ├── storage/                  # Konfigurasi JSON dan database SQLite
│   └── util/                     # Logger dan diagnostik sistem
├── config.json                   # Berkas konfigurasi utama
├── go.mod                        # Definisi modul Go
├── go.sum                        # Checksum dependensi
└── main.go                       # Entry point aplikasi
```

---

## Daftar Perintah

| Kategori | Perintah | Alias | Akses | Deskripsi |
| :--- | :--- | :--- | :---: | :--- |
| **General** | `.getpp` | `.pp`, `.avatar` | Publik | Mengambil foto profil WhatsApp via reply, tag, nomor, atau profil sendiri |
| **General** | `.menu` | `.help` | Publik | Menampilkan daftar seluruh perintah yang tersedia |
| **General** | `.ping` | `.p` | Publik | Menampilkan latensi dan durasi aktif bot |
| **General** | Reaksi `❌` | `delete_reaction` | Publik / Admin | Menghapus pesan bot atau pesan grup (admin/owner) via reaksi emoji |
| **System** | `.os` | `.infoserver`, `.server` | Publik | Menampilkan spesifikasi hardware, OS, CPU load, dan uptime |
| **System** | `.rss` | `.ram`, `.mem` | Publik | Menampilkan alokasi memori runtime Go dan RSS proses |
| **Owner** | `.self/public` | `.self`, `.public` | Owner | Mengubah mode operasi bot (Self / Public) |
| **Owner** | `$ <command>` | - | Owner | Menjalankan perintah shell bash pada server |

---

## Panduan Instalasi

### Prasyarat

- Go versi 1.21 atau yang lebih baru.
- Sistem operasi Linux, macOS, atau Windows.
- Akses terminal atau shell.

### 1. Kloning Repositori

```bash
git clone https://github.com/jochris/Gowa.git
cd Gowa
```

### 2. Unduh Dependensi & Kompilasi

```bash
go mod tidy
go build -o gowa .
```

### 3. Tautkan Akun WhatsApp

Jalankan binary dengan menyertakan nomor telepon yang ingin digunakan (gunakan format kode negara tanpa simbol `+`):

```bash
./gowa -phone 6281234567890
```

Terminal akan menampilkan kode pairing 8 digit. Masukkan kode tersebut pada aplikasi WhatsApp:
1. Buka **Pengaturan** WhatsApp.
2. Pilih **Perangkat Tertaut** > **Tautkan Perangkat**.
3. Pilih opsi **Tautkan dengan nomor telepon saja**.
4. Masukkan kode 8 digit yang muncul di terminal.

Setelah berhasil terhubung, sesi akan tersimpan secara otomatis di `data/gowa.db`.

### 4. Menjalankan di Latar Belakang

Untuk menjalankan aplikasi secara terus-menerus sebagai background service pada Linux:

```bash
nohup ./gowa > gowa.log 2>&1 &
```

---

## Konfigurasi

Aplikasi membaca konfigurasi dari berkas `config.json`. Jika berkas belum tersedia, konfigurasi default akan dibuat secara otomatis saat pertama kali dijalankan.

Contoh `config.json`:

```json
{
  "bot_name": "Gowa",
  "prefix": ".",
  "db_path": "data/gowa.db",
  "phone_number": "6281234567890",
  "log_level": "INFO",
  "is_public": true,
  "owner_numbers": [
    "6281234567890"
  ]
}
```

### Parameter Konfigurasi

| Parameter | Tipe Data | Default | Keterangan |
| :--- | :--- | :--- | :--- |
| `bot_name` | String | `"Gowa"` | Nama identitas bot |
| `prefix` | String | `"."` | Karakter awalan untuk pemanggilan perintah |
| `db_path` | String | `"data/gowa.db"` | Path penyimpanan berkas database SQLite |
| `phone_number` | String | `""` | Nomor telepon untuk inisialisasi pairing code |
| `log_level` | String | `"INFO"` | Tingkat logging terminal (`DEBUG`, `INFO`, `WARN`, `ERROR`) |
| `is_public` | Boolean | `true` | `true` untuk merespon semua user, `false` untuk mode owner-only |
| `owner_numbers` | Array | `[]` | Daftar nomor telepon yang memiliki hak akses perintah owner |

---

## Pengembangan

### Menambahkan Perintah Baru

Untuk menambahkan perintah baru, buat berkas Go pada direktori kategori yang sesuai (`cmd/<kategori>/`).

Contoh pembuatan `cmd/general/uptime.go`:

```go
package general

import (
	"fmt"
	"time"

	"gowa/internal/command"
)

var startTime = time.Now()

func init() {
	command.Register(&command.Command{
		Name:    "uptime",
		Cmd:     "uptime",
		Categ:   "general",
		Desc:    "Menampilkan durasi aktif aplikasi",
		Alias:   []string{"up"},
		Execute: handleUptime,
	})
}

func handleUptime(ctx *command.Context) error {
	duration := time.Since(startTime).Truncate(time.Second)
	return ctx.Reply(fmt.Sprintf("Uptime: %s", duration.String()))
}
```

Jika menambahkan folder kategori baru (misalnya `cmd/tools/`), tambahkan impor modul pada [`cmd/loader.go`](cmd/loader.go):

```go
import (
	_ "gowa/cmd/general"
	_ "gowa/cmd/owner"
	_ "gowa/cmd/system"
	_ "gowa/cmd/tools"
)
```

Perintah baru akan terdaftar secara otomatis ke dalam registry dan tampil pada daftar `.menu`.

---

## Lisensi

Proyek ini dilisensikan di bawah lisensi [MIT](LICENSE).
