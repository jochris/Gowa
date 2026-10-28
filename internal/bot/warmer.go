package bot

import (
	"context"
	"sync"
	"time"

	"gowa/internal/util"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

var warmupOnce sync.Once

func PreWarmGroups(client *whatsmeow.Client) {
	warmupOnce.Do(func() {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
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

			util.Info("WARMER", "Ditemukan %d grup. Meng-cache anggota & perangkat peserta...", len(groups))

			totalParticipants := 0
			totalGroups := len(groups)

			for i, group := range groups {
				select {
				case <-time.After(50 * time.Millisecond):
				case <-ctx.Done():
					return
				}

				info, err := client.GetGroupInfo(ctx, group.JID)
				if err != nil {
					totalParticipants += group.ParticipantCount
					continue
				}

				count := len(info.Participants)
				totalParticipants += count

				participantJIDs := make([]types.JID, count)
				for idx, p := range info.Participants {
					participantJIDs[idx] = p.JID
				}

				if len(participantJIDs) > 0 {
					for b := 0; b < len(participantJIDs); b += 100 {
						end := b + 100
						if end > len(participantJIDs) {
							end = len(participantJIDs)
						}
						_, _ = client.GetUserDevices(ctx, participantJIDs[b:end])
					}
				}

				if (i+1)%15 == 0 || i+1 == totalGroups {
					util.Info("WARMER", "Progress: %d/%d grup ter-cache...", i+1, totalGroups)
				}
			}

			util.Success("WARMER", "Selesai! %d grup & %d anggota siap di RAM.", totalGroups, totalParticipants)
		}()
	})
}
