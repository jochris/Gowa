package general

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"gowa/internal/command"
)

var (
	urlHostRegex    = regexp.MustCompile(`(?i)\bhttps?://[^\s<>"'` + "`" + `\)]+`)
	ipv4Regex       = regexp.MustCompile(`\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`)
	domainHostRegex = regexp.MustCompile(`\b([a-zA-Z0-9][-a-zA-Z0-9]*\.)+[a-zA-Z]{2,}\b`)
)

type IPInfoResponse struct {
	Status      string  `json:"status"`
	Message     string  `json:"message"`
	Country     string  `json:"country"`
	CountryCode string  `json:"countryCode"`
	Region      string  `json:"region"`
	RegionName  string  `json:"regionName"`
	City        string  `json:"city"`
	Zip         string  `json:"zip"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
	Timezone    string  `json:"timezone"`
	ISP         string  `json:"isp"`
	Org         string  `json:"org"`
	AS          string  `json:"as"`
	Mobile      bool    `json:"mobile"`
	Proxy       bool    `json:"proxy"`
	Hosting     bool    `json:"hosting"`
	Query       string  `json:"query"`
}

func init() {
	command.Register(&command.Command{
		Name:    "ip",
		Cmd:     "ip",
		Categ:   "general",
		Desc:    "Lacak informasi lokasi, ISP, dan tipe jaringan IP/Domain (via argumen/reply)",
		Alias:   []string{"geoip", "whoisip"},
		Execute: handleIP,
	})
}

func handleIP(ctx *command.Context) error {
	ctx.Typing()

	targetHost := extractTargetHost(ctx)
	if targetHost == "" {
		return ctx.Reply(fmt.Sprintf(
			"*Cara Penggunaan .ip:*\n"+
				"• `%sip <ip/domain>`\n"+
				"• Atau balas pesan yang berisi IP/Domain/Link dengan `%sip`\n\n"+
				"*Contoh:*\n"+
				"• `%sip 8.8.8.8`\n"+
				"• `%sip 1.1.1.1`\n"+
				"• `%sip google.com`\n"+
				"• `%sip https://github.com`",
			ctx.BotPrefix, ctx.BotPrefix, ctx.BotPrefix, ctx.BotPrefix, ctx.BotPrefix, ctx.BotPrefix,
		))
	}

	reqCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	apiURL := fmt.Sprintf("http://ip-api.com/json/%s?fields=status,message,country,countryCode,region,regionName,city,zip,lat,lon,timezone,isp,org,as,query,mobile,proxy,hosting", url.PathEscape(targetHost))

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, apiURL, nil)
	if err != nil {
		return ctx.Reply("❌ Gagal membuat permintaan: " + err.Error())
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 Gowa-Bot/1.0")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ctx.Reply("❌ Gagal menghubungi server IP lookup: " + err.Error())
	}
	defer resp.Body.Close()

	var ipInfo IPInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&ipInfo); err != nil {
		return ctx.Reply("❌ Gagal membaca respon data IP.")
	}

	if strings.ToLower(ipInfo.Status) != "success" {
		errMsg := ipInfo.Message
		if errMsg == "" {
			errMsg = "Target tidak ditemukan atau format IP/domain tidak valid."
		}
		return ctx.Reply(fmt.Sprintf("❌ Pelacakan gagal untuk `%s`:\n• Pesan: %s", targetHost, errMsg))
	}

	mobileStatus := "❌ Tidak"
	if ipInfo.Mobile {
		mobileStatus = "✅ Ya (Data Seluler)"
	}

	proxyStatus := "❌ Tidak"
	if ipInfo.Proxy {
		proxyStatus = "⚠️ Ya (VPN / Proxy)"
	}

	hostingStatus := "❌ Tidak"
	if ipInfo.Hosting {
		hostingStatus = "🏢 Ya (Data Center / Hosting)"
	}

	locationParts := []string{}
	if ipInfo.City != "" {
		locationParts = append(locationParts, ipInfo.City)
	}
	if ipInfo.RegionName != "" {
		locationParts = append(locationParts, ipInfo.RegionName)
	}
	if ipInfo.Zip != "" {
		locationParts = append(locationParts, fmt.Sprintf("(Kode Pos: %s)", ipInfo.Zip))
	}
	locationStr := strings.Join(locationParts, ", ")
	if locationStr == "" {
		locationStr = "-"
	}

	mapsURL := fmt.Sprintf("https://www.google.com/maps?q=%.4f,%.4f", ipInfo.Lat, ipInfo.Lon)

	var sb strings.Builder
	sb.WriteString("🌐 *IP & Geolocation Lookup*\n\n")
	sb.WriteString(fmt.Sprintf("• *Target:* `%s`\n", targetHost))
	sb.WriteString(fmt.Sprintf("• *IP Address:* `%s`\n", ipInfo.Query))
	sb.WriteString(fmt.Sprintf("• *Negara:* %s %s (%s)\n", countryFlag(ipInfo.CountryCode), ipInfo.Country, ipInfo.CountryCode))
	sb.WriteString(fmt.Sprintf("• *Wilayah / Kota:* %s\n", locationStr))
	if ipInfo.Timezone != "" {
		sb.WriteString(fmt.Sprintf("• *Zona Waktu:* %s\n", ipInfo.Timezone))
	}
	if ipInfo.ISP != "" {
		sb.WriteString(fmt.Sprintf("• *ISP / Provider:* %s\n", ipInfo.ISP))
	}
	if ipInfo.Org != "" && ipInfo.Org != ipInfo.ISP {
		sb.WriteString(fmt.Sprintf("• *Organisasi:* %s\n", ipInfo.Org))
	}
	if ipInfo.AS != "" {
		sb.WriteString(fmt.Sprintf("• *ASN:* %s\n", ipInfo.AS))
	}
	sb.WriteString("• *Karakteristik Jaringan:*\n")
	sb.WriteString(fmt.Sprintf("  - Seluler: %s\n", mobileStatus))
	sb.WriteString(fmt.Sprintf("  - Proxy / VPN: %s\n", proxyStatus))
	sb.WriteString(fmt.Sprintf("  - Server Hosting: %s\n", hostingStatus))
	if ipInfo.Lat != 0 || ipInfo.Lon != 0 {
		sb.WriteString(fmt.Sprintf("• *Koordinat:* `%.4f, %.4f`\n", ipInfo.Lat, ipInfo.Lon))
		sb.WriteString(fmt.Sprintf("• *Google Maps:* %s", mapsURL))
	}

	return ctx.Reply(strings.TrimSpace(sb.String()))
}

