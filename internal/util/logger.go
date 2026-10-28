package util

import (
	"fmt"
	"strings"
	"sync"
	"time"

	waLog "go.mau.fi/whatsmeow/util/log"
)

const (
	colorReset   = "\033[0m"
	colorRed     = "\033[31m"
	colorGreen   = "\033[32m"
	colorYellow  = "\033[33m"
	colorBlue    = "\033[34m"
	colorMagenta = "\033[35m"
	colorCyan    = "\033[36m"
	colorGray    = "\033[90m"
	colorBold    = "\033[1m"
)

var mu sync.Mutex

func formatTime() string {
	return time.Now().Format("15:04:05")
}

func Info(tag, format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s[%s]%s %sℹ️  [%s]%s %s\n", colorGray, formatTime(), colorReset, colorCyan, tag, colorReset, msg)
}

func Success(tag, format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s[%s]%s %s✅ [%s]%s %s\n", colorGray, formatTime(), colorReset, colorGreen, tag, colorReset, msg)
}

func Warn(tag, format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s[%s]%s %s⚠️  [%s]%s %s\n", colorGray, formatTime(), colorReset, colorYellow, tag, colorReset, msg)
}

func Error(tag, format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s[%s]%s %s❌ [%s]%s %s\n", colorGray, formatTime(), colorReset, colorRed, tag, colorReset, msg)
}

func Command(cmd, sender, chat string, isGroup bool) {
	mu.Lock()
	defer mu.Unlock()
	chatType := "Private"
	if isGroup {
		chatType = "Group"
	}
	fmt.Printf("%s[%s]%s %s💬 [CMD]%s %s%s%s %s|%s Dari: %s%s%s %s|%s Chat: %s%s (%s)%s\n",
		colorGray, formatTime(), colorReset,
		colorMagenta+colorBold, colorReset,
		colorCyan+colorBold, cmd, colorReset,
		colorGray, colorReset,
		colorYellow, sender, colorReset,
		colorGray, colorReset,
		colorGreen, chat, chatType, colorReset,
	)
}

type CleanLogger struct {
	module string
}

func NewLogger(module string) waLog.Logger {
	return &CleanLogger{module: module}
}

func (l *CleanLogger) Sub(module string) waLog.Logger {
	return &CleanLogger{module: l.module + "/" + module}
}

func (l *CleanLogger) Debugf(msg string, args ...any) {}

func (l *CleanLogger) Infof(msg string, args ...any) {
	formatted := fmt.Sprintf(msg, args...)
	if strings.Contains(formatted, "Successfully authenticated") {
		Success("Auth", "WhatsApp Web berhasil terautentikasi.")
	} else if strings.Contains(formatted, "Successfully paired") {
		Success("Auth", "Perangkat berhasil ditautkan.")
	} else if strings.Contains(formatted, "Connected") {
		Success("Network", "Terhubung ke server WhatsApp WebSocket.")
	}
}

func (l *CleanLogger) Warnf(msg string, args ...any) {
	formatted := fmt.Sprintf(msg, args...)
	if strings.Contains(formatted, "Unavailable message") ||
		strings.Contains(formatted, "failed to decrypt") ||
		strings.Contains(formatted, "no sender key") ||
		strings.Contains(formatted, "duplicate contacts found") ||
		strings.Contains(formatted, "Missing response in item") ||
		strings.Contains(formatted, "Failed to store LID mappings") ||
		strings.Contains(formatted, "Failed to store redacted phones") ||
		strings.Contains(formatted, "Error decrypting message") ||
		strings.Contains(formatted, "Got 515 code") ||
		strings.Contains(formatted, "rate-overlimit") {
		return
	}
	Warn(l.module, formatted)
}

func (l *CleanLogger) Errorf(msg string, args ...any) {
	formatted := fmt.Sprintf(msg, args...)
	if strings.Contains(formatted, "failed to decrypt") ||
		strings.Contains(formatted, "no sender key") {
		return
	}
	Error(l.module, formatted)
}
