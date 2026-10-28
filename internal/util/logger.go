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

type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

var (
	mu           sync.Mutex
	currentLevel = LevelInfo
)

func SetLogLevel(level string) {
	mu.Lock()
	defer mu.Unlock()
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "DEBUG":
		currentLevel = LevelDebug
	case "INFO":
		currentLevel = LevelInfo
	case "WARN", "WARNING":
		currentLevel = LevelWarn
	case "ERROR":
		currentLevel = LevelError
	default:
		currentLevel = LevelInfo
	}
}

func formatTime() string {
	return time.Now().Format("15:04:05")
}

func Debug(tag, format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()
	if currentLevel > LevelDebug {
		return
	}
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s[%s]%s %s🔍 [%s]%s %s\n", colorGray, formatTime(), colorReset, colorBlue, tag, colorReset, msg)
}

func Info(tag, format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()
	if currentLevel > LevelInfo {
		return
	}
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s[%s]%s %sℹ️  [%s]%s %s\n", colorGray, formatTime(), colorReset, colorCyan, tag, colorReset, msg)
}

func Success(tag, format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()
	if currentLevel > LevelInfo {
		return
	}
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s[%s]%s %s✅ [%s]%s %s\n", colorGray, formatTime(), colorReset, colorGreen, tag, colorReset, msg)
}

func Warn(tag, format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()
	if currentLevel > LevelWarn {
		return
	}
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s[%s]%s %s⚠️  [%s]%s %s\n", colorGray, formatTime(), colorReset, colorYellow, tag, colorReset, msg)
}

func Error(tag, format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()
	if currentLevel > LevelError {
		return
	}
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

func (l *CleanLogger) Debugf(msg string, args ...any) {
	if currentLevel <= LevelDebug {
		Debug(l.module, msg, args...)
	}
}

func (l *CleanLogger) Infof(msg string, args ...any) {
	formatted := fmt.Sprintf(msg, args...)
	if strings.Contains(formatted, "Successfully authenticated") {
		Success("Auth", "WhatsApp Web berhasil terautentikasi.")
	} else if strings.Contains(formatted, "Successfully paired") {
		Success("Auth", "Perangkat berhasil ditautkan.")
	} else if strings.Contains(formatted, "Connected") {
		Success("Network", "Terhubung ke server WhatsApp WebSocket.")
	} else if currentLevel <= LevelInfo {
		Info(l.module, "%s", formatted)
	}
}

func (l *CleanLogger) Warnf(msg string, args ...any) {
	formatted := fmt.Sprintf(msg, args...)
	if shouldFilterMessage(formatted) {
		return
	}
	Warn(l.module, "%s", formatted)
}

func (l *CleanLogger) Errorf(msg string, args ...any) {
	formatted := fmt.Sprintf(msg, args...)
	if strings.Contains(formatted, "failed to decrypt") || strings.Contains(formatted, "no sender key") {
		return
	}
	Error(l.module, "%s", formatted)
}

func shouldFilterMessage(msg string) bool {
	ignored := []string{
		"Unavailable message",
		"failed to decrypt",
		"no sender key",
		"duplicate contacts found",
		"Missing response in item",
		"Failed to store LID mappings",
		"Failed to store redacted phones",
		"Error decrypting message",
		"Got 515 code",
		"rate-overlimit",
	}
	for _, term := range ignored {
		if strings.Contains(msg, term) {
			return true
		}
	}
	return false
}
