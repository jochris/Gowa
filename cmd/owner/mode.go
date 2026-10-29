package owner

import (
	"fmt"
	"strings"

	"gowa/internal/command"
)

func init() {
	command.Register(&command.Command{
		Name:    "self/public",
		Cmd:     "self/public",
		Categ:   "owner",
		Desc:    "Ubah mode bot: Self atau Public",
		Alias:   []string{"self", "public", "mode"},
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
		_ = ctx.Config.Save()
		return ctx.Reply("*Mode:* `self`")
	}

	if trigger == "public" || (len(ctx.Args) > 0 && strings.EqualFold(ctx.Args[0], "public")) {
		ctx.Config.IsPublic = true
		_ = ctx.Config.Save()
		return ctx.Reply("*Mode:* `public`")
	}

	ctx.Config.IsPublic = !ctx.Config.IsPublic
	_ = ctx.Config.Save()
	status := "self"
	if ctx.Config.IsPublic {
		status = "public"
	}
	return ctx.Reply(fmt.Sprintf("*Mode:* `%s`", status))
}
