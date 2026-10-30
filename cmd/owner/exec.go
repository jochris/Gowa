package owner

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"gowa/internal/command"
)

func init() {
	command.Register(&command.Command{
		Name:    "$",
		Cmd:     "$",
		Categ:   "owner",
		Desc:    "Eksekusi perintah terminal shell (Khusus Owner)",
		Alias:   nil,
		Execute: handleExec,
	})
}

func handleExec(ctx *command.Context) error {
	if !ctx.IsOwner {
		return nil
	}

	commandLine := strings.TrimSpace(ctx.RawArgs)
	if commandLine == "" {
		return ctx.Reply("*Usage:* `$ <cmd>`")
	}

	ctx.Typing()

	baseCtx := ctx.Ctx
	if baseCtx == nil {
		baseCtx = context.Background()
	}

	cmdCtx, cancel := context.WithTimeout(baseCtx, 15*time.Second)
	defer cancel()

	shell := "bash"
	shellFlag := "-c"
	if runtime.GOOS == "windows" {
		shell = "cmd"
		shellFlag = "/c"
	} else if _, err := exec.LookPath("bash"); err != nil {
		shell = "sh"
	}

	cmd := exec.CommandContext(cmdCtx, shell, shellFlag, commandLine)
	output, err := cmd.CombinedOutput()

	result := strings.TrimSpace(string(output))
	if result == "" {
		if err != nil {
			result = fmt.Sprintf("*Error:* %v", err)
		} else {
			result = "_Selesai tanpa output._"
		}
	} else {
		if len(result) > 4000 {
			result = result[:4000] + "\n_... (output dipotong)_"
		}
		result = fmt.Sprintf("```\n%s\n```", result)
	}

	return ctx.Reply(result)
}
