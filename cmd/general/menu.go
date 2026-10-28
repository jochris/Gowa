package general

import (
	"fmt"
	"strings"

	"gowa/internal/command"
)

func init() {
	command.Register(&command.Command{
		Name:    "menu",
		Cmd:     "menu",
		Categ:   "general",
		Desc:    "Menampilkan daftar perintah bot",
		Alias:   []string{"help"},
		Execute: handleMenu,
	})
}

func handleMenu(ctx *command.Context) error {
	categories := command.GetCategories()

	var sb strings.Builder

	for i, cat := range categories {
		sb.WriteString(fmt.Sprintf("[%s]\n", strings.ToLower(cat)))
		cmds := command.GetByCategory(cat)
		for _, cmd := range cmds {
			firstAlias := ""
			if len(cmd.Alias) > 0 {
				for _, a := range cmd.Alias {
					clean := strings.TrimPrefix(a, ".")
					if !strings.EqualFold(clean, cmd.Cmd) {
						firstAlias = fmt.Sprintf(" (%s%s)", ctx.BotPrefix, clean)
						break
					}
				}
			}
			sb.WriteString(fmt.Sprintf("• %s%s\n", cmd.Cmd, firstAlias))
		}
		if i < len(categories)-1 {
			sb.WriteString("\n")
		}
	}

	return ctx.Reply(strings.TrimSpace(sb.String()))
}
