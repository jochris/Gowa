<div align="center">

```text
 ██████╗  ██████╗ ██╗    ██╗ █████╗ 
██╔════╝ ██╔═══██╗██║    ██║██╔══██╗
██║  ███╗██║   ██║██║ █╗ ██║███████║
██║   ██║██║   ██║██║███╗██║██╔══██║
╚██████╔╝╚██████╔╝╚███╔███╔╝██║  ██║
 ╚═════╝  ╚═════╝  ╚══╝╚══╝ ╚═╝  ╚═╝
```

# ⚡ G O W A ⚡
### 🤖 High Performance & Modular WhatsApp Bot
*Built with Golang & Whatsmeow • Full-Text Minimalist • High-Throughput Group Engine*

---

<p align="center">
  <img src="https://img.shields.io/badge/Language-Go%201.27+-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go" />
  <img src="https://img.shields.io/badge/Library-Whatsmeow-25D366?style=for-the-badge&logo=whatsapp&logoColor=white" alt="Whatsmeow" />
  <img src="https://img.shields.io/badge/Database-SQLite%20(Pure%20Go)-003B57?style=for-the-badge&logo=sqlite&logoColor=white" alt="SQLite" />
  <img src="https://img.shields.io/badge/OS-Linux%20ARM64-FCC624?style=for-the-badge&logo=linux&logoColor=black" alt="Linux" />
  <img src="https://img.shields.io/badge/Architecture-Modular-8A2BE2?style=for-the-badge&logo=blueprint&logoColor=white" alt="Modular" />
  <img src="https://img.shields.io/badge/Latency-Ultra%20Fast-brightgreen?style=for-the-badge&logo=speedtest&logoColor=white" alt="Speed" />
  <img src="https://img.shields.io/badge/License-MIT-orange?style=for-the-badge" alt="License" />
</p>

</div>

---

## 🌟 Fitur Unggulan

<table>
  <tr>
    <td width="50%">
      <b>📱 Pairing Code Authentication</b><br>
      Koneksi instan menggunakan 8-digit kode pairing langsung ke nomor telepon tanpa scan QR.
    </td>
    <td width="50%">
      <b>💬 Full-Text Minimalist</b><br>
      Output respon teks murni yang bersih, cepat, dan to-the-point tanpa bingkai emoji berlebihan (gaya JaWa-Bot).
    </td>
  </tr>
  <tr>
    <td width="50%">
      <b>⚡ Zero-Allocation Fast Path</b><br>
      Menyaring ribuan chat obrolan biasa di grup besar dalam tempo <b>~2 nanodetik</b> tanpa memicu beban Garbage Collector (GC).
    </td>
    <td width="50%">
      <b>🔄 Group Pre-Warmer & Cache</b><br>
      Meng-cache puluhan grup dan ribuan anggota di awal koneksi. <b>Respon perintah pertama di grup langsung instan tanpa perlu kirim 2x!</b>
    </td>
  </tr>
  <tr>
    <td width="50%">
      <b>🧵 Fixed Worker Pool</b><br>
      Antrean eksekusi <i>bounded channel</i> (buffer 512) dengan 16–64 worker threads konstan mencegah <i>goroutine explosion</i>.
    </td>
    <td width="50%">
      <b>🗄️ Pure Go SQLite WAL Mode</b><br>
      Menggunakan <code>modernc.org/sqlite</code> (tanpa CGO) dengan Write-Ahead Logging dan 64MB RAM page cache.
    </td>
  </tr>
  <tr>
    <td width="50%">
      <b>👑 Self / Public Mode</b><br>
      Pengaturan fleksibel apakah bot merespon seluruh anggota grup (Public) atau hanya melayani nomor Owner (Self).
    </td>
    <td width="50%">
      <b>💻 Terminal Shell Exec</b><br>
      Eksekusi perintah terminal bash langsung dari WhatsApp dengan shortcut cepat <code>> &lt;perintah&gt;</code>.
    </td>
  </tr>
</table>

---

## 📁 Struktur Direktori Modular

