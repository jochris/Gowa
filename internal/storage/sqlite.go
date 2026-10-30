package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"time"

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
		"file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=temp_store(MEMORY)&_pragma=cache_size(-4000)&_pragma=mmap_size(0)&_pragma=busy_timeout(5000)",
		dbPath,
	)

	container, err := sqlstore.New(ctx, "sqlite", dsn, dbLog)
	if err != nil {
		return nil, fmt.Errorf("gagal menginisialisasi sqlstore: %w", err)
	}

	return container, nil
}

func GetDatabaseSize(dbPath string) int64 {
	var total int64
	files := []string{dbPath, dbPath + "-wal", dbPath + "-shm"}
	for _, f := range files {
		if fi, err := os.Stat(f); err == nil {
			total += fi.Size()
		}
	}
	return total
}

func VacuumDatabase(ctx context.Context, dbPath string) (int64, int64, time.Duration, error) {
	beforeSize := GetDatabaseSize(dbPath)
	start := time.Now()

	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(10000)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return beforeSize, beforeSize, 0, fmt.Errorf("gagal membuka database: %w", err)
	}
	defer db.Close()

	if _, err := db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return beforeSize, beforeSize, 0, fmt.Errorf("wal checkpoint gagal: %w", err)
	}

	if _, err := db.ExecContext(ctx, "VACUUM"); err != nil {
		return beforeSize, beforeSize, 0, fmt.Errorf("vacuum gagal: %w", err)
	}

	if _, err := db.ExecContext(ctx, "PRAGMA optimize"); err != nil {
		return beforeSize, beforeSize, 0, fmt.Errorf("optimize gagal: %w", err)
	}

	runtime.GC()
	debug.FreeOSMemory()

	afterSize := GetDatabaseSize(dbPath)
	duration := time.Since(start)

	return beforeSize, afterSize, duration, nil
}
