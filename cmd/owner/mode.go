package owner

import (
	"fmt"
	"strings"

	"gowa/internal/command"
	"gowa/internal/storage"
)

func init() {
	command.Register(&command.Command{
		Name:    "self",
		Cmd:     "self",
		Categ:   "owner",
		Desc:    "Ubah mode bot: Self atau Public",
		Alias:   []string{"public", "mode"},
		Execute: handleMode,
	})
}

func handleMode(ctx *command.Context) error {
	if !ctx.IsOwner {
		return nil
	}

	trigger := strings.ToLower(ctx.Command)

	if trigger == "self" || (len(ctx.Args) > 0 && strings.EqualFold(ctx.Args[0], "self")) {
		ctx.Config.IsPublic = false
		_ = storage.SaveConfig("config.json", ctx.Config)
		return ctx.Reply("*Mode:* `self`")
	}

	if trigger == "public" || (len(ctx.Args) > 0 && strings.EqualFold(ctx.Args[0], "public")) {
		ctx.Config.IsPublic = true
		_ = storage.SaveConfig("config.json", ctx.Config)
		return ctx.Reply("*Mode:* `public`")
	}

	ctx.Config.IsPublic = !ctx.Config.IsPublic
	_ = storage.SaveConfig("config.json", ctx.Config)
	status := "self"
	if ctx.Config.IsPublic {
		status = "public"
	}
	return ctx.Reply(fmt.Sprintf("*Mode:* `%s`", status))
}
