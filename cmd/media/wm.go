package media

import (
	"context"
	"fmt"
	"strings"

	"gowa/internal/command"

	waProto "go.mau.fi/whatsmeow/proto/waE2E"
)

func init() {
	command.Register(&command.Command{
		Name:    "wm",
		Cmd:     "wm",
		Categ:   "media",
		Desc:    "Mengubah watermark / metadata (pack & author) stiker WhatsApp",
		Alias:   []string{"take"},
		Execute: handleWM,
	})
}

func handleWM(ctx *command.Context) error {
	ctx.Typing()

	c := ctx.Ctx
	if c == nil {
		c = context.Background()
	}

	var targetMsg *waProto.Message
	if ctx.Event != nil && ctx.Event.Message != nil {
		unwrapped := command.UnwrapMessage(ctx.Event.Message)
		if isStickerSource(unwrapped) {
			targetMsg = unwrapped
		}
	}
	if targetMsg == nil {
		quoted := ctx.GetQuotedMessage()
		if quoted != nil && isStickerSource(quoted) {
			targetMsg = quoted
		}
	}

	if targetMsg == nil {
		return ctx.Reply(fmt.Sprintf(
			"❌ Balas stiker yang ingin diubah watermark-nya.\n\n"+
				"*Cara Penggunaan:*\n"+
				"- Balas stiker dengan: `%swm <pack> | <author>`\n"+
				"- Contoh: `%swm Gowa | Astralune`",
			ctx.BotPrefix, ctx.BotPrefix,
		))
	}

	var packName, authorName string
	cleanArgs := strings.TrimSpace(ctx.RawArgs)

	if cleanArgs == "" {
		if !ctx.IsOwner {
			return ctx.Reply(fmt.Sprintf(
				"❌ Masukkan nama pack dan author untuk watermark.\n\n"+
					"*Cara Penggunaan:*\n"+
					"- Balas stiker dengan: `%swm <pack> | <author>`\n"+
					"- Contoh: `%swm Gowa | Astralune`",
				ctx.BotPrefix, ctx.BotPrefix,
			))
		}
		packName = defaultStickerPack
		if ctx.Config != nil && ctx.Config.BotName != "" {
			packName = ctx.Config.BotName
		}
		authorName = defaultStickerAuthor
	} else {
		packName, authorName = parseStickerMetadata(cleanArgs)
		if !ctx.IsOwner && !strings.Contains(cleanArgs, "|") && ctx.Event != nil && ctx.Event.Info.PushName != "" {
			authorName = ctx.Event.Info.PushName
		}
	}

	fileData, err := ctx.DownloadMedia(targetMsg)
	if err != nil {
		return ctx.Reply("❌ Gagal mengunduh media dari WhatsApp: " + err.Error())
	}

	if int64(len(fileData)) > maxStickerInputBytes {
		return ctx.Reply("❌ Ukuran media terlalu besar (maksimal 25 MB).")
	}

	mediaType, isAnimated := detectStickerSourceType(targetMsg)

	finalBytes, isAnim, err := processSticker(c, fileData, mediaType, isAnimated, packName, authorName)
	if err != nil {
		return ctx.Reply("❌ Gagal memproses watermark stiker: " + err.Error())
	}

	return ctx.ReplySticker(finalBytes, isAnim)
}
