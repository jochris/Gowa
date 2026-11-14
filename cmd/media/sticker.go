package media

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gowa/internal/command"

	waProto "go.mau.fi/whatsmeow/proto/waE2E"
)

const (
	defaultStickerPack   = "Gowa"
	defaultStickerAuthor = "Astralune"
	maxStickerVideoSec   = 8
	maxStickerInputBytes = 25 * 1024 * 1024
)

func init() {
	command.Register(&command.Command{
		Name:    "sticker",
		Cmd:     "sticker",
		Categ:   "media",
		Desc:    "Mengubah foto, video singkat, GIF, atau stiker menjadi stiker WhatsApp",
		Alias:   []string{"s", "stiker"},
		Execute: handleSticker,
	})
}

func handleSticker(ctx *command.Context) error {
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

	var (
		fileData   []byte
		mediaType  string
		isAnimated bool
		err        error
	)

	if targetMsg != nil {
		mediaType, isAnimated = detectStickerSourceType(targetMsg)
		fileData, err = ctx.DownloadMedia(targetMsg)
		if err != nil {
			return ctx.Reply("❌ Gagal mengunduh media dari WhatsApp: " + err.Error())
		}
	} else if len(ctx.Args) > 0 {
		for _, arg := range ctx.Args {
			if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") {
				dlCtx, cancel := context.WithTimeout(c, 45*time.Second)
				defer cancel()

				fileData, mediaType, isAnimated, err = downloadStickerURL(dlCtx, arg)
				if err != nil {
					return ctx.Reply("❌ Gagal mengunduh media dari URL: " + err.Error())
				}
				break
			}
		}
	}

	if len(fileData) == 0 {
		return ctx.Reply(fmt.Sprintf(
			"❌ Tidak ditemukan foto, video, atau stiker untuk diproses.\n\n"+
				"*Cara Penggunaan:*\n"+
				"- Balas foto / video / stiker dengan: `%ss`\n"+
				"- Kirim foto / video dengan caption: `%ss`\n"+
				"- Dari link: `%ss https://example.com/foto.jpg`\n"+
				"- Untuk ubah watermark stiker: gunakan `%swm Pack | Author`",
			ctx.BotPrefix, ctx.BotPrefix, ctx.BotPrefix, ctx.BotPrefix,
		))
	}

	if int64(len(fileData)) > maxStickerInputBytes {
		return ctx.Reply("❌ Ukuran media terlalu besar (maksimal 25 MB).")
	}

	packName, authorName := parseStickerMetadata(ctx.RawArgs)
	if ctx.Config != nil && ctx.Config.BotName != "" && strings.TrimSpace(ctx.RawArgs) == "" {
		packName = ctx.Config.BotName
	}

	finalBytes, isAnim, err := processSticker(c, fileData, mediaType, isAnimated, packName, authorName)
	if err != nil {
		return ctx.Reply("❌ " + err.Error())
	}

	return ctx.ReplySticker(finalBytes, isAnim)
}

