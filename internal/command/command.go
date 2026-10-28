package command

import (
	"context"
	"strings"

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

	target := c.Chat.ToNonAD()
	resp, err := c.Client.SendMessage(c.Ctx, target, msg)
	if err == nil && c.SentCache != nil && resp.ID != "" {
		c.SentCache.Add(string(resp.ID))
	}
	return err
}

func (c *Context) Send(text string) error {
	msg := &waProto.Message{
		Conversation: proto.String(text),
	}
	target := c.Chat.ToNonAD()
	resp, err := c.Client.SendMessage(c.Ctx, target, msg)
	if err == nil && c.SentCache != nil && resp.ID != "" {
		c.SentCache.Add(string(resp.ID))
	}
	return err
}

func (c *Context) SendTo(jid types.JID, text string) error {
	msg := &waProto.Message{
		Conversation: proto.String(text),
	}
	target := jid.ToNonAD()
	resp, err := c.Client.SendMessage(c.Ctx, target, msg)
	if err == nil && c.SentCache != nil && resp.ID != "" {
		c.SentCache.Add(string(resp.ID))
	}
	return err
}

func (c *Context) Typing() {
	go func() {
		_ = c.Client.SendChatPresence(c.Ctx, c.Chat.ToNonAD(), types.ChatPresenceComposing, types.ChatPresenceMediaText)
	}()
}

func HasPrefix(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}