Arsitektur direktori didesain bersih dan logis, di mana setiap folder memiliki minimal 2 file yang saling terintegrasi (tidak ada folder 1 file):

```text
/root/Gowa
├── cmd/                          # 📦 Modul Perintah Fitur per Kategori
│   ├── general/                  # 🌐 Fitur Umum
│   │   ├── menu.go               # Daftar perintah bot full-text
│   │   └── ping.go               # Cek latensi dan durasi aktif
│   ├── owner/                    # 👑 Fitur Khusus Owner
│   │   ├── exec.go               # Terminal bash execution
│   │   └── mode.go               # Mode toggle (self / public)
│   ├── system/                   # 🖥️ Fitur Hardware & Sistem
│   │   ├── info_server.go        # Detail OS, Processor, CPU, GPU, Load
│   │   └── ram.go                # Status memori RSS & alokasi Go
│   └── loader.go                 # Registrasi dinamis seluruh kategori
├── internal/                     # ⚙️ Komponen Inti Internal Bot
│   ├── bot/                      # 🤖 WhatsApp Engine & Concurrency
│   │   ├── bot.go                # Lifecycle, inisialisasi, & pairing
│   │   ├── event.go              # Zero-allocation event dispatcher
│   │   ├── pool.go               # Bounded Worker Pool
│   │   └── warmer.go             # Group participant pre-warmer
│   ├── command/                  # 🏷️ Command Framework & Registry
│   │   ├── cache.go              # SentCache pencegah infinite loop
│   │   ├── command.go            # Struct Command metadata & Context
│   │   └── registry.go           # Global registry & dynamic search
│   ├── storage/                  # 💾 Persistensi Data & Konfigurasi
│   │   ├── config.go             # Config loader & JSON persistence
│   │   └── sqlite.go             # SQLite WAL initialization
│   └── util/                     # 🛠️ Utilitas Sistem & Logging
│       ├── logger.go             # Clean ANSI terminal logger
│       └── system.go             # Hardware, CPU, GPU, RSS, & Uptime metrics
├── config.json                   # 📄 File konfigurasi bot
├── go.mod
├── go.sum
└── main.go                       # 🚀 Entry point aplikasi
```

---

## 📋 Daftar Perintah (Command Matrix)

| Kategori | Perintah | Alias | Izin Akses | Deskripsi |
| :--- | :--- | :--- | :---: | :--- |
| **`general`** | `.menu` | `.help` | 👥 Publik / Owner | Menampilkan daftar seluruh perintah terdaftar |
| **`general`** | `.ping` | `.p` | 👥 Publik / Owner | Cek respon latensi dan durasi aktif bot |
| **`system`** | `.infoserver`| `.info` | 👥 Publik / Owner | Spesifikasi OS, Processor/SoC, CPU, GPU, Load, & Uptime |
| **`system`** | `.ram` | `.mem` | 👥 Publik / Owner | Rincian memori RSS proses, heap Go, dan Garbage Collector |
| **`owner`** | `.self` | - | 👑 Owner | Mengubah mode bot ke Self (hanya merespon Owner) |
| **`owner`** | `.public` | - | 👑 Owner | Mengubah mode bot ke Public (merespon semua user) |
| **`owner`** | `.mode` | - | 👑 Owner | Mengecek atau beralih mode aktif bot |
| **`owner`** | `> <cmd>` | `.exec`, `$`, `sh` | 👑 Owner | Mengeksekusi perintah terminal Linux di server |

---

## 💬 Contoh Tampilan Respon (Full-Text)

<details>
<summary><b>1. Respon Perintah <code>.ping</code></b></summary>

```text
Pong
Uptime: 2h 15m 30s
Latency: 12ms
```
</details>

<details>
<summary><b>2. Respon Perintah <code>.menu</code></b></summary>

```text
[general]
• ping (.p)
• menu (.help)

[system]
• infoserver (.info)
• ram (.mem)

[owner]
• exec (>)
• self (.public)
```
</details>

<details>
<summary><b>3. Respon Perintah <code>.infoserver</code></b></summary>

