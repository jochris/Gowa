package media

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

type ThumbnailItem struct {
	URL    string `json:"url"`
	Width  int64  `json:"width,omitempty"`
	Height int64  `json:"height,omitempty"`
}

type TrackItem struct {
	VideoID    string          `json:"video_id,omitempty"`
	Title      string          `json:"title"`
	Artist     string          `json:"artist,omitempty"`
	Album      string          `json:"album,omitempty"`
	Duration   string          `json:"duration,omitempty"`
	Thumbnails []ThumbnailItem `json:"thumbnails,omitempty"`
	URL        string          `json:"url,omitempty"`
}

type SearchResult struct {
	Query     string      `json:"query"`
	TopResult any         `json:"top_result,omitempty"`
	Songs     []TrackItem `json:"songs,omitempty"`
}

type AudioStreamItem struct {
	Format      string `json:"format"`
	Quality     string `json:"quality"`
	FileType    string `json:"file_type"`
	DownloadURL string `json:"download_url"`
	Status      string `json:"status"`
}

type YTMusicEngine struct {
	client        *http.Client
	dlAPIKey      string
	primaryDLDom  string
	fallbackDLDom string
}

func NewYTMusicEngine(timeout time.Duration) (*YTMusicEngine, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	transport := &http.Transport{
		TLSClientConfig:    &tls.Config{InsecureSkipVerify: true},
		Proxy:              http.ProxyFromEnvironment,
		MaxIdleConns:       100,
		IdleConnTimeout:    90 * time.Second,
		DisableCompression: false,
		ForceAttemptHTTP2:  true,
	}

	client := &http.Client{
		Jar:       jar,
		Transport: transport,
		Timeout:   timeout,
	}

	return &YTMusicEngine{
		client:        client,
		dlAPIKey:      "dfcb6d76f2f6a9894gjkege8a4ab232222",
		primaryDLDom:  "p.savenow.to",
		fallbackDLDom: "p.lbserver.xyz",
	}, nil
}

func (y *YTMusicEngine) applyHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "id-ID,id;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Origin", "https://music.youtube.com")
	req.Header.Set("Referer", "https://music.youtube.com/")
	req.Header.Set("Sec-Ch-Ua", `"Google Chrome";v="131", "Chromium";v="131", "Not_A Brand";v="24"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"Windows"`)
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
}

