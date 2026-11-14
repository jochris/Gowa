package bot

import (
	"context"
	"runtime"
	"runtime/debug"
	"sync"
	"time"

	"gowa/internal/util"

	"go.mau.fi/whatsmeow"
)

var warmupOnce sync.Once

func PreWarmGroups(client *whatsmeow.Client) {
	warmupOnce.Do(func() {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			select {
			case <-time.After(2 * time.Second):
			case <-ctx.Done():
				return
			}

			groups, err := client.GetJoinedGroups(ctx)
			if err != nil {
				util.Warn("WARMER", "Gagal mengambil daftar grup: %v", err)
				return
			}

			if len(groups) == 0 {
				util.Info("WARMER", "Bot belum terdaftar di grup manapun.")
				return
			}

			totalParticipants := 0
			totalGroups := len(groups)

			for _, group := range groups {
				if len(group.Participants) > 0 {
					totalParticipants += len(group.Participants)
				} else {
					totalParticipants += group.ParticipantCount
				}
			}

			util.Success("WARMER", "Selesai! %d grup & %d anggota siap di RAM.", totalGroups, totalParticipants)
			runtime.GC()
			debug.FreeOSMemory()
		}()
	})
}