```text
OS: linux (arm64)
Processor: Qualcomm Snapdragon 680 (KHAJE)
CPU: 8 cores (Cortex-A73 @ 2.40 GHz)
CPU Load: 0.12, 0.08, 0.05
GPU: Qualcomm Adreno 610
Process RSS: 28.50 MB
Go Runtime: go1.27.3
Goroutines: 38
Uptime: 2h 15m 30s
```
</details>

<details>
<summary><b>4. Respon Perintah <code>.ram</code></b></summary>

```text
Process RSS: 28.50 MB
Go Alloc: 15.20 MB
Go Total Alloc: 32.10 MB
Go Sys: 28.50 MB
GC: 14 runs
Uptime: 2h 15m 30s
```
</details>

<details>
<summary><b>5. Respon Perintah <code>.self</code> / <code>.public</code></b></summary>

```text
Mode: self
```
*atau*
```text
Mode: public
```
</details>

<details>
<summary><b>6. Respon Perintah <code>> uname -a</code> (Exec Shell)</b></summary>

```text
Linux localhost 4.19.157-perf-28692735 aarch64 GNU/Linux
```
</details>

---

## ⚙️ Berkas Konfigurasi (`config.json`)

```json
{
  "bot_name": "Gowa",
  "prefix": ".",
  "db_path": "data/gowa.db",
  "phone_number": "62895416602000",
  "log_level": "INFO",
  "is_public": true,
  "owner_numbers": [
    "62895416602000"
  ]
}
```

* `bot_name`: Nama identitas bot Anda.
* `prefix`: Karakter awalan untuk memanggil perintah (default: `.`).
* `db_path`: Direktori berkas penyimpanan sesi SQLite.
* `phone_number`: Nomor telepon untuk penautan pairing code (tanpa tanda `+`).
* `is_public`: `true` untuk mode publik, `false` untuk mode self-only.
* `owner_numbers`: Daftar nomor telepon yang memiliki wewenang fitur Owner.

---

## 🚀 Panduan Menjalankan

### 1. Kompilasi Binary
```bash
go build -o gowa .
```

### 2. Jalankan dan Tautkan Akun (Pairing Code)
```bash
./gowa -phone 62895416602000
```
1. Masukkan kode 8 digit yang muncul di terminal ke aplikasi WhatsApp:
   * **Pengaturan** > **Perangkat Tertaut** > **Tautkan Perangkat** > **Tautkan dengan nomor telepon saja**.
2. Sesi login akan tersimpan permanen di `data/gowa.db`.

### 3. Menjalankan di Latar Belakang (Daemon Process)
```bash
nohup ./gowa > gowa.log 2>&1 &
```

---

## ➕ Cara Menambah Fitur Baru

Cukup buat file baru di dalam direktori kategori yang sesuai pada `cmd/<kategori>/`:

Contoh file baru `cmd/general/waktu.go`:
```go
package general

import (
	"fmt"
	"time"

	"gowa/internal/command"
)

func init() {
	command.Register(&command.Command{
		Name:    "waktu",
		Cmd:     "waktu",
		Categ:   "general",
		Desc:    "Menampilkan tanggal dan waktu server saat ini",
		Alias:   []string{"jam", "time"},
		Execute: handleWaktu,
	})
}

func handleWaktu(ctx *command.Context) error {
	waktu := time.Now().Format("Monday, 02 January 2006 | 15:04:05 MST")
	return ctx.Reply(fmt.Sprintf("Waktu Server: %s", waktu))
}
```

Jika membuat **kategori folder baru** (misalnya `cmd/tools/`), cukup daftarkan impor paketnya di [`cmd/loader.go`](cmd/loader.go):
```go
import (
	_ "gowa/cmd/general"
	_ "gowa/cmd/owner"
	_ "gowa/cmd/system"
	_ "gowa/cmd/tools"
)
```
Perintah baru akan otomatis terdaftar dan langsung muncul pada output perintah `.menu`!

---

<div align="center">
  <sub>Dibuat dengan ❤️ menggunakan Golang & Whatsmeow. Bebas dikembangkan dan dimodifikasi.</sub>
</div>
