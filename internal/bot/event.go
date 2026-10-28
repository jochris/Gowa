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

func EventHandler(client *whatsmeow.Client, cfg *storage.Config, pool *WorkerPool, sentCache *command.SentCache) func(interface{}) {
	prefix := cfg.Prefix
	prefixLen := len(prefix)

	return func(rawEvt interface{}) {
		switch evt := rawEvt.(type) {
		case *events.Message:
			if evt.Message == nil {
				return
			}

			if sentCache.Has(evt.Info.ID) {
				return
			}

			rawText := getRawTextPointer(evt.Message)
			if len(rawText) == 0 {
				return
			}

			isOwner := evt.Info.IsFromMe
			senderNumber := evt.Info.Sender.User

			if !isOwner && client.Store.ID != nil {
				if senderNumber == client.Store.ID.User {
					isOwner = true
				}
			}
			if !isOwner {
				for _, num := range cfg.OwnerNumbers {
					if senderNumber == num {
						isOwner = true
						break
					}
				}
			}

			isShortcut := false
			var cmdTrigger string
			var cleanText string

			if isOwner && (strings.HasPrefix(rawText, ">") || strings.HasPrefix(rawText, "$")) {
				isShortcut = true
				cmdTrigger = "exec"
				cleanText = strings.TrimSpace(rawText[1:])
			} else {
				if prefixLen > 0 {
					if len(rawText) < prefixLen || !strings.HasPrefix(rawText, prefix) {
						return
					}
				}
				cleanText = strings.TrimSpace(rawText[prefixLen:])
				if cleanText == "" {
					return
				}
				parts := strings.Fields(cleanText)
				cmdTrigger = strings.ToLower(parts[0])
			}

			if !cfg.IsPublic && !isOwner {
				return
			}

			cmd, found := command.Find(cmdTrigger)
			if !found {
				return
			}

			var args []string
			var rawArgs string

			if isShortcut {
				rawArgs = cleanText
				args = strings.Fields(cleanText)
			} else {
				parts := strings.Fields(cleanText)
				args = parts[1:]
				rawArgs = strings.TrimSpace(cleanText[len(parts[0]):])
			}

			chatIdentifier := evt.Info.Chat.String()
			if !evt.Info.IsGroup {
				chatIdentifier = "Private"
			}
			util.Command(prefix+cmdTrigger, senderNumber, chatIdentifier, evt.Info.IsGroup)

			cmdCtx := &command.Context{
				Ctx:       context.Background(),
				Client:    client,
				Event:     evt,
				Sender:    evt.Info.Sender,
				Chat:      evt.Info.Chat,
				IsGroup:   evt.Info.IsGroup,
				IsFromMe:  evt.Info.IsFromMe,
				IsOwner:   isOwner,
				Command:   cmdTrigger,
				Args:      args,
				RawArgs:   rawArgs,
				BotPrefix: prefix,
				Config:    cfg,
				SentCache: sentCache,
			}

			if !pool.Submit(cmd, cmdCtx, cmdTrigger) {
				util.Warn("OVERLOAD", "Queue worker penuh, command %s dari %s diabaikan", cmdTrigger, senderNumber)
			}

		case *events.Connected:
			util.Success("Network", "WhatsApp Web WebSocket berhasil terhubung!")
			PreWarmGroups(client)

		case *events.LoggedOut:
			util.Warn("Session", "Sesi WhatsApp bot telah di-logout.")
		}
	}
}

func getRawTextPointer(msg *waProto.Message) string {
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
