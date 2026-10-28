package bot

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"gowa/internal/command"
	"gowa/internal/storage"
	"gowa/internal/util"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
)

type Bot struct {
	Config    *storage.Config
	Client    *whatsmeow.Client
	Container *sqlstore.Container
	Pool      *WorkerPool
}

func New(cfg *storage.Config) (*Bot, error) {
	ctx := context.Background()

	container, err := storage.InitDatabase(ctx, cfg.DBPath, cfg.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("initsialiasi database gagal: %w", err)
	}

	deviceStore, err := container.GetFirstDevice(ctx)
	if err != nil {
		return nil, fmt.Errorf("gagal mendapatkan device store: %w", err)
	}

	clientLog := util.NewLogger("Whatsmeow")
	client := whatsmeow.NewClient(deviceStore, clientLog)

	client.EnableAutoReconnect = true
	client.AutoTrustIdentity = true
	client.ManualHistorySyncDownload = true
	client.DisableManualHistorySyncReceipt = true

	pool := NewWorkerPool(512)
	sentCache := command.NewSentCache()

	client.AddEventHandler(EventHandler(client, cfg, pool, sentCache))

	return &Bot{
		Config:    cfg,
		Client:    client,
		Container: container,
		Pool:      pool,
	}, nil
}

func (b *Bot) Start() error {
	ctx := context.Background()

	if b.Client.Store.ID == nil {
		qrChan, _ := b.Client.GetQRChannel(ctx)
		if err := b.Client.Connect(); err != nil {
			return fmt.Errorf("gagal menghubungkan ke server WhatsApp: %w", err)
		}

		for evt := range qrChan {
			if evt.Event == "code" {
				break
			}
		}

		phoneNumber := strings.TrimSpace(b.Config.PhoneNumber)
		if phoneNumber == "" {
			fmt.Println()
			fmt.Println("┌──────────────────────────────────────────────────────────┐")
			fmt.Println("│           PENDAFTARAN NOMOR WHATSAPP (PAIRING)          │")
			fmt.Println("└──────────────────────────────────────────────────────────┘")
			fmt.Print("👉 Masukkan nomor HP WhatsApp Anda (contoh: 628123456789): ")
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("gagal membaca input nomor: %w", err)
			}
			phoneNumber = strings.TrimSpace(input)
		}

		phoneNumber = cleanPhoneNumber(phoneNumber)
		if phoneNumber == "" {
			return fmt.Errorf("nomor telepon tidak valid")
		}

		util.Info("Auth", "Meminta kode pairing untuk nomor: %s ...", phoneNumber)
		code, err := b.Client.PairPhone(ctx, phoneNumber, true, whatsmeow.PairClientChrome, "Chrome (Linux)")
		if err != nil {
			return fmt.Errorf("gagal meminta pairing code: %w", err)
		}

		formattedCode := formatPairingCode(code)

		fmt.Println()
		fmt.Println("╔══════════════════════════════════════════════════════════╗")
		fmt.Println("║               KODE PAIRING WHATSAPP ANDA                 ║")
		fmt.Printf("║                     >>  %s  <<                     ║\n", formattedCode)
		fmt.Println("╠══════════════════════════════════════════════════════════╣")
		fmt.Println("║ Langkah menghubungkan:                                   ║")
		fmt.Println("║ 1. Buka aplikasi WhatsApp di HP Anda                     ║")
		fmt.Println("║ 2. Buka menu Perangkat Tertaut (Linked Devices)          ║")
		fmt.Println("║ 3. Tekan 'Tautkan Perangkat'                             ║")
		fmt.Println("║ 4. Pilih 'Tautkan dengan nomor telepon saja'             ║")
		fmt.Println("║ 5. Masukkan 8 digit kode pairing di atas                 ║")
		fmt.Println("╚══════════════════════════════════════════════════════════╝")
		fmt.Println()
	} else {
		util.Info("Session", "Sesi tersimpan ditemukan, menyambungkan kembali ke WhatsApp...")
		if err := b.Client.Connect(); err != nil {
			return fmt.Errorf("gagal menyambungkan: %w", err)
		}
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	fmt.Println()
	util.Info("Shutdown", "Mematikan bot dan membersihkan proses...")
	b.Pool.Stop()
	b.Client.Disconnect()
	_ = b.Container.Close()
	util.Success("Shutdown", "Bot berhasil dimatikan dengan aman.")
	return nil
}

func cleanPhoneNumber(phone string) string {
	var digits strings.Builder
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	s := digits.String()
	if strings.HasPrefix(s, "08") {
		s = "628" + strings.TrimPrefix(s, "08")
	}
	return s
}

func formatPairingCode(code string) string {
	cleaned := strings.ReplaceAll(code, "-", "")
	if len(cleaned) == 8 {
		return cleaned[:4] + "-" + cleaned[4:]
	}
	return code
}
