package command

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

var (
	ErrNoQuotedMessage = errors.New("tidak ada pesan yang di-reply")
	ErrNoMediaFound    = errors.New("tidak ditemukan lampiran media pada pesan")
)

func (c *Context) React(emoji string) error {
	senderJID := c.Sender
	if !c.IsGroup {
		senderJID = types.EmptyJID
	}
	return c.ReactTo(c.Event.Info.ID, senderJID, emoji)
}

func (c *Context) ReactTo(targetID types.MessageID, targetSender types.JID, emoji string) error {
	reactionMsg := c.Client.BuildReaction(c.Chat, targetSender, targetID, emoji)
	return c.sendInternal(c.Chat, reactionMsg)
}

func (c *Context) MarkRead() error {
	ctx := c.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	markCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	senderJID := c.Sender
	if !c.IsGroup {
		senderJID = types.EmptyJID
	}

	return c.Client.MarkRead(markCtx, []types.MessageID{c.Event.Info.ID}, c.Event.Info.Timestamp, c.Chat, senderJID)
}

func (c *Context) Revoke(targetID types.MessageID, targetSender types.JID) error {
	revokeMsg := c.Client.BuildRevoke(c.Chat, targetSender, targetID)
	return c.sendInternal(c.Chat, revokeMsg)
}

func (c *Context) DownloadMedia(msg *waProto.Message) ([]byte, error) {
	if msg == nil {
		return nil, ErrNoMediaFound
	}

	ctx := c.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	dlCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	if img := msg.GetImageMessage(); img != nil {
		return c.Client.Download(dlCtx, img)
	}
	if vid := msg.GetVideoMessage(); vid != nil {
		return c.Client.Download(dlCtx, vid)
	}
	if aud := msg.GetAudioMessage(); aud != nil {
		return c.Client.Download(dlCtx, aud)
	}
	if doc := msg.GetDocumentMessage(); doc != nil {
		return c.Client.Download(dlCtx, doc)
	}
	if st := msg.GetStickerMessage(); st != nil {
		return c.Client.Download(dlCtx, st)
	}

	return nil, ErrNoMediaFound
}

func (c *Context) DownloadQuotedMedia() ([]byte, error) {
	quoted := c.GetQuotedMessage()
	if quoted == nil {
		return nil, ErrNoQuotedMessage
	}
	return c.DownloadMedia(quoted)
}

func UnwrapMessage(msg *waProto.Message) *waProto.Message {
	if msg == nil {
		return nil
	}
	if ep := msg.GetEphemeralMessage(); ep != nil && ep.GetMessage() != nil {
		return UnwrapMessage(ep.GetMessage())
	}
	if vo := msg.GetViewOnceMessage(); vo != nil && vo.GetMessage() != nil {
		return UnwrapMessage(vo.GetMessage())
	}
	if vom := msg.GetViewOnceMessageV2(); vom != nil && vom.GetMessage() != nil {
		return UnwrapMessage(vom.GetMessage())
	}
	if doc := msg.GetDocumentWithCaptionMessage(); doc != nil && doc.GetMessage() != nil {
		return UnwrapMessage(doc.GetMessage())
	}
	return msg
}

func (c *Context) GetContextInfo() *waProto.ContextInfo {
	if c.Event == nil || c.Event.Message == nil {
		return nil
	}
	msg := UnwrapMessage(c.Event.Message)
	if msg == nil {
		return nil
	}
	if ext := msg.GetExtendedTextMessage(); ext != nil && ext.GetContextInfo() != nil {
		return ext.GetContextInfo()
	}
	if img := msg.GetImageMessage(); img != nil && img.GetContextInfo() != nil {
		return img.GetContextInfo()
	}
	if vid := msg.GetVideoMessage(); vid != nil && vid.GetContextInfo() != nil {
		return vid.GetContextInfo()
	}
	if doc := msg.GetDocumentMessage(); doc != nil && doc.GetContextInfo() != nil {
		return doc.GetContextInfo()
	}
	if st := msg.GetStickerMessage(); st != nil && st.GetContextInfo() != nil {
		return st.GetContextInfo()
	}
	return nil
}

func (c *Context) GetQuotedMessage() *waProto.Message {
	ci := c.GetContextInfo()
	if ci != nil {
		return UnwrapMessage(ci.GetQuotedMessage())
	}
	return nil
}

