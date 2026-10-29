package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	BotName       string   `json:"bot_name"`
	Prefix        string   `json:"prefix"`
	DBPath        string   `json:"db_path"`
	PhoneNumber   string   `json:"phone_number"`
	LogLevel      string   `json:"log_level"`
	IsPublic      bool     `json:"is_public"`
	OwnerNumbers  []string `json:"owner_numbers"`
	IgnoreOffline bool     `json:"ignore_offline"`
	FilePath      string   `json:"-"`
}

var DefaultConfig = Config{
	BotName:       "Gowa",
	Prefix:        ".",
	DBPath:        "data/gowa.db",
	PhoneNumber:   "",
	LogLevel:      "INFO",
	IsPublic:      true,
	OwnerNumbers:  []string{},
	IgnoreOffline: true,
	FilePath:      "config.json",
}

func (c *Config) Validate() {
	if strings.TrimSpace(c.BotName) == "" {
		c.BotName = "Gowa"
	}
	if strings.TrimSpace(c.Prefix) == "" {
		c.Prefix = "."
	}
	if strings.TrimSpace(c.DBPath) == "" {
		c.DBPath = "data/gowa.db"
	}
	if strings.TrimSpace(c.LogLevel) == "" {
		c.LogLevel = "INFO"
	}
	if c.OwnerNumbers == nil {
		c.OwnerNumbers = []string{}
	}
}

func (c *Config) Save() error {
	path := c.FilePath
	if path == "" {
		path = "config.json"
	}
	return SaveConfig(path, c)
}

func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig
	cfg.FilePath = path

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg.Validate()
			_ = SaveConfig(path, &cfg)
			return &cfg, nil
		}
		return nil, fmt.Errorf("gagal membuka berkas konfigurasi: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("gagal membedah JSON konfigurasi: %w", err)
	}

	cfg.FilePath = path
	cfg.Validate()
	return &cfg, nil
}

func SaveConfig(path string, cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("gagal serialisasi konfigurasi: %w", err)
	}

	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("gagal membuat direktori konfigurasi: %w", err)
		}
	}

	tmpFile := path + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("gagal menulis berkas sementara: %w", err)
	}

	if err := os.Rename(tmpFile, path); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("gagal mengganti berkas konfigurasi secara atomik: %w", err)
	}

	return nil
}
