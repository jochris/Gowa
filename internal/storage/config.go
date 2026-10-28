package storage

import (
	"encoding/json"
	"os"
)

type Config struct {
	BotName      string   `json:"bot_name"`
	Prefix       string   `json:"prefix"`
	DBPath       string   `json:"db_path"`
	PhoneNumber  string   `json:"phone_number"`
	LogLevel     string   `json:"log_level"`
	IsPublic     bool     `json:"is_public"`
	OwnerNumbers []string `json:"owner_numbers"`
}

var DefaultConfig = Config{
	BotName:      "Gowa",
	Prefix:       ".",
	DBPath:       "data/gowa.db",
	PhoneNumber:  "",
	LogLevel:     "INFO",
	IsPublic:     true,
	OwnerNumbers: []string{},
}

func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			_ = SaveConfig(path, &cfg)
			return &cfg, nil
		}
		return nil, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func SaveConfig(path string, cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
