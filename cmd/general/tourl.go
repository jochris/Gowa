package general

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"gowa/internal/command"

	waProto "go.mau.fi/whatsmeow/proto/waE2E"
)

const (
	defaultLuneeAPIKey = "lune_live_303497fe31f347d77cd35e043952ef3d118dd933ecbe6628"
	luneeUploadURL     = "https://direct.lunee.lol/api/upload"
	maxUploadSize      = 50 * 1024 * 1024
)

type LuneeUploadResponse struct {
	Success           bool   `json:"success"`
	ID                string `json:"id"`
	UserID            string `json:"user_id"`
	FileName          string `json:"file_name"`
	FileSize          int64  `json:"file_size"`
	MimeType          string `json:"mime_type"`
	URL               string `json:"url"`
	DownloadURL       string `json:"download_url"`
	ShortURL          string `json:"short_url"`
	DeletionToken     string `json:"deletion_token"`
	BurnAfterDownload bool   `json:"burn_after_download"`
	Error             string `json:"error"`
	Message           string `json:"message"`
}

func init() {
	command.Register(&command.Command{
		Name:    "tourl",
		Cmd:     "tourl",
		Categ:   "general",
		Desc:    "Upload media (gambar, video, audio, berkas, stiker) atau URL ke Lunee",
		Alias:   []string{"upload", "up", "tolunee"},
		Execute: handleToURL,
	})
}

func handleToURL(ctx *command.Context) error {
	ctx.Typing()

	c := ctx.Ctx
	if c == nil {
		c = context.Background()
	}

	var targetMsg *waProto.Message
	if ctx.Event != nil && ctx.Event.Message != nil {
		unwrapped := command.UnwrapMessage(ctx.Event.Message)
		if isMediaMessage(unwrapped) {
			targetMsg = unwrapped
		}
	}
	if targetMsg == nil {
		quoted := ctx.GetQuotedMessage()
		if quoted != nil && isMediaMessage(quoted) {
			targetMsg = quoted
		}
	}

	var (
		fileData []byte
		fileName string
		mimeType string
		err      error
	)

	if targetMsg != nil {
		fileName, mimeType = detectMediaInfo(targetMsg)

		fileData, err = ctx.DownloadMedia(targetMsg)
		if err != nil {
			return ctx.Reply("❌ Gagal mengunduh media dari WhatsApp: " + err.Error())
		}
	} else if len(ctx.Args) > 0 {
		rawURL := strings.TrimSpace(ctx.RawArgs)
		if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
			dlCtx, dlCancel := context.WithTimeout(c, 90*time.Second)
			defer dlCancel()

			fileData, fileName, mimeType, err = downloadWebURL(dlCtx, rawURL)
			if err != nil {
				return ctx.Reply("❌ Gagal mengunduh file dari URL: " + err.Error())
			}
		}
	}

	if len(fileData) == 0 {
		return ctx.Reply(fmt.Sprintf(
			"❌ Tidak ditemukan lampiran media atau link untuk di-upload.\n\n"+
				"*Cara Penggunaan:*\n"+
				"- Balas media (foto, video, audio, berkas, stiker) dengan: `%stourl`\n"+
				"- Kirim media langsung dengan caption: `%stourl`\n"+
				"- Masukkan URL langsung: `%stourl https://example.com/file.zip`",
			ctx.BotPrefix, ctx.BotPrefix, ctx.BotPrefix,
		))
	}

	if int64(len(fileData)) > maxUploadSize {
		return ctx.Reply(fmt.Sprintf("❌ Ukuran file (%s) melebihi batas maksimum 50 MB.", formatBytes(int64(len(fileData)))))
	}

	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = http.DetectContentType(fileData)
	}

	apiKey := defaultLuneeAPIKey
	if ctx.Config != nil && ctx.Config.LuneeAPIKey != "" {
		apiKey = ctx.Config.LuneeAPIKey
	}

	upCtx, upCancel := context.WithTimeout(c, 90*time.Second)
	defer upCancel()

	result, err := uploadToLunee(upCtx, fileData, fileName, apiKey)
	if err != nil {
		return ctx.Reply("❌ Gagal mengunggah file ke Lunee: " + err.Error())
	}

	downloadLink := result.DownloadURL
	if downloadLink == "" {
		downloadLink = result.URL
	}

	var sb strings.Builder
	sb.WriteString("*File Uploaded Successfully*\n\n")
	sb.WriteString(fmt.Sprintf("- *File Name:* %s\n", result.FileName))
	sb.WriteString(fmt.Sprintf("- *Download Link:* %s", downloadLink))

	return ctx.Reply(sb.String())
}

func isMediaMessage(msg *waProto.Message) bool {
	if msg == nil {
		return false
	}
	return msg.GetImageMessage() != nil ||
		msg.GetVideoMessage() != nil ||
		msg.GetAudioMessage() != nil ||
		msg.GetDocumentMessage() != nil ||
		msg.GetStickerMessage() != nil
}

