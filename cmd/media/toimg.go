package media

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"gowa/internal/command"

	waProto "go.mau.fi/whatsmeow/proto/waE2E"
)

func init() {
	command.Register(&command.Command{
		Name:    "toimg/tovid",
		Cmd:     "toimg/tovid",
		Categ:   "media",
		Desc:    "Mengubah stiker statis menjadi gambar atau stiker animasi menjadi video",
		Alias:   []string{"toimg", "tovid", "tovideo", "toimage"},
		Execute: handleToImg,
	})
}

func handleToImg(ctx *command.Context) error {
	ctx.Typing()

	c := ctx.Ctx
	if c == nil {
		c = context.Background()
	}

	var stickerMsg *waProto.StickerMessage
	quoted := ctx.GetQuotedMessage()
	if quoted != nil {
		stickerMsg = quoted.GetStickerMessage()
	}

	if stickerMsg == nil {
		return ctx.Reply(fmt.Sprintf(
			"❌ Balas stiker yang ingin diubah menjadi gambar/video.\n\n"+
				"*Cara Penggunaan:*\n"+
				"- Balas stiker statis dengan: `%stoimg`\n"+
				"- Balas stiker animasi dengan: `%stovid` atau `%stoimg`",
			ctx.BotPrefix, ctx.BotPrefix, ctx.BotPrefix,
		))
	}

	stickerBytes, err := ctx.DownloadMedia(quoted)
	if err != nil {
		return ctx.Reply("❌ Gagal mengunduh stiker: " + err.Error())
	}

	tmpDir, err := os.MkdirTemp("", "gowa_toimg_*")
	if err != nil {
		return ctx.Reply("❌ Gagal membuat ruang kerja sementara: " + err.Error())
	}
	defer os.RemoveAll(tmpDir)

	inputWebP := filepath.Join(tmpDir, "input.webp")
	if err := os.WriteFile(inputWebP, stickerBytes, 0644); err != nil {
		return ctx.Reply("❌ Gagal menyimpan stiker sementara: " + err.Error())
	}

	isAnimated := stickerMsg.GetIsAnimated()

	if isAnimated {
		outputMP4 := filepath.Join(tmpDir, "output.mp4")
		procCtx, procCancel := context.WithTimeout(c, 45*time.Second)
		defer procCancel()

		cmd := exec.CommandContext(procCtx, "ffmpeg", "-i", inputWebP, "-c:v", "libx264", "-pix_fmt", "yuv420p", "-movflags", "+faststart", "-y", outputMP4)
		if out, err := cmd.CombinedOutput(); err != nil {
			return ctx.Reply("❌ Gagal mengonversi stiker animasi ke video: " + string(out))
		}

		mp4Bytes, err := os.ReadFile(outputMP4)
		if err != nil {
			return ctx.Reply("❌ Gagal membaca file video hasil konversi: " + err.Error())
		}

		return ctx.ReplyVideo(mp4Bytes, "video/mp4", "Stiker animasi berhasil dikonversi ke video.")
	}

	outputPNG := filepath.Join(tmpDir, "output.png")
	procCtx, procCancel := context.WithTimeout(c, 30*time.Second)
	defer procCancel()

	cmd := exec.CommandContext(procCtx, "ffmpeg", "-i", inputWebP, "-c:v", "png", "-y", outputPNG)
	if out, err := cmd.CombinedOutput(); err != nil {
		return ctx.Reply("❌ Gagal mengonversi stiker ke gambar: " + string(out))
	}

	pngBytes, err := os.ReadFile(outputPNG)
	if err != nil {
		return ctx.Reply("❌ Gagal membaca file gambar hasil konversi: " + err.Error())
	}

	return ctx.ReplyImage(pngBytes, "image/png", "Stiker berhasil dikonversi ke gambar.")
}
