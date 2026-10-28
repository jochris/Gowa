package general

import (
	"fmt"
	"time"

	"gowa/internal/command"
	"gowa/internal/util"
)

func init() {
	command.Register(&command.Command{
		Name:    "ping",
		Cmd:     "ping",
		Categ:   "general",
		Desc:    "Cek kecepatan respon bot dan durasi aktif (uptime)",
		Alias:   []string{"p"},
		Execute: handlePing,
	})
}

func handlePing(ctx *command.Context) error {
	now := time.Now()
	var latency time.Duration

	if !ctx.Event.Info.Timestamp.IsZero() {
		latency = now.Sub(ctx.Event.Info.Timestamp)
		if latency < 0 {
			latency = 0
		}
	}

	reply := fmt.Sprintf("Pong\nUptime: %s\nLatency: %v",
		util.GetUptimeFormatted(),
		latency.Round(time.Millisecond),
	)

	return ctx.Reply(reply)
}
