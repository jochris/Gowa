package command

import (
	"context"
	"time"

	"gowa/internal/storage"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

type Command struct {
	Name    string
	Cmd     string
	Categ   string
	Desc    string
	Alias   []string
	Execute func(ctx *Context) error
}

type Context struct {
	Ctx       context.Context
	Client    *whatsmeow.Client
	Event     *events.Message
	Sender    types.JID
	Chat      types.JID
	IsGroup   bool
	IsFromMe  bool
	IsOwner   bool
	Command   string
	Args      []string
	RawArgs   string
	BotPrefix string
	Config    *storage.Config
	SentCache *SentCache
}

func (c *Context) sendInternal(target types.JID, msg *waProto.Message) error {
	ctx := c.Ctx
	if ctx == nil {
		ctx = context.Background()
	}

	sendCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	resp, err := c.Client.SendMessage(sendCtx, target.ToNonAD(), msg)
	if err == nil && c.SentCache != nil && resp.ID != "" {
		c.SentCache.Add(string(resp.ID))
	}
	return err
}

func (c *Context) Reply(text string) error {
	var participant *string
	if c.IsGroup {
		p := c.Event.Info.Sender.ToNonAD().String()
		participant = proto.String(p)
	}

	msg := &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: proto.String(text),
			ContextInfo: &waProto.ContextInfo{
				StanzaID:      proto.String(c.Event.Info.ID),
				Participant:   participant,
				QuotedMessage: c.Event.Message,
			},
		},
	}

	return c.sendInternal(c.Chat, msg)
}

func (c *Context) Send(text string) error {
	msg := &waProto.Message{
		Conversation: proto.String(text),
	}
	return c.sendInternal(c.Chat, msg)
}

func (c *Context) SendTo(jid types.JID, text string) error {
	msg := &waProto.Message{
		Conversation: proto.String(text),
	}
	return c.sendInternal(jid, msg)
}

func (c *Context) Typing() {
	ctx := c.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	go func() {
		typeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_ = c.Client.SendChatPresence(typeCtx, c.Chat.ToNonAD(), types.ChatPresenceComposing, types.ChatPresenceMediaText)
	}()
}