func (c *Context) SendImage(data []byte, mimeType, caption string) error {
	if mimeType == "" {
		mimeType = "image/jpeg"
	}

	ctx := c.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	upCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	resp, err := c.Client.Upload(upCtx, data, whatsmeow.MediaImage)
	if err != nil {
		return fmt.Errorf("upload gambar gagal: %w", err)
	}

	msg := &waProto.Message{
		ImageMessage: &waProto.ImageMessage{
			Caption:       proto.String(caption),
			Mimetype:      proto.String(mimeType),
			URL:           &resp.URL,
			DirectPath:    &resp.DirectPath,
			MediaKey:      resp.MediaKey,
			FileEncSHA256: resp.FileEncSHA256,
			FileSHA256:    resp.FileSHA256,
			FileLength:    &resp.FileLength,
		},
	}

	return c.sendInternal(c.Chat, msg)
}

func (c *Context) ReplyImage(data []byte, mimeType, caption string, mentions ...string) error {
	if mimeType == "" {
		mimeType = "image/jpeg"
	}

	ctx := c.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	upCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	resp, err := c.Client.Upload(upCtx, data, whatsmeow.MediaImage)
	if err != nil {
		return fmt.Errorf("upload gambar gagal: %w", err)
	}

	var participant *string
	if c.IsGroup {
		p := c.Event.Info.Sender.ToNonAD().String()
		participant = proto.String(p)
	}

	ci := &waProto.ContextInfo{
		StanzaID:      proto.String(c.Event.Info.ID),
		Participant:   participant,
		QuotedMessage: c.Event.Message,
	}
	if len(mentions) > 0 {
		ci.MentionedJID = mentions
	}

	msg := &waProto.Message{
		ImageMessage: &waProto.ImageMessage{
			Caption:       proto.String(caption),
			Mimetype:      proto.String(mimeType),
			URL:           &resp.URL,
			DirectPath:    &resp.DirectPath,
			MediaKey:      resp.MediaKey,
			FileEncSHA256: resp.FileEncSHA256,
			FileSHA256:    resp.FileSHA256,
			FileLength:    &resp.FileLength,
			ContextInfo:   ci,
		},
	}

	return c.sendInternal(c.Chat, msg)
}

func (c *Context) SendAudio(data []byte, mimeType string, isPtt bool) error {
	if mimeType == "" {
		mimeType = "audio/ogg; codecs=opus"
	}

	ctx := c.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	upCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	resp, err := c.Client.Upload(upCtx, data, whatsmeow.MediaAudio)
	if err != nil {
		return fmt.Errorf("upload audio gagal: %w", err)
	}

	msg := &waProto.Message{
		AudioMessage: &waProto.AudioMessage{
			Mimetype:      proto.String(mimeType),
			PTT:           proto.Bool(isPtt),
			URL:           &resp.URL,
			DirectPath:    &resp.DirectPath,
			MediaKey:      resp.MediaKey,
			FileEncSHA256: resp.FileEncSHA256,
			FileSHA256:    resp.FileSHA256,
			FileLength:    &resp.FileLength,
		},
	}

	return c.sendInternal(c.Chat, msg)
}

func (c *Context) ReplyAudio(data []byte, mimeType string, isPtt bool) error {
	if mimeType == "" {
		mimeType = "audio/ogg; codecs=opus"
	}

	ctx := c.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	upCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	resp, err := c.Client.Upload(upCtx, data, whatsmeow.MediaAudio)
	if err != nil {
		return fmt.Errorf("upload audio gagal: %w", err)
	}

	var participant *string
	if c.IsGroup {
		p := c.Event.Info.Sender.ToNonAD().String()
		participant = proto.String(p)
	}

	msg := &waProto.Message{
		AudioMessage: &waProto.AudioMessage{
			Mimetype:      proto.String(mimeType),
			PTT:           proto.Bool(isPtt),
			URL:           &resp.URL,
			DirectPath:    &resp.DirectPath,
			MediaKey:      resp.MediaKey,
			FileEncSHA256: resp.FileEncSHA256,
			FileSHA256:    resp.FileSHA256,
			FileLength:    &resp.FileLength,
			ContextInfo: &waProto.ContextInfo{
				StanzaID:      proto.String(c.Event.Info.ID),
				Participant:   participant,
				QuotedMessage: c.Event.Message,
			},
		},
	}

	return c.sendInternal(c.Chat, msg)
}

