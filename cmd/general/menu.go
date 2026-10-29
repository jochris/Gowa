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
	botTitle := "GOWA"
	if ctx.Config != nil && ctx.Config.BotName != "" {
		botTitle = strings.ToUpper(ctx.Config.BotName)
	}
	sb.WriteString(fmt.Sprintf("*%s - Command List*\n\n", botTitle))

	for i, cat := range categories {
		catTitle := strings.ToUpper(cat[:1]) + strings.ToLower(cat[1:])
		sb.WriteString(fmt.Sprintf("*%s*\n", catTitle))
		cmds := command.GetByCategory(cat)
		for _, cmd := range cmds {
			firstAlias := ""
			if !strings.Contains(cmd.Cmd, "/") && len(cmd.Alias) > 0 {
				for _, a := range cmd.Alias {
					clean := strings.TrimPrefix(a, ".")
					if !strings.EqualFold(clean, cmd.Cmd) {
						firstAlias = fmt.Sprintf(" _(%s%s)_", ctx.BotPrefix, clean)
						break
					}
				}
			}

			if strings.HasPrefix(cmd.Cmd, "$") {
				sb.WriteString(fmt.Sprintf("• `%s <cmd>`%s\n", cmd.Cmd, firstAlias))
			} else {
				sb.WriteString(fmt.Sprintf("• `%s%s`%s\n", ctx.BotPrefix, cmd.Cmd, firstAlias))
			}
		}
		if i < len(categories)-1 {
			sb.WriteString("\n")
		}
	}

	return ctx.Reply(strings.TrimSpace(sb.String()))
}
