package bot

import (
	"context"
	"strings"
	"time"

	"gowa/internal/command"
	"gowa/internal/storage"
	"gowa/internal/util"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

var botStartTime = time.Now()

func EventHandler(client *whatsmeow.Client, cfg *storage.Config, pool *WorkerPool, sentCache *command.SentCache) func(any) {
	return func(rawEvt any) {
		switch evt := rawEvt.(type) {
		case *events.Message:
			handleIncomingMessage(evt, client, cfg, pool, sentCache)

		case *events.CallOffer:
			handleCallOffer(evt, client)

		case *events.GroupInfo:
			handleGroupInfo(evt)

		case *events.OfflineSyncPreview:
			if evt.Messages > 0 {
				util.Info("Sync", "Melewati %d pesan offline/lama dari server WhatsApp...", evt.Messages)
			}

		case *events.OfflineSyncCompleted:
			util.Success("Sync", "Sinkronisasi offline selesai (%d total event). Bot siap memproses pesan realtime.", evt.Count)

		case *events.Connected:
			util.Success("Network", "WhatsApp Web WebSocket berhasil terhubung!")
			PreWarmGroups(client)

		case *events.LoggedOut:
			util.Warn("Session", "Sesi WhatsApp bot telah di-logout.")
		}
	}
}

func handleCallOffer(evt *events.CallOffer, client *whatsmeow.Client) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_ = client.RejectCall(ctx, evt.CallCreator, evt.CallID)
		util.Warn("Call", "Panggilan masuk dari %s otomatis ditolak", evt.CallCreator.User)

		rejectMsg := &waProto.Message{
			Conversation: proto.String("*Pemberitahuan Otomatis:*\nNomor ini adalah bot WhatsApp dan tidak dapat menerima panggilan telepon atau video."),
		}
		_, _ = client.SendMessage(ctx, evt.CallCreator.ToNonAD(), rejectMsg)
	}()
}

func handleGroupInfo(evt *events.GroupInfo) {
	if len(evt.Join) > 0 {
		util.Info("Group", "%d anggota baru bergabung ke grup %s", len(evt.Join), evt.JID.User)
	}
	if len(evt.Leave) > 0 {
		util.Info("Group", "%d anggota keluar dari grup %s", len(evt.Leave), evt.JID.User)
	}
	if len(evt.Promote) > 0 {
		util.Info("Group", "%d anggota dipromosikan jadi admin di grup %s", len(evt.Promote), evt.JID.User)
	}
	if len(evt.Demote) > 0 {
		util.Info("Group", "%d admin diturunkan jadi anggota di grup %s", len(evt.Demote), evt.JID.User)
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

	if cfg.IgnoreOffline && !evt.Info.Timestamp.IsZero() {
		msgAge := time.Since(evt.Info.Timestamp)
		if msgAge > 2*time.Minute || evt.Info.Timestamp.Before(botStartTime.Add(-30*time.Second)) {
			return
		}
	}

	rawText := extractMessageText(evt.Message)
	if len(rawText) == 0 {
		return
	}

	ctx := context.Background()
	senderPN, senderNumber, isOwner := resolveSenderInfo(ctx, evt, client, cfg)

	trigger, args, rawArgs, isShortcut, ok := parseCommandInput(rawText, cfg.Prefix, isOwner)
	if !ok {
		return
	}

	if !cfg.IsPublic && !isOwner {
		util.Warn("Mode", "Perintah .%s dari %s diabaikan (Mode bot saat ini: Self)", trigger, senderNumber)
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
		Ctx:       ctx,
		Client:    client,
		Event:     evt,
		Sender:    evt.Info.Sender,
		SenderPN:  senderPN,
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

	_ = cmdCtx.MarkRead()

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

func resolveSenderInfo(ctx context.Context, evt *events.Message, client *whatsmeow.Client, cfg *storage.Config) (types.JID, string, bool) {
	senderJID := evt.Info.Sender
	senderAlt := evt.Info.SenderAlt

	if !senderAlt.IsEmpty() {
		if senderJID.Server == types.HiddenUserServer && senderAlt.Server == types.DefaultUserServer {
			client.StoreLIDPNMapping(ctx, senderJID, senderAlt)
		} else if senderJID.Server == types.DefaultUserServer && senderAlt.Server == types.HiddenUserServer {
			client.StoreLIDPNMapping(ctx, senderAlt, senderJID)
		}
	}

	pnJID := senderJID
	if senderJID.Server == types.HiddenUserServer {
		if senderAlt.Server == types.DefaultUserServer {
			pnJID = senderAlt
		} else if client.Store != nil {
			if alt, err := client.Store.GetAltJID(ctx, senderJID); err == nil && !alt.IsEmpty() && alt.Server == types.DefaultUserServer {
				pnJID = alt
			}
		}
	}

	senderNumber := cleanPhoneNumber(pnJID.User)
	rawSenderUser := cleanPhoneNumber(senderJID.User)
	isOwner := evt.Info.IsFromMe

	if !isOwner && client.Store != nil {
		if client.Store.ID != nil {
			botUser := cleanPhoneNumber(client.Store.ID.User)
			if senderNumber == botUser || rawSenderUser == botUser {
				isOwner = true
			}
		}
		if !client.Store.LID.IsEmpty() {
			botLID := cleanPhoneNumber(client.Store.LID.User)
			if rawSenderUser == botLID || senderNumber == botLID {
				isOwner = true
			}
		}
	}

	if !isOwner {
		for _, num := range cfg.OwnerNumbers {
			cleanOwner := cleanPhoneNumber(num)
			if senderNumber == cleanOwner || rawSenderUser == cleanOwner {
				isOwner = true
				break
			}
		}
	}

	return pnJID, senderNumber, isOwner
}

func parseCommandInput(rawText, prefix string, isOwner bool) (trigger string, args []string, rawArgs string, isShortcut bool, ok bool) {
	trimmed := strings.TrimSpace(rawText)
	if trimmed == "" {
		return "", nil, "", false, false
	}

	if isOwner && strings.HasPrefix(trimmed, "$") {
		cleanText := strings.TrimSpace(trimmed[1:])
		fields := strings.Fields(cleanText)
		return "$", fields, cleanText, true, true
	}

	prefixLen := len(prefix)
	hasPrefix := false
	cleanText := trimmed

	if prefixLen > 0 && strings.HasPrefix(trimmed, prefix) {
		hasPrefix = true
		cleanText = strings.TrimSpace(trimmed[prefixLen:])
	}

	if cleanText == "" {
		return "", nil, "", false, false
	}

	parts := strings.Fields(cleanText)
	if len(parts) == 0 {
		return "", nil, "", false, false
	}

	potentialTrigger := strings.ToLower(parts[0])

	if hasPrefix {
		return potentialTrigger, parts[1:], strings.TrimSpace(cleanText[len(parts[0]):]), false, true
	}

	if isOwner {
		if _, exists := command.Find(potentialTrigger); exists {
			return potentialTrigger, parts[1:], strings.TrimSpace(cleanText[len(parts[0]):]), true, true
		}
	}

	return "", nil, "", false, false
}