func extractHost(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		if u, err := url.Parse(raw); err == nil && u.Hostname() != "" {
			return u.Hostname()
		}
	}

	if match := urlHostRegex.FindString(raw); match != "" {
		if u, err := url.Parse(match); err == nil && u.Hostname() != "" {
			return u.Hostname()
		}
	}

	if match := ipv4Regex.FindString(raw); match != "" {
		if ip := net.ParseIP(match); ip != nil {
			return match
		}
	}

	if match := domainHostRegex.FindString(raw); match != "" {
		clean := strings.TrimRight(match, ".,;:!?")
		if strings.Contains(clean, ".") {
			return clean
		}
	}

	host := raw
	if strings.Contains(host, "/") {
		host = strings.SplitN(host, "/", 2)[0]
	}
	if strings.Contains(host, ":") {
		host = strings.SplitN(host, ":", 2)[0]
	}

	return strings.Trim(host, "[]()<>\"'`")
}

func extractTargetHost(ctx *command.Context) string {
	if ctx.RawArgs != "" {
		if host := extractHost(ctx.RawArgs); host != "" {
			return host
		}
	}

	quotedMsg := ctx.GetQuotedMessage()
	if quotedMsg != nil {
		if ext := quotedMsg.GetExtendedTextMessage(); ext != nil {
			if h := extractHost(ext.GetText()); h != "" {
				return h
			}
			if h := extractHost(ext.GetMatchedText()); h != "" {
				return h
			}
		}
		if conv := quotedMsg.GetConversation(); conv != "" {
			if h := extractHost(conv); h != "" {
				return h
			}
		}
		if img := quotedMsg.GetImageMessage(); img != nil {
			if h := extractHost(img.GetCaption()); h != "" {
				return h
			}
		}
		if vid := quotedMsg.GetVideoMessage(); vid != nil {
			if h := extractHost(vid.GetCaption()); h != "" {
				return h
			}
		}
		if doc := quotedMsg.GetDocumentMessage(); doc != nil {
			if h := extractHost(doc.GetCaption()); h != "" {
				return h
			}
		}
	}

	ci := ctx.GetContextInfo()
	if ci != nil && ci.GetQuotedMessage() != nil {
		rawQuoted := command.UnwrapMessage(ci.GetQuotedMessage())
		if rawQuoted != nil {
			if ext := rawQuoted.GetExtendedTextMessage(); ext != nil {
				if h := extractHost(ext.GetText()); h != "" {
					return h
				}
				if h := extractHost(ext.GetMatchedText()); h != "" {
					return h
				}
			}
			if conv := rawQuoted.GetConversation(); conv != "" {
				if h := extractHost(conv); h != "" {
					return h
				}
			}
		}
	}

	return ""
}

func countryFlag(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	if len(code) != 2 {
		return "🌐"
	}
	r1 := rune(code[0]) - 'A' + 0x1F1E6
	r2 := rune(code[1]) - 'A' + 0x1F1E6
	if r1 >= 0x1F1E6 && r1 <= 0x1F1FF && r2 >= 0x1F1E6 && r2 <= 0x1F1FF {
		return string([]rune{r1, r2})
	}
	return "🌐"
}