func (y *YTMusicEngine) postInnerTube(ctx context.Context, endpoint string, payload map[string]any) (gjson.Result, error) {
	fullURL := "https://music.youtube.com/youtubei/v1/" + strings.TrimPrefix(endpoint, "/")

	if _, ok := payload["context"]; !ok {
		payload["context"] = map[string]any{
			"client": map[string]any{
				"clientName":    "WEB_REMIX",
				"clientVersion": "1.20241001.01.00",
				"hl":            "id",
				"gl":            "ID",
			},
		}
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return gjson.Result{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return gjson.Result{}, err
	}
	y.applyHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := y.client.Do(req)
	if err != nil {
		return gjson.Result{}, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return gjson.Result{}, err
	}

	return gjson.ParseBytes(respBytes), nil
}

func (y *YTMusicEngine) parseThumbnails(raw gjson.Result) []ThumbnailItem {
	var list []ThumbnailItem
	for _, item := range raw.Array() {
		u := item.Get("url").String()
		if u != "" {
			list = append(list, ThumbnailItem{
				URL:    u,
				Width:  item.Get("width").Int(),
				Height: item.Get("height").Int(),
			})
		}
	}
	return list
}

func (y *YTMusicEngine) Search(ctx context.Context, query string) (*SearchResult, error) {
	res, err := y.postInnerTube(ctx, "search", map[string]any{
		"query": query,
	})
	if err != nil {
		return nil, err
	}

	result := &SearchResult{
		Query: query,
	}

	sections := res.Get("contents.tabbedSearchResultsRenderer.tabs.0.tabRenderer.content.sectionListRenderer.contents").Array()
	for _, sec := range sections {
		if card := sec.Get("musicCardShelfRenderer"); card.Exists() {
			title := card.Get("title.runs.0.text").String()
			videoID := card.Get("title.runs.0.navigationEndpoint.watchEndpoint.videoId").String()
			subtitle := card.Get("subtitle.runs").Array()
			var subTexts []string
			for _, st := range subtitle {
				t := strings.TrimSpace(st.Get("text").String())
				if t != "" && t != "•" {
					subTexts = append(subTexts, t)
				}
			}

			topItem := map[string]any{
				"title":      title,
				"video_id":   videoID,
				"url":        "https://music.youtube.com/watch?v=" + videoID,
				"subtitles":  subTexts,
				"thumbnails": y.parseThumbnails(card.Get("thumbnail.musicThumbnailRenderer.thumbnail.thumbnails")),
			}
			result.TopResult = topItem

			if videoID != "" {
				artist := ""
				if len(subTexts) > 0 {
					artist = subTexts[0]
				}
				result.Songs = append(result.Songs, TrackItem{
					VideoID:    videoID,
					Title:      title,
					Artist:     artist,
					Thumbnails: y.parseThumbnails(card.Get("thumbnail.musicThumbnailRenderer.thumbnail.thumbnails")),
					URL:        "https://music.youtube.com/watch?v=" + videoID,
				})
			}
		}

		items := sec.Get("itemSectionRenderer.contents").Array()
		if len(items) == 0 {
			items = sec.Get("musicShelfRenderer.contents").Array()
		}

		for _, item := range items {
			renderer := item.Get("musicResponsiveListItemRenderer")
			if !renderer.Exists() {
				continue
			}

			title := renderer.Get("flexColumns.0.musicResponsiveListItemFlexColumnRenderer.text.runs.0.text").String()
			navEndpoint := renderer.Get("flexColumns.0.musicResponsiveListItemFlexColumnRenderer.text.runs.0.navigationEndpoint")
			watchVideoID := navEndpoint.Get("watchEndpoint.videoId").String()
			if watchVideoID == "" {
				watchVideoID = renderer.Get("playlistItemData.videoId").String()
			}

			subtitleRuns := renderer.Get("flexColumns.1.musicResponsiveListItemFlexColumnRenderer.text.runs").Array()
			var subParts []string
			for _, r := range subtitleRuns {
				t := strings.TrimSpace(r.Get("text").String())
				if t != "" && t != "•" {
					subParts = append(subParts, t)
				}
			}

			itemType := ""
			artistName := ""
			durationStr := ""

			if len(subParts) > 0 {
				itemType = strings.ToLower(subParts[0])
			}
			if len(subParts) > 1 {
				artistName = subParts[1]
			}
			if len(subParts) > 2 && regexp.MustCompile(`^\d+:\d+`).MatchString(subParts[len(subParts)-1]) {
				durationStr = subParts[len(subParts)-1]
			}

			thumbs := y.parseThumbnails(renderer.Get("thumbnail.musicThumbnailRenderer.thumbnail.thumbnails"))

			if watchVideoID != "" || itemType == "lagu" || itemType == "song" || itemType == "video" {
				result.Songs = append(result.Songs, TrackItem{
					VideoID:    watchVideoID,
					Title:      title,
					Artist:     artistName,
					Duration:   durationStr,
					Thumbnails: thumbs,
					URL:        "https://music.youtube.com/watch?v=" + watchVideoID,
				})
			}
		}
	}

	return result, nil
}

func (y *YTMusicEngine) GetTrackInfo(ctx context.Context, videoID string) (*TrackItem, error) {
	cleanID := y.extractVideoID(videoID)
	if cleanID == "" {
		return nil, fmt.Errorf("ID video tidak valid")
	}

	nextRes, err := y.postInnerTube(ctx, "next", map[string]any{
		"videoId": cleanID,
	})
	if err != nil {
		return nil, err
	}

	tabs := nextRes.Get("contents.singleColumnMusicWatchNextResultsRenderer.tabbedRenderer.watchNextTabbedResultsRenderer.tabs").Array()
	for _, tab := range tabs {
		queueContents := tab.Get("tabRenderer.content.musicQueueRenderer.content.playlistPanelRenderer.contents").Array()
		if len(queueContents) > 0 {
			first := queueContents[0].Get("playlistPanelVideoRenderer")
			if first.Exists() {
				title := first.Get("title.runs.0.text").String()
				artist := first.Get("longBylineText.runs.0.text").String()
				duration := first.Get("lengthText.runs.0.text").String()
				thumbs := y.parseThumbnails(first.Get("thumbnail.thumbnails"))

				return &TrackItem{
					VideoID:    cleanID,
					Title:      title,
					Artist:     artist,
					Duration:   duration,
					Thumbnails: thumbs,
					URL:        "https://music.youtube.com/watch?v=" + cleanID,
				}, nil
			}
		}
	}

	return &TrackItem{
		VideoID:  cleanID,
		Title:    "YouTube Audio Track",
		URL:      "https://music.youtube.com/watch?v=" + cleanID,
		Duration: "-",
	}, nil
}

func (y *YTMusicEngine) resolveDLStream(ctx context.Context, targetURL string, format string) (*AudioStreamItem, error) {
	formatQuality := "High Quality Audio"
	fileType := strings.ToLower(format)

	q := url.Values{}
	q.Set("format", format)
	q.Set("url", targetURL)
	q.Set("apikey", y.dlAPIKey)

	domains := []string{y.primaryDLDom, y.fallbackDLDom}
	var initRes gjson.Result
	var lastErr error

	for _, d := range domains {
		ep := fmt.Sprintf("https://%s/api/v2/download?%s", d, q.Encode())
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, ep, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
		req.Header.Set("Origin", "https://loader.to")
		req.Header.Set("Referer", "https://loader.to/")

		resp, err := y.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		res := gjson.ParseBytes(b)
		if res.Get("success").Bool() || res.Get("progress_url").String() != "" {
			initRes = res
			break
		}
	}

	if !initRes.Exists() {
		return nil, fmt.Errorf("dl_service_init_failed: %v", lastErr)
	}

	progressURL := initRes.Get("progress_url").String()
	if progressURL == "" {
		progressURL = fmt.Sprintf("https://%s/api/v2/progress?id=%s", y.primaryDLDom, initRes.Get("id").String())
	}

	for i := 0; i < 25; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, progressURL, nil)
		if err == nil {
			req.Header.Set("User-Agent", "Mozilla/5.0")
			req.Header.Set("Origin", "https://loader.to")
			req.Header.Set("Referer", "https://loader.to/")
			resp, err := y.client.Do(req)
			if err == nil {
				b, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				pRes := gjson.ParseBytes(b)
				dlURL := pRes.Get("download_url").String()
				if (pRes.Get("success").Int() == 1 || pRes.Get("text").String() == "Finished") && dlURL != "" {
					return &AudioStreamItem{
						Format:      format,
						Quality:     formatQuality,
						FileType:    fileType,
						DownloadURL: dlURL,
						Status:      "Available",
					}, nil
				}
				if pRes.Get("text").String() == "Error" || pRes.Get("error").Exists() {
					break
				}
			}
		}
		time.Sleep(1200 * time.Millisecond)
	}

	return nil, fmt.Errorf("dl_stream_generation_timeout")
}