func processSticker(c context.Context, fileData []byte, mediaType string, isAnimated bool, packName, authorName string) ([]byte, bool, error) {
	tmpDir, err := os.MkdirTemp("", "gowa_sticker_*")
	if err != nil {
		return nil, false, fmt.Errorf("gagal membuat ruang kerja sementara: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	inputExt := ".bin"
	switch mediaType {
	case "image":
		inputExt = ".jpg"
	case "video":
		inputExt = ".mp4"
	case "sticker":
		inputExt = ".webp"
	}

	inputFile := filepath.Join(tmpDir, "input"+inputExt)
	if err := os.WriteFile(inputFile, fileData, 0644); err != nil {
		return nil, false, fmt.Errorf("gagal menyimpan media sementara: %w", err)
	}

	rawWebpFile := filepath.Join(tmpDir, "raw.webp")
	finalWebpFile := filepath.Join(tmpDir, "final.webp")
	exifFile := filepath.Join(tmpDir, "metadata.exif")

	exifBytes := buildStickerExif(packName, authorName)
	if err := os.WriteFile(exifFile, exifBytes, 0644); err != nil {
		return nil, false, fmt.Errorf("gagal membuat metadata stiker: %w", err)
	}

	if mediaType == "sticker" {
		rawWebpFile = inputFile
	} else if mediaType == "video" {
		procCtx, procCancel := context.WithTimeout(c, 60*time.Second)
		defer procCancel()

		vf := "scale=512:512:force_original_aspect_ratio=decrease,fps=15,pad=512:512:(512-iw)/2:(512-ih)/2:color=0x00000000"
		durStr := fmt.Sprintf("%d", maxStickerVideoSec)
		cmd := exec.CommandContext(procCtx, "ffmpeg", "-t", durStr, "-i", inputFile, "-vf", vf, "-c:v", "libwebp", "-lossless", "0", "-compression_level", "4", "-q:v", "55", "-loop", "0", "-an", "-vsync", "0", "-y", rawWebpFile)
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, false, fmt.Errorf("gagal mengonversi video ke stiker animasi: %s", string(out))
		}
		isAnimated = true
	} else {
		procCtx, procCancel := context.WithTimeout(c, 30*time.Second)
		defer procCancel()

		vf := "scale=512:512:force_original_aspect_ratio=decrease,pad=512:512:(512-iw)/2:(512-ih)/2:color=0x00000000"
		cmd := exec.CommandContext(procCtx, "ffmpeg", "-i", inputFile, "-vf", vf, "-c:v", "libwebp", "-lossless", "0", "-compression_level", "4", "-q:v", "75", "-loop", "0", "-an", "-y", rawWebpFile)
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, false, fmt.Errorf("gagal mengonversi gambar ke stiker: %s", string(out))
		}
	}

	muxCtx, muxCancel := context.WithTimeout(c, 15*time.Second)
	defer muxCancel()

	muxCmd := exec.CommandContext(muxCtx, "webpmux", "-set", "exif", exifFile, rawWebpFile, "-o", finalWebpFile)
	if err := muxCmd.Run(); err != nil {
		finalWebpFile = rawWebpFile
	}

	finalBytes, err := os.ReadFile(finalWebpFile)
	if err != nil {
		return nil, false, fmt.Errorf("gagal membaca file stiker akhir: %w", err)
	}

	if len(finalBytes) > 1024*1024 && isAnimated && mediaType != "sticker" {
		optWebpFile := filepath.Join(tmpDir, "opt.webp")
		optCtx, optCancel := context.WithTimeout(c, 60*time.Second)
		defer optCancel()

		vf := "scale=384:384:force_original_aspect_ratio=decrease,fps=12,pad=384:384:(384-iw)/2:(384-ih)/2:color=0x00000000"
		cmd := exec.CommandContext(optCtx, "ffmpeg", "-t", "6", "-i", inputFile, "-vf", vf, "-c:v", "libwebp", "-lossless", "0", "-compression_level", "5", "-q:v", "40", "-loop", "0", "-an", "-vsync", "0", "-y", optWebpFile)
		if err := cmd.Run(); err == nil {
			muxOpt := exec.CommandContext(optCtx, "webpmux", "-set", "exif", exifFile, optWebpFile, "-o", finalWebpFile)
			if err := muxOpt.Run(); err == nil {
				if b, err := os.ReadFile(finalWebpFile); err == nil && len(b) <= 1024*1024 {
					finalBytes = b
				}
			}
		}
	}

	return finalBytes, isAnimated, nil
}

func isStickerSource(msg *waProto.Message) bool {
	if msg == nil {
		return false
	}
	return msg.GetImageMessage() != nil ||
		msg.GetVideoMessage() != nil ||
		msg.GetStickerMessage() != nil ||
		(msg.GetDocumentMessage() != nil && isImageOrVideoMime(msg.GetDocumentMessage().GetMimetype()))
}

