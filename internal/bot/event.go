package bot

import (
	"context"
	"strings"

	"gowa/internal/command"
	"gowa/internal/storage"
	"gowa/internal/util"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types/events"
)

func EventHandler(client *whatsmeow.Client, cfg *storage.Config, pool *WorkerPool, sentCache *command.SentCache) func(any) {
	return func(rawEvt any) {
		switch evt := rawEvt.(type) {
		case *events.Message:
			handleIncomingMessage(evt, client, cfg, pool, sentCache)

		case *events.Connected:
			util.Success("Network", "WhatsApp Web WebSocket berhasil terhubung!")
			PreWarmGroups(client)

		case *events.LoggedOut:
			util.Warn("Session", "Sesi WhatsApp bot telah di-logout.")
		}
	}
}

func handleIncomingMessage(
	evt *events.Message,
	client *whatsmeow.Client,
	cfg *storage.Config,
	pool *WorkerPool,
	sentCache *command.SentCache,
) {
	if evt.Message == nil || sentCache.Has(evt.Info.ID) {
		return
	}

	rawText := extractMessageText(evt.Message)
	if len(rawText) == 0 {
		return
	}

	senderNumber, isOwner := resolveSenderInfo(evt, client, cfg)

	trigger, args, rawArgs, isShortcut, ok := parseCommandInput(rawText, cfg.Prefix, isOwner)
	if !ok {
		return
	}

	if !cfg.IsPublic && !isOwner {
		return
	}

	cmd, found := command.Find(trigger)
	if !found {
		return
	}

	chatIdentifier := evt.Info.Chat.String()
	if !evt.Info.IsGroup {
		chatIdentifier = "Private"
	}

	logPrefix := cfg.Prefix
	if isShortcut {
		logPrefix = ""
	}
	util.Command(logPrefix+trigger, senderNumber, chatIdentifier, evt.Info.IsGroup)

	cmdCtx := &command.Context{
		Ctx:       context.Background(),
		Client:    client,
		Event:     evt,
		Sender:    evt.Info.Sender,
		Chat:      evt.Info.Chat,
		IsGroup:   evt.Info.IsGroup,
		IsFromMe:  evt.Info.IsFromMe,
		IsOwner:   isOwner,
		Command:   trigger,
		Args:      args,
		RawArgs:   rawArgs,
		BotPrefix: cfg.Prefix,
		Config:    cfg,
		SentCache: sentCache,
	}

	if !pool.Submit(cmd, cmdCtx, trigger) {
		util.Warn("OVERLOAD", "Queue worker penuh, perintah %s dari %s diabaikan", trigger, senderNumber)
	}
}

func extractMessageText(msg *waProto.Message) string {
	if msg == nil {
		return ""
	}
	if msg.Conversation != nil {
		return *msg.Conversation
	}
	if msg.ExtendedTextMessage != nil && msg.ExtendedTextMessage.Text != nil {
		return *msg.ExtendedTextMessage.Text
	}
	if msg.ImageMessage != nil && msg.ImageMessage.Caption != nil {
		return *msg.ImageMessage.Caption
	}
	if msg.VideoMessage != nil && msg.VideoMessage.Caption != nil {
		return *msg.VideoMessage.Caption
	}
	if msg.DocumentMessage != nil && msg.DocumentMessage.Caption != nil {
		return *msg.DocumentMessage.Caption
	}
	return ""
}

func resolveSenderInfo(evt *events.Message, client *whatsmeow.Client, cfg *storage.Config) (string, bool) {
	isOwner := evt.Info.IsFromMe
	senderNumber := evt.Info.Sender.User

	if !isOwner && client.Store.ID != nil && senderNumber == client.Store.ID.User {
		isOwner = true
	}

	if !isOwner {
		for _, num := range cfg.OwnerNumbers {
			if senderNumber == num {
				isOwner = true
				break
			}
		}
	}

	return senderNumber, isOwner
}

func parseCommandInput(rawText, prefix string, isOwner bool) (trigger string, args []string, rawArgs string, isShortcut bool, ok bool) {
	if isOwner && (strings.HasPrefix(rawText, ">") || strings.HasPrefix(rawText, "$")) {
		cleanText := strings.TrimSpace(rawText[1:])
		fields := strings.Fields(cleanText)
		return "exec", fields, cleanText, true, true
	}

	prefixLen := len(prefix)
	if prefixLen > 0 {
		if len(rawText) < prefixLen || !strings.HasPrefix(rawText, prefix) {
			return "", nil, "", false, false
		}
	}

	cleanText := strings.TrimSpace(rawText[prefixLen:])
	if cleanText == "" {
		return "", nil, "", false, false
	}

	parts := strings.Fields(cleanText)
	trigger = strings.ToLower(parts[0])
	args = parts[1:]
	rawArgs = strings.TrimSpace(cleanText[len(parts[0]):])

	return trigger, args, rawArgs, false, true
}