func (y *YTMusicEngine) DownloadAudio(ctx context.Context, videoID string) ([]byte, string, error) {
	cleanID := y.extractVideoID(videoID)
	if cleanID == "" {
		return nil, "", fmt.Errorf("ID video tidak valid")
	}

	targetURL := "https://music.youtube.com/watch?v=" + cleanID

	formats := []string{"mp3"}
	var stream *AudioStreamItem
	var lastErr error

	for _, f := range formats {
		s, err := y.resolveDLStream(ctx, targetURL, f)
		if err == nil && s != nil && s.DownloadURL != "" {
			stream = s
			break
		}
		lastErr = err
	}

	if stream == nil || stream.DownloadURL == "" {
		if lastErr != nil {
			return nil, "", fmt.Errorf("gagal memproses streaming audio: %w", lastErr)
		}
		return nil, "", fmt.Errorf("tidak ada stream audio yang tersedia")
	}

	dlReq, err := http.NewRequestWithContext(ctx, http.MethodGet, stream.DownloadURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("gagal membuat permintaan unduh: %w", err)
	}
	dlReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	dlResp, err := y.client.Do(dlReq)
	if err != nil {
		return nil, "", fmt.Errorf("gagal mengunduh audio: %w", err)
	}
	defer dlResp.Body.Close()

	if dlResp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("server audio mengembalikan HTTP %d", dlResp.StatusCode)
	}

	const maxAudioBytes = 70 * 1024 * 1024
	lr := io.LimitReader(dlResp.Body, maxAudioBytes+1)
	audioData, err := io.ReadAll(lr)
	if err != nil {
		return nil, "", fmt.Errorf("gagal membaca stream audio: %w", err)
	}
	if int64(len(audioData)) > maxAudioBytes {
		return nil, "", fmt.Errorf("ukuran audio terlalu besar (maksimal 70 MB)")
	}

	return audioData, stream.FileType, nil
}

