package general

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"gowa/internal/command"
)

var (
	urlRegex    = regexp.MustCompile(`(?i)\bhttps?://[^\s<>"'` + "`" + `\)]+`)
	wwwRegex    = regexp.MustCompile(`(?i)\bwww\.[^\s<>"'` + "`" + `\)]+`)
	domainRegex = regexp.MustCompile(`(?i)\b[a-zA-Z0-9][-a-zA-Z0-9]*\.[a-zA-Z]{2,}(/[^\s<>"'` + "`" + `\)]*)?`)
)

const maxDownloadSize = 50 * 1024 * 1024

func init() {
	command.Register(&command.Command{
		Name:    "get",
		Cmd:     "get",
		Categ:   "general",
		Desc:    "HTTP GET URL: tampilkan teks/JSON lengkap atau kirim media/file semua ekstensi",
		Alias:   []string{"fetch"},
		Execute: handleGet,
	})
}

func handleGet(ctx *command.Context) error {
	ctx.Typing()

	targetURL := extractTargetURL(ctx)
	if targetURL == "" {
		return ctx.Reply(fmt.Sprintf(
			"*Cara Penggunaan .get:*\n"+
				"• `%sget <url>`\n"+
				"• Atau balas pesan yang berisi link URL dengan `%sget`\n\n"+
				"*Contoh:*\n"+
				"• `%sget https://api.ipify.org?format=json` (Teks/JSON)\n"+
				"• `%sget https://example.com/foto.jpg` (Gambar)\n"+
				"• `%sget https://example.com/video.mp4` (Video)\n"+
				"• `%sget https://example.com/audio.mp3` (Audio)\n"+
				"• `%sget https://example.com/berkas.pdf` (Semua Ekstensi Dokumen/File)",
			ctx.BotPrefix, ctx.BotPrefix, ctx.BotPrefix, ctx.BotPrefix, ctx.BotPrefix, ctx.BotPrefix, ctx.BotPrefix,
		))
	}

	reqCtx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, targetURL, nil)
	if err != nil {
		return ctx.Reply("❌ Gagal membuat permintaan HTTP: " + err.Error())
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "*/*")

	client := &http.Client{
		Timeout: 90 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return ctx.Reply("❌ Gagal menghubungi URL: " + err.Error())
	}
	defer resp.Body.Close()

	limitR := io.LimitReader(resp.Body, maxDownloadSize+1)
	bodyBytes, err := io.ReadAll(limitR)
	if err != nil {
		return ctx.Reply("❌ Gagal membaca respon dari server: " + err.Error())
	}

	if len(bodyBytes) > maxDownloadSize {
		return ctx.Reply("❌ Ukuran data melebihi batas maksimum 50 MB.")
	}

	contentType := resp.Header.Get("Content-Type")
	mediaType, _, _ := mime.ParseMediaType(contentType)
	if mediaType == "" {
		mediaType = http.DetectContentType(bodyBytes)
	}

	fileName := resolveFileName(targetURL, resp.Header.Get("Content-Disposition"), mediaType)
	fileSizeFormatted := formatByteSize(int64(len(bodyBytes)))

	if isTextContent(mediaType, bodyBytes) {
		formattedText := string(bodyBytes)

		if strings.Contains(mediaType, "json") || (len(bodyBytes) > 0 && (bodyBytes[0] == '{' || bodyBytes[0] == '[')) {
			var prettyBuf bytes.Buffer
			if err := json.Indent(&prettyBuf, bodyBytes, "", "  "); err == nil {
				formattedText = prettyBuf.String()
			}
		}

		if len(formattedText) <= 40000 {
			msg := fmt.Sprintf(
				"🌐 *HTTP GET Response*\n"+
					"• *Status:* %d %s\n"+
					"• *Type:* `%s`\n"+
					"• *Size:* %s\n\n"+
					"```\n%s\n```",
				resp.StatusCode,
				http.StatusText(resp.StatusCode),
				mediaType,
				fileSizeFormatted,
				formattedText,
			)
			return ctx.Reply(msg)
		}

		docCaption := fmt.Sprintf(
			"🌐 *HTTP GET Response (Dokumen)*\n"+
				"• *Status:* %d %s\n"+
				"• *Type:* `%s`\n"+
				"• *Size:* %s\n"+
				"_(Panjang teks: %d karakter, dikirim sebagai berkas)_",
			resp.StatusCode,
			http.StatusText(resp.StatusCode),
			mediaType,
			fileSizeFormatted,
			len(formattedText),
		)

		textFileName := fileName
		if !strings.HasSuffix(strings.ToLower(textFileName), ".txt") && !strings.HasSuffix(strings.ToLower(textFileName), ".json") {
			if strings.Contains(mediaType, "json") {
				textFileName += ".json"
			} else {
				textFileName += ".txt"
			}
		}

		return ctx.ReplyDocument(bodyBytes, textFileName, mediaType, docCaption)
	}

	caption := fmt.Sprintf(
		"📦 *Fetched Media / File*\n"+
			"• *File:* `%s`\n"+
			"• *Status:* %d %s\n"+
			"• *Type:* `%s`\n"+
			"• *Size:* %s",
		fileName,
		resp.StatusCode,
		http.StatusText(resp.StatusCode),
		mediaType,
		fileSizeFormatted,
	)

	switch {
	case mediaType == "image/jpeg" || mediaType == "image/png" || mediaType == "image/webp":
		return ctx.ReplyImage(bodyBytes, mediaType, caption)

	case mediaType == "video/mp4" || strings.HasSuffix(strings.ToLower(fileName), ".mp4"):
		return ctx.ReplyVideo(bodyBytes, "video/mp4", caption)

	case mediaType == "audio/mpeg" || mediaType == "audio/ogg" || mediaType == "audio/mp4":
		return ctx.ReplyAudio(bodyBytes, mediaType, false)

	default:
		return ctx.ReplyDocument(bodyBytes, fileName, mediaType, caption)
	}
}