func detectMediaInfo(msg *waProto.Message) (fileName, mimeType string) {
	if msg == nil {
		return "file.bin", "application/octet-stream"
	}
	ts := time.Now().Format("20060102_150405")
	if doc := msg.GetDocumentMessage(); doc != nil {
		fn := doc.GetFileName()
		mt := doc.GetMimetype()
		if fn == "" {
			ext := extensionFromMime(mt)
			fn = fmt.Sprintf("document_%s%s", ts, ext)
		}
		return fn, mt
	}
	if img := msg.GetImageMessage(); img != nil {
		mt := img.GetMimetype()
		ext := extensionFromMime(mt)
		if ext == "" {
			ext = ".jpg"
		}
		return fmt.Sprintf("image_%s%s", ts, ext), mt
	}
	if vid := msg.GetVideoMessage(); vid != nil {
		mt := vid.GetMimetype()
		ext := extensionFromMime(mt)
		if ext == "" {
			ext = ".mp4"
		}
		return fmt.Sprintf("video_%s%s", ts, ext), mt
	}
	if aud := msg.GetAudioMessage(); aud != nil {
		mt := aud.GetMimetype()
		ext := extensionFromMime(mt)
		if ext == "" {
			ext = ".mp3"
		}
		return fmt.Sprintf("audio_%s%s", ts, ext), mt
	}
	if st := msg.GetStickerMessage(); st != nil {
		return fmt.Sprintf("sticker_%s.webp", ts), "image/webp"
	}
	return fmt.Sprintf("file_%s.bin", ts), "application/octet-stream"
}

func downloadWebURL(ctx context.Context, rawURL string) ([]byte, string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", "", fmt.Errorf("URL tidak valid: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Gowa/1.0")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", "", fmt.Errorf("gagal menghubungi host URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", "", fmt.Errorf("server mengembalikan status HTTP %d", resp.StatusCode)
	}

	lr := io.LimitReader(resp.Body, maxUploadSize+1)
	data, err := io.ReadAll(lr)
	if err != nil {
		return nil, "", "", fmt.Errorf("gagal membaca data URL: %w", err)
	}
	if int64(len(data)) > maxUploadSize {
		return nil, "", "", fmt.Errorf("ukuran file melebihi batas 50 MB")
	}

	mimeType := resp.Header.Get("Content-Type")
	if idx := strings.Index(mimeType, ";"); idx != -1 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}
	if mimeType == "" {
		mimeType = http.DetectContentType(data)
	}

	fileName := ""
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if _, params, err := mime.ParseMediaType(cd); err == nil {
			if fn, ok := params["filename"]; ok && fn != "" {
				fileName = fn
			}
		}
	}
	if fileName == "" {
		if parsed, err := url.Parse(rawURL); err == nil {
			base := path.Base(parsed.Path)
			if base != "" && base != "." && base != "/" {
				fileName = base
			}
		}
	}
	if fileName == "" {
		ext := extensionFromMime(mimeType)
		if ext == "" {
			ext = ".bin"
		}
		fileName = fmt.Sprintf("file_%s%s", time.Now().Format("20060102_150405"), ext)
	}

	return data, fileName, mimeType, nil
}

func uploadToLunee(ctx context.Context, fileData []byte, fileName, apiKey string) (*LuneeUploadResponse, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return nil, fmt.Errorf("gagal membuat form file: %w", err)
	}

	if _, err := part.Write(fileData); err != nil {
		return nil, fmt.Errorf("gagal menulis data file: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("gagal menutup multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, luneeUploadURL, &body)
	if err != nil {
		return nil, fmt.Errorf("gagal membuat request upload: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gagal upload ke server: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gagal membaca respon server: %w", err)
	}

	var result LuneeUploadResponse
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return nil, fmt.Errorf("gagal decode respon JSON (HTTP %d): %s", resp.StatusCode, string(respBytes))
	}

	if !result.Success {
		errMsg := result.Error
		if errMsg == "" {
			errMsg = result.Message
		}
		if errMsg == "" {
			errMsg = fmt.Sprintf("HTTP status %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("server menolak upload: %s", errMsg)
	}

	return &result, nil
}

func extensionFromMime(mt string) string {
	mt = strings.ToLower(strings.TrimSpace(mt))
	if idx := strings.Index(mt, ";"); idx != -1 {
		mt = strings.TrimSpace(mt[:idx])
	}
	switch mt {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "video/mp4":
		return ".mp4"
	case "video/mkv", "video/x-matroska":
		return ".mkv"
	case "audio/mpeg", "audio/mp3":
		return ".mp3"
	case "audio/ogg", "audio/opus":
		return ".ogg"
	case "audio/wav":
		return ".wav"
	case "application/pdf":
		return ".pdf"
	case "application/zip":
		return ".zip"
	case "application/x-rar-compressed":
		return ".rar"
	case "application/json":
		return ".json"
	case "text/plain":
		return ".txt"
	default:
		if exts, err := mime.ExtensionsByType(mt); err == nil && len(exts) > 0 {
			return exts[0]
		}
		return ""
	}
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