func (y *YTMusicEngine) extractVideoID(raw string) string {
	clean := strings.TrimSpace(raw)
	if len(clean) == 11 && regexp.MustCompile(`^[a-zA-Z0-9_-]{11}$`).MatchString(clean) {
		return clean
	}
	reV := regexp.MustCompile(`[?&]v=([a-zA-Z0-9_-]{11})`)
	if m := reV.FindStringSubmatch(clean); len(m) > 1 {
		return m[1]
	}
	reShort := regexp.MustCompile(`youtu\.be/([a-zA-Z0-9_-]{11})`)
	if m := reShort.FindStringSubmatch(clean); len(m) > 1 {
		return m[1]
	}
	reShorts := regexp.MustCompile(`/shorts/([a-zA-Z0-9_-]{11})`)
	if m := reShorts.FindStringSubmatch(clean); len(m) > 1 {
		return m[1]
	}
	return ""
}

func convertAudioToOpus(ctx context.Context, inputData []byte) ([]byte, uint32, error) {
	tmpDir, err := os.MkdirTemp("", "gowa_play_*")
	if err != nil {
		return nil, 0, err
	}
	defer os.RemoveAll(tmpDir)

	inPath := filepath.Join(tmpDir, "input.bin")
	outPath := filepath.Join(tmpDir, "output.opus")

	if err := os.WriteFile(inPath, inputData, 0644); err != nil {
		return nil, 0, err
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", "-i", inPath, "-vn", "-c:a", "libopus", "-b:a", "128k", "-vbr", "on", "-f", "ogg", "-y", outPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, 0, fmt.Errorf("ffmpeg error: %v, output: %s", err, string(out))
	}

	var durSec uint32
	probeCmd := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", outPath)
	if pOut, err := probeCmd.Output(); err == nil {
		var f float64
		if _, scanErr := fmt.Sscanf(strings.TrimSpace(string(pOut)), "%f", &f); scanErr == nil && f > 0 {
			durSec = uint32(f)
		}
	}

	opusBytes, err := os.ReadFile(outPath)
	if err != nil {
		return nil, 0, err
	}

	return opusBytes, durSec, nil
}

func parseDurationSec(durStr string) uint32 {
	parts := strings.Split(strings.TrimSpace(durStr), ":")
	if len(parts) == 2 {
		var m, s uint32
		fmt.Sscanf(parts[0], "%d", &m)
		fmt.Sscanf(parts[1], "%d", &s)
		return m*60 + s
	} else if len(parts) == 3 {
		var h, m, s uint32
		fmt.Sscanf(parts[0], "%d", &h)
		fmt.Sscanf(parts[1], "%d", &m)
		fmt.Sscanf(parts[2], "%d", &s)
		return h*3600 + m*60 + s
	}
	return 0
}
