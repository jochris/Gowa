package main

import (
	"flag"
	"fmt"
	"strings"

	"gowa/internal/bot"
	"gowa/internal/command"
	"gowa/internal/storage"
	"gowa/internal/util"

	_ "gowa/cmd"
)

func main() {
	configPath := flag.String("config", "config.json", "Lokasi file konfigurasi")
	phoneFlag := flag.String("phone", "", "Nomor telepon untuk pairing code (contoh: 628123456789)")
	prefixFlag := flag.String("prefix", "", "Prefix perintah bot (contoh: . atau !)")
	flag.Parse()

	cfg, err := storage.LoadConfig(*configPath)
	if err != nil {
		util.Error("Config", "Gagal memuat konfigurasi: %v", err)
		return
	}

	if *phoneFlag != "" {
		cfg.PhoneNumber = *phoneFlag
	}
	if *prefixFlag != "" {
		cfg.Prefix = *prefixFlag
	}

	fmt.Println()
	fmt.Println("  \033[1;36m╭──────────────────────────────────────────────╮\033[0m")
	fmt.Printf("  \033[1;36m│\033[0m   🤖 \033[1;32m%s\033[0m - \033[90mWhatsApp High Performance Bot\033[0m     \033[1;36m│\033[0m\n", cfg.BotName)
	fmt.Println("  \033[1;36m├──────────────────────────────────────────────┤\033[0m")
	fmt.Printf("  \033[1;36m│\033[0m  • Prefix   : \033[1;33m%-32s\033[0m \033[1;36m│\033[0m\n", cfg.Prefix)
	fmt.Printf("  \033[1;36m│\033[0m  • Kategori : \033[35m%-32s\033[0m \033[1;36m│\033[0m\n", strings.Join(command.GetCategories(), ", "))
	fmt.Printf("  \033[1;36m│\033[0m  • Perintah : \033[1;37m%-32s\033[0m \033[1;36m│\033[0m\n", fmt.Sprintf("%d commands terdaftar", len(command.GetAll())))
	fmt.Println("  \033[1;36m╰──────────────────────────────────────────────╯\033[0m")
	fmt.Println()

	b, err := bot.New(cfg)
	if err != nil {
		util.Error("Init", "Gagal membuat instance bot: %v", err)
		return
	}

	if err := b.Start(); err != nil {
		util.Error("Runtime", "Bot berhenti dengan error: %v", err)
	}
}
