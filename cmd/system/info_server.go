package system

import (
	"fmt"
	"runtime"

	"gowa/internal/command"
	"gowa/internal/util"
)

func init() {
	command.Register(&command.Command{
		Name:    "infoserver",
		Cmd:     "infoserver",
		Categ:   "system",
		Desc:    "Menampilkan informasi spesifikasi OS, Processor, CPU, GPU, dan runtime",
		Alias:   []string{"info"},
		Execute: handleInfoServer,
	})
}

func handleInfoServer(ctx *command.Context) error {
	processor, cpu, cpuLoad, gpu := util.GetHardwareReport()

	reply := fmt.Sprintf(
		"OS: %s (%s)\nProcessor: %s\nCPU: %s\nCPU Load: %s\nGPU: %s\nProcess RSS: %s\nGo Runtime: %s\nGoroutines: %d\nUptime: %s",
		runtime.GOOS,
		runtime.GOARCH,
		processor,
		cpu,
		cpuLoad,
		gpu,
		util.GetProcessRSS(),
		runtime.Version(),
		runtime.NumGoroutine(),
		util.GetUptimeFormatted(),
	)

	return ctx.Reply(reply)
}
