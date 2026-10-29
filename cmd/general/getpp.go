package general

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"gowa/internal/command"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

func init() {
	command.Register(&command.Command{
		Name:    "getpp",
		Cmd:     "getpp",
		Categ:   "general",
		Desc:    "Ambil foto profil via reply, tag, nomor, atau profil sendiri",
		Alias:   []string{"pp", "avatar"},
		Execute: handleGetPP,
	})
}

func handleGetPP(ctx *command.Context) error {
	ctx.Typing()

	c := ctx.Ctx
	if c == nil {
		c = context.Background()
	}

	var targetJID types.JID
	ci := ctx.GetContextInfo()

	if len(ctx.Args) > 0 {
		if ci != nil && len(ci.GetMentionedJID()) > 0 {
			for _, m := range ci.GetMentionedJID() {
				if parsed, err := types.ParseJID(m); err == nil && !parsed.IsEmpty() {
					targetJID = parsed.ToNonAD()
					break
				}
			}
		}

		if targetJID.IsEmpty() {
			num := cleanNumber(ctx.RawArgs)
			if len(num) >= 9 {
				chkCtx, chkCancel := context.WithTimeout(c, 15*time.Second)
				defer chkCancel()

				resp, err := ctx.Client.IsOnWhatsApp(chkCtx, []string{"+" + num})
				if err != nil {
					return ctx.Reply("❌ Gagal memeriksa nomor di WhatsApp: " + err.Error())
				}
				if len(resp) == 0 || !resp[0].IsIn {
					return ctx.Reply(fmt.Sprintf("❌ Nomor +%s tidak terdaftar di WhatsApp.", num))
				}
				if !resp[0].JID.IsEmpty() {
					targetJID = resp[0].JID.ToNonAD()
				} else {
					targetJID = types.NewJID(num, types.DefaultUserServer)
				}
			} else {
				return ctx.Reply(fmt.Sprintf("❌ Format nomor tidak valid.\n\n*Cara Penggunaan:*\n• Reply pesan target: `%sgetpp`\n• Tag pengguna: `%sgetpp @user`\n• Masukkan nomor: `%sgetpp 628xxxxxxxx`", ctx.BotPrefix, ctx.BotPrefix, ctx.BotPrefix))
			}
		}
	} else if ci != nil && ci.GetQuotedMessage() != nil {
		pStr := ci.GetParticipant()
		if pStr != "" {
			if parsed, err := types.ParseJID(pStr); err == nil && !parsed.IsEmpty() {
				targetJID = parsed.ToNonAD()
			}
		} else if !ctx.IsGroup {
			targetJID = ctx.Chat.ToNonAD()
		}
	}

	if targetJID.IsEmpty() {
		targetJID = ctx.Sender.ToNonAD()
	}

	var commonGID types.JID
	if ctx.IsGroup {
		commonGID = ctx.Chat.ToNonAD()
	}

	fetchPic := func(jid types.JID) (*types.ProfilePictureInfo, error) {
		reqCtx, reqCancel := context.WithTimeout(c, 15*time.Second)
		defer reqCancel()

		info, err := ctx.Client.GetProfilePictureInfo(reqCtx, jid, &whatsmeow.GetProfilePictureParams{
			Preview:   false,
			CommonGID: commonGID,
		})
		if err == nil && info != nil && info.URL != "" {
			return info, nil
		}
		info, err = ctx.Client.GetProfilePictureInfo(reqCtx, jid, &whatsmeow.GetProfilePictureParams{
			Preview:   true,
			CommonGID: commonGID,
		})
		if err == nil && info != nil && info.URL != "" {
			return info, nil
		}
		return nil, err
	}

	picInfo, err := fetchPic(targetJID)
	if err != nil && targetJID.Server == types.HiddenUserServer {
		if pn, errPN := ctx.ResolvePN(targetJID); errPN == nil && !pn.IsEmpty() {
			picInfo, err = fetchPic(pn.ToNonAD())
		}
	}
	if err != nil && targetJID.Server == types.DefaultUserServer {
		if lid, errLID := ctx.ResolveLID(targetJID); errLID == nil && !lid.IsEmpty() {
			picInfo, err = fetchPic(lid.ToNonAD())
		}
	}

	if err != nil {
		if errors.Is(err, whatsmeow.ErrProfilePictureUnauthorized) {
			return ctx.Reply("❌ Foto profil disembunyikan oleh pengguna (pengaturan privasi).")
		}
		if errors.Is(err, whatsmeow.ErrProfilePictureNotSet) {
			return ctx.Reply("❌ Target tidak memiliki foto profil.")
		}
		return ctx.Reply("❌ Gagal mengambil foto profil: " + err.Error())
	}

	if picInfo == nil || picInfo.URL == "" {
		return ctx.Reply("❌ Target tidak memiliki foto profil.")
	}

	dlCtx, dlCancel := context.WithTimeout(c, 30*time.Second)
	defer dlCancel()

	req, err := http.NewRequestWithContext(dlCtx, http.MethodGet, picInfo.URL, nil)
	if err != nil {
		return ctx.Reply("❌ Gagal menyiapkan unduhan foto profil.")
	}

	httpResp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ctx.Reply("❌ Gagal mengunduh gambar foto profil.")
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return ctx.Reply(fmt.Sprintf("❌ Server WhatsApp mengembalikan kode status HTTP %d.", httpResp.StatusCode))
	}

	imgData, err := io.ReadAll(httpResp.Body)
	if err != nil || len(imgData) == 0 {
		return ctx.Reply("❌ Data gambar foto profil kosong atau gagal dibaca.")
	}

	displayNumber := targetJID.User
	if targetJID.Server == types.HiddenUserServer {
		if pn, err := ctx.ResolvePN(targetJID); err == nil && !pn.IsEmpty() {
			displayNumber = pn.User
		}
	}

	if targetJID.Server == types.GroupServer {
		caption := fmt.Sprintf("📸 *Group Photo*\n• ID: `%s`", targetJID.User)
		return ctx.ReplyImage(imgData, "image/jpeg", caption)
	}

	caption := fmt.Sprintf("📸 *Profile Picture*\n• Target: @%s", displayNumber)
	mentionJID := displayNumber + "@s.whatsapp.net"
	return ctx.ReplyImage(imgData, "image/jpeg", caption, mentionJID)
}

func cleanNumber(input string) string {
	var sb strings.Builder
	for _, r := range input {
		if r >= '0' && r <= '9' {
			sb.WriteRune(r)
		}
	}
	s := sb.String()
	if strings.HasPrefix(s, "0") && len(s) > 8 {
		s = "62" + s[1:]
	} else if strings.HasPrefix(s, "8") && len(s) >= 9 && len(s) <= 13 {
		s = "62" + s
	}
	return s
}
