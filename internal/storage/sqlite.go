package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"gowa/internal/util"

	"go.mau.fi/whatsmeow/store/sqlstore"
	_ "modernc.org/sqlite"
)

func InitDatabase(ctx context.Context, dbPath string, logLevel string) (*sqlstore.Container, error) {
	dir := filepath.Dir(dbPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("gagal membuat direktori database: %w", err)
		}
	}

	dbLog := util.NewLogger("Database")

	dsn := fmt.Sprintf(
		"file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=temp_store(MEMORY)&_pragma=cache_size(-64000)&_pragma=mmap_size(268435456)&_pragma=busy_timeout(5000)",
		dbPath,
	)

	container, err := sqlstore.New(ctx, "sqlite", dsn, dbLog)
	if err != nil {
		return nil, fmt.Errorf("gagal menginisialisasi sqlstore: %w", err)
	}

	return container, nil
}