func detectStickerSourceType(msg *waProto.Message) (mediaType string, isAnimated bool) {
	if msg == nil {
		return "image", false
	}
	if st := msg.GetStickerMessage(); st != nil {
		return "sticker", st.GetIsAnimated()
	}
	if vid := msg.GetVideoMessage(); vid != nil {
		return "video", true
	}
	if doc := msg.GetDocumentMessage(); doc != nil {
		mt := strings.ToLower(doc.GetMimetype())
		if strings.HasPrefix(mt, "video/") || strings.Contains(mt, "gif") {
			return "video", true
		}
		return "image", false
	}
	return "image", false
}

func isImageOrVideoMime(mt string) bool {
	mt = strings.ToLower(strings.TrimSpace(mt))
	return strings.HasPrefix(mt, "image/") || strings.HasPrefix(mt, "video/")
}

func parseStickerMetadata(rawArgs string) (pack, author string) {
	clean := strings.TrimSpace(rawArgs)
	if clean == "" {
		return defaultStickerPack, defaultStickerAuthor
	}

	parts := strings.SplitN(clean, "|", 2)
	pack = strings.TrimSpace(parts[0])
	if pack == "" {
		pack = defaultStickerPack
	}

	if len(parts) > 1 {
		author = strings.TrimSpace(parts[1])
	}
	if author == "" {
		author = defaultStickerAuthor
	}

	return pack, author
}

func buildStickerExif(packName, author string) []byte {
	jsonObj := map[string]interface{}{
		"sticker-pack-id":        "gowa-sticker-" + time.Now().Format("20060102150405"),
		"sticker-pack-name":      packName,
		"sticker-pack-publisher": author,
		"emojis":                 []string{"🤖", "✨"},
	}

	jsonBytes, err := json.Marshal(jsonObj)
	if err != nil {
		jsonBytes = []byte(`{"sticker-pack-name":"Gowa Bot","sticker-pack-publisher":"Astralune"}`)
	}

	header := []byte{
		0x49, 0x49, 0x2A, 0x00,
		0x08, 0x00, 0x00, 0x00,
		0x01, 0x00,
		0x41, 0x57,
		0x07, 0x00,
		0x00, 0x00, 0x00, 0x00,
		0x16, 0x00, 0x00, 0x00,
	}

	binary.LittleEndian.PutUint32(header[14:18], uint32(len(jsonBytes)))

	buf := new(bytes.Buffer)
	buf.Write(header)
	buf.Write(jsonBytes)
	return buf.Bytes()
}

func downloadStickerURL(ctx context.Context, rawURL string) ([]byte, string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", false, fmt.Errorf("URL tidak valid: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Gowa/1.0")

	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", false, fmt.Errorf("gagal menghubungi host URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", false, fmt.Errorf("server URL mengembalikan status HTTP %d", resp.StatusCode)
	}

	lr := io.LimitReader(resp.Body, maxStickerInputBytes+1)
	data, err := io.ReadAll(lr)
	if err != nil {
		return nil, "", false, fmt.Errorf("gagal membaca data URL: %w", err)
	}
	if int64(len(data)) > maxStickerInputBytes {
		return nil, "", false, fmt.Errorf("ukuran file URL melebihi batas 25 MB")
	}

	ct := resp.Header.Get("Content-Type")
	if idx := strings.Index(ct, ";"); idx != -1 {
		ct = strings.TrimSpace(ct[:idx])
	}
	if ct == "" {
		ct = http.DetectContentType(data)
	}

	ct = strings.ToLower(ct)
	if strings.Contains(ct, "gif") || strings.HasPrefix(ct, "video/") {
		return data, "video", true, nil
	}
	if ct == "image/webp" {
		return data, "sticker", false, nil
	}
	return data, "image", false, nil
}
