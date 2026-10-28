package system

import (
	"fmt"
	"runtime"

	"gowa/internal/command"
	"gowa/internal/util"
)

func init() {
	command.Register(&command.Command{
		Name:    "ram",
		Cmd:     "ram",
		Categ:   "system",
		Desc:    "Menampilkan laporan memori proses dan runtime",
		Alias:   []string{"mem"},
		Execute: handleRam,
	})
}

func handleRam(ctx *command.Context) error {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	reply := fmt.Sprintf(
		"Process RSS: %s\nGo Alloc: %.2f MB\nGo Total Alloc: %.2f MB\nGo Sys: %.2f MB\nGC: %d runs\nUptime: %s",
		util.GetProcessRSS(),
		float64(m.Alloc)/1024/1024,
		float64(m.TotalAlloc)/1024/1024,
		float64(m.Sys)/1024/1024,
		m.NumGC,
		util.GetUptimeFormatted(),
	)

	return ctx.Reply(reply)
}
