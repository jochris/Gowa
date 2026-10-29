package system

import (
	"fmt"
	"runtime"

	"gowa/internal/command"
	"gowa/internal/util"
)

func init() {
	command.Register(&command.Command{
		Name:    "rss",
		Cmd:     "rss",
		Categ:   "system",
		Desc:    "Menampilkan laporan memori proses dan runtime",
		Alias:   []string{"ram", "mem"},
		Execute: handleRss,
	})
}

func handleRss(ctx *command.Context) error {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	reply := fmt.Sprintf(
		"*Memory Statistics*\n\n"+
			"• *Process RSS:* %s\n"+
			"• *Go Alloc:* %.2f MB\n"+
			"• *Go Total Alloc:* %.2f MB\n"+
			"• *Go Sys:* %.2f MB\n"+
			"• *GC Runs:* %d\n"+
			"• *Uptime:* %s",
		util.GetProcessRSS(),
		float64(m.Alloc)/1024/1024,
		float64(m.TotalAlloc)/1024/1024,
		float64(m.Sys)/1024/1024,
		m.NumGC,
		util.GetUptimeFormatted(),
	)

	return ctx.Reply(reply)
}