func findURLInText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if match := urlRegex.FindString(text); match != "" {
		return cleanURL(match)
	}
	if match := wwwRegex.FindString(text); match != "" {
		return cleanURL("https://" + match)
	}
	if match := domainRegex.FindString(text); match != "" {
		return cleanURL("https://" + match)
	}
	return ""
}

func cleanURL(u string) string {
	u = strings.TrimRight(u, ".,;:!?)>}\"'`")
	return u
}

func extractTargetURL(ctx *command.Context) string {
	if ctx.RawArgs != "" {
		if u := findURLInText(ctx.RawArgs); u != "" {
			return u
		}
	}

	quotedMsg := ctx.GetQuotedMessage()
	if quotedMsg != nil {
		if ext := quotedMsg.GetExtendedTextMessage(); ext != nil {
			if u := findURLInText(ext.GetText()); u != "" {
				return u
			}
			if u := findURLInText(ext.GetMatchedText()); u != "" {
				return u
			}
			if u := findURLInText(ext.GetVideoContentURL()); u != "" {
				return u
			}
			if ext.GetContextInfo() != nil && ext.GetContextInfo().GetExternalAdReply() != nil {
				if u := findURLInText(ext.GetContextInfo().GetExternalAdReply().GetSourceURL()); u != "" {
					return u
				}
			}
		}
		if conv := quotedMsg.GetConversation(); conv != "" {
			if u := findURLInText(conv); u != "" {
				return u
			}
		}
		if img := quotedMsg.GetImageMessage(); img != nil {
			if u := findURLInText(img.GetCaption()); u != "" {
				return u
			}
		}
		if vid := quotedMsg.GetVideoMessage(); vid != nil {
			if u := findURLInText(vid.GetCaption()); u != "" {
				return u
			}
		}
		if doc := quotedMsg.GetDocumentMessage(); doc != nil {
			if u := findURLInText(doc.GetCaption()); u != "" {
				return u
			}
		}
	}

	ci := ctx.GetContextInfo()
	if ci != nil && ci.GetQuotedMessage() != nil {
		rawQuoted := command.UnwrapMessage(ci.GetQuotedMessage())
		if rawQuoted != nil {
			if ext := rawQuoted.GetExtendedTextMessage(); ext != nil {
				if u := findURLInText(ext.GetText()); u != "" {
					return u
				}
				if u := findURLInText(ext.GetMatchedText()); u != "" {
					return u
				}
			}
			if conv := rawQuoted.GetConversation(); conv != "" {
				if u := findURLInText(conv); u != "" {
					return u
				}
			}
		}
	}

	return ""
}

func resolveFileName(targetURL, contentDisp, mediaType string) string {
	if contentDisp != "" {
		if _, params, err := mime.ParseMediaType(contentDisp); err == nil {
			if fn, ok := params["filename"]; ok && strings.TrimSpace(fn) != "" {
				return path.Base(strings.TrimSpace(fn))
			}
		}
	}

	if u, err := url.Parse(targetURL); err == nil {
		base := path.Base(u.Path)
		if base != "" && base != "." && base != "/" {
			return base
		}
	}

	exts, _ := mime.ExtensionsByType(mediaType)
	if len(exts) > 0 {
		return "file" + exts[0]
	}

	return "file.bin"
}

func isTextContent(mediaType string, body []byte) bool {
	if strings.HasPrefix(mediaType, "text/") {
		return true
	}
	if mediaType == "application/json" || strings.HasSuffix(mediaType, "+json") {
		return true
	}
	if mediaType == "application/xml" || strings.HasSuffix(mediaType, "+xml") {
		return true
	}
	if mediaType == "application/javascript" || mediaType == "application/x-javascript" {
		return true
	}
	if mediaType == "application/x-yaml" || mediaType == "application/yaml" {
		return true
	}
	if mediaType == "application/sql" || mediaType == "application/graphql" {
		return true
	}

	if len(body) > 0 && len(body) <= 512 {
		if bytes.IndexByte(body, 0) == -1 && utf8.Valid(body) {
			return true
		}
	} else if len(body) > 512 {
		if bytes.IndexByte(body[:512], 0) == -1 && utf8.Valid(body[:512]) {
			return true
		}
	}

	return false
}

func formatByteSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
