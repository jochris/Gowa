package media

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gowa/internal/command"
)

func init() {
	command.Register(&command.Command{
		Name:    "play",
		Cmd:     "play",
		Categ:   "media",
		Desc:    "Mencari dan memutar lagu / audio YouTube",
		Alias:   []string{"song", "ytplay"},
		Execute: handlePlay,
	})
}

func handlePlay(ctx *command.Context) error {
	ctx.Typing()

	if strings.TrimSpace(ctx.RawArgs) == "" {
		return ctx.Reply(fmt.Sprintf(
			"❌ Masukkan judul lagu atau link YouTube yang ingin diputar.\n\n"+
				"*Cara Penggunaan:*\n"+
				"- `%splay <judul lagu>`\n"+
				"- `%splay <link YouTube>`\n"+
				"- Contoh: `%splay faded alan walker`",
			ctx.BotPrefix, ctx.BotPrefix, ctx.BotPrefix,
		))
	}

	c := ctx.Ctx
	if c == nil {
		c = context.Background()
	}

	engine, err := NewYTMusicEngine(30 * time.Second)
	if err != nil {
		return ctx.Reply("❌ Gagal menginisialisasi engine YouTube: " + err.Error())
	}

	query := strings.TrimSpace(ctx.RawArgs)
	_ = ctx.React("🔍")

	var (
		targetVideoID string
		trackTitle    string
		trackArtist   string
		trackDuration string
	)

	videoID := engine.extractVideoID(query)
	if videoID != "" {
		targetVideoID = videoID
		infoCtx, infoCancel := context.WithTimeout(c, 25*time.Second)
		track, err := engine.GetTrackInfo(infoCtx, videoID)
		infoCancel()

		if err == nil && track != nil {
			trackTitle = track.Title
			trackArtist = track.Artist
			trackDuration = track.Duration
		} else {
			trackTitle = "YouTube Audio Track"
			trackDuration = "-"
		}
	} else {
		searchCtx, searchCancel := context.WithTimeout(c, 30*time.Second)
		res, err := engine.Search(searchCtx, query)
		searchCancel()

		if err != nil {
			_ = ctx.React("❌")
			return ctx.Reply("❌ Gagal mencari lagu di YouTube Music: " + err.Error())
		}

		if len(res.Songs) == 0 {
			_ = ctx.React("❌")
			return ctx.Reply(fmt.Sprintf("❌ Lagu tidak ditemukan untuk pencarian: *%s*", query))
		}

		topSong := res.Songs[0]
		targetVideoID = topSong.VideoID
		trackTitle = topSong.Title
		trackArtist = topSong.Artist
		trackDuration = topSong.Duration
	}

	if trackTitle == "" {
		trackTitle = "YouTube Audio Track"
	}
	if trackArtist == "" {
		trackArtist = "Unknown Artist"
	}
	if trackDuration == "" {
		trackDuration = "-"
	}

	infoMsg := fmt.Sprintf(
		"🎵 *YouTube Play*\n\n"+
			"- *Judul:* %s\n"+
			"- *Artis:* %s\n"+
			"- *Durasi:* %s\n"+
			"- *Tautan:* https://youtu.be/%s\n\n"+
			"_Sedang mengunduh audio..._",
		trackTitle, trackArtist, trackDuration, targetVideoID,
	)
	_ = ctx.Reply(infoMsg)
	_ = ctx.React("⏳")

	dlCtx, dlCancel := context.WithTimeout(c, 90*time.Second)
	audioBytes, _, err := engine.DownloadAudio(dlCtx, targetVideoID)
	dlCancel()

	if err != nil {
		_ = ctx.React("❌")
		return ctx.Reply("❌ Gagal mengunduh audio: " + err.Error())
	}

	convCtx, convCancel := context.WithTimeout(c, 60*time.Second)
	opusBytes, durSec, err := convertAudioToOpus(convCtx, audioBytes)
	convCancel()

	if err != nil {
		_ = ctx.React("❌")
		return ctx.Reply("❌ Gagal memproses format audio WhatsApp: " + err.Error())
	}

	if durSec == 0 && trackDuration != "-" {
		durSec = parseDurationSec(trackDuration)
	}

	err = ctx.ReplyAudio(opusBytes, "audio/ogg; codecs=opus", false, durSec)
	if err != nil {
		_ = ctx.React("❌")
		return ctx.Reply("❌ Gagal mengirim audio ke WhatsApp: " + err.Error())
	}

	_ = ctx.React("🎵")
	return nil
}