func (c *Context) SendVideo(data []byte, mimeType, caption string) error {
	if mimeType == "" {
		mimeType = "video/mp4"
	}

	ctx := c.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	upCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	resp, err := c.Client.Upload(upCtx, data, whatsmeow.MediaVideo)
	if err != nil {
		return fmt.Errorf("upload video gagal: %w", err)
	}

	msg := &waProto.Message{
		VideoMessage: &waProto.VideoMessage{
			Caption:       proto.String(caption),
			Mimetype:      proto.String(mimeType),
			URL:           &resp.URL,
			DirectPath:    &resp.DirectPath,
			MediaKey:      resp.MediaKey,
			FileEncSHA256: resp.FileEncSHA256,
			FileSHA256:    resp.FileSHA256,
			FileLength:    &resp.FileLength,
		},
	}

	return c.sendInternal(c.Chat, msg)
}

func (c *Context) ReplyVideo(data []byte, mimeType, caption string) error {
	if mimeType == "" {
		mimeType = "video/mp4"
	}

	ctx := c.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	upCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	resp, err := c.Client.Upload(upCtx, data, whatsmeow.MediaVideo)
	if err != nil {
		return fmt.Errorf("upload video gagal: %w", err)
	}

	var participant *string
	if c.IsGroup {
		p := c.Event.Info.Sender.ToNonAD().String()
		participant = proto.String(p)
	}

	msg := &waProto.Message{
		VideoMessage: &waProto.VideoMessage{
			Caption:       proto.String(caption),
			Mimetype:      proto.String(mimeType),
			URL:           &resp.URL,
			DirectPath:    &resp.DirectPath,
			MediaKey:      resp.MediaKey,
			FileEncSHA256: resp.FileEncSHA256,
			FileSHA256:    resp.FileSHA256,
			FileLength:    &resp.FileLength,
			ContextInfo: &waProto.ContextInfo{
				StanzaID:      proto.String(c.Event.Info.ID),
				Participant:   participant,
				QuotedMessage: c.Event.Message,
			},
		},
	}

	return c.sendInternal(c.Chat, msg)
}

func (c *Context) SendDocument(data []byte, fileName, mimeType, caption string) error {
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	ctx := c.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	upCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	resp, err := c.Client.Upload(upCtx, data, whatsmeow.MediaDocument)
	if err != nil {
		return fmt.Errorf("upload dokumen gagal: %w", err)
	}

	msg := &waProto.Message{
		DocumentMessage: &waProto.DocumentMessage{
			Title:         proto.String(fileName),
			FileName:      proto.String(fileName),
			Caption:       proto.String(caption),
			Mimetype:      proto.String(mimeType),
			URL:           &resp.URL,
			DirectPath:    &resp.DirectPath,
			MediaKey:      resp.MediaKey,
			FileEncSHA256: resp.FileEncSHA256,
			FileSHA256:    resp.FileSHA256,
			FileLength:    &resp.FileLength,
		},
	}

	return c.sendInternal(c.Chat, msg)
}

func (c *Context) ReplyDocument(data []byte, fileName, mimeType, caption string) error {
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	ctx := c.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	upCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	resp, err := c.Client.Upload(upCtx, data, whatsmeow.MediaDocument)
	if err != nil {
		return fmt.Errorf("upload dokumen gagal: %w", err)
	}

	var participant *string
	if c.IsGroup {
		p := c.Event.Info.Sender.ToNonAD().String()
		participant = proto.String(p)
	}

	msg := &waProto.Message{
		DocumentMessage: &waProto.DocumentMessage{
			Title:         proto.String(fileName),
			FileName:      proto.String(fileName),
			Caption:       proto.String(caption),
			Mimetype:      proto.String(mimeType),
			URL:           &resp.URL,
			DirectPath:    &resp.DirectPath,
			MediaKey:      resp.MediaKey,
			FileEncSHA256: resp.FileEncSHA256,
			FileSHA256:    resp.FileSHA256,
			FileLength:    &resp.FileLength,
			ContextInfo: &waProto.ContextInfo{
				StanzaID:      proto.String(c.Event.Info.ID),
				Participant:   participant,
				QuotedMessage: c.Event.Message,
			},
		},
	}

	return c.sendInternal(c.Chat, msg)
}
