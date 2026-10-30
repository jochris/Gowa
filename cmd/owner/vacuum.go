package owner

import (
	"context"
	"fmt"
	"time"

	"gowa/internal/command"
	"gowa/internal/storage"
)

func init() {
	command.Register(&command.Command{
		Name:    "vacuum",
		Cmd:     "vacuum",
		Categ:   "owner",
		Desc:    "Optimasi & pemadatan database SQLite (Khusus Owner)",
		Alias:   []string{"clean", "cleandb"},
		Execute: handleVacuum,
	})
}

func handleVacuum(ctx *command.Context) error {
	if !ctx.IsOwner {
		return nil
	}

	ctx.Typing()

	dbPath := "data/gowa.db"
	if ctx.Config != nil && ctx.Config.DBPath != "" {
		dbPath = ctx.Config.DBPath
	}

	reqCtx := ctx.Ctx
	if reqCtx == nil {
		reqCtx = context.Background()
	}

	vCtx, cancel := context.WithTimeout(reqCtx, 30*time.Second)
	defer cancel()

	beforeSize, afterSize, duration, err := storage.VacuumDatabase(vCtx, dbPath)
	if err != nil {
		return ctx.Reply("❌ Gagal melakukan vacuum database: " + err.Error())
	}

	savedBytes := beforeSize - afterSize
	if savedBytes < 0 {
		savedBytes = 0
	}
	savedPercentage := 0.0
	if beforeSize > 0 {
		savedPercentage = (float64(savedBytes) / float64(beforeSize)) * 100.0
	}

	reply := fmt.Sprintf(
		"🧹 *Database Vacuum & Optimization Selesai!*\n\n"+
			"• *Ukuran Sebelum:* %s\n"+
			"• *Ukuran Sesudah:* %s\n"+
			"• *Ruang Dihemat:* %s (%.1f%%)\n"+
			"• *WAL Checkpoint:* Selesai (Truncated)\n"+
			"• *RAM Scavenger:* Selesai (OS Memory Freed)\n"+
			"• *Durasi:* %v",
		formatByteSize(beforeSize),
		formatByteSize(afterSize),
		formatByteSize(savedBytes),
		savedPercentage,
		duration.Round(time.Millisecond),
	)

	return ctx.Reply(reply)
}

func formatByteSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
