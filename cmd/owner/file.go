package owner

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gowa/internal/command"
)

const maxFileOutput = 32 * 1024

func init() {
	command.Register(&command.Command{
		Name:    "file",
		Cmd:     "file",
		Categ:   "owner",
		Desc:    "Kelola file: create, read, delete (Khusus Owner)",
		Execute: handleFile,
	})
}

const fileUsage = "*Usage:*\n" +
	"`file create [-f] <path> [isi]`\n" +
	"`file read <path>`\n" +
	"`file delete [-r] <path>`\n\n" +
	"_Path boleh dikutip_ `\"...\"` atau menggunakan `~/`.\n" +
	"_Path berakhiran_ `/` _pada create membuat direktori._"

func handleFile(ctx *command.Context) error {
	if !ctx.IsOwner {
		return nil
	}

	sub, rest := splitHead(ctx.RawArgs)
	if sub == "" {
		return ctx.Reply(fileUsage)
	}

	ctx.Typing()

	switch strings.ToLower(sub) {
	case "create", "new", "touch":
		return fileCreate(ctx, rest)
	case "read", "cat":
		return fileRead(ctx, rest)
	case "delete", "del", "rm":
		return fileDelete(ctx, rest)
	default:
		return ctx.Reply(fileUsage)
	}
}

func fileCreate(ctx *command.Context, rest string) error {
	flags, path, content := parseFileArgs(rest)
	if path == "" {
		return ctx.Reply("*Usage:* `file create [-f] <path> [isi]`")
	}

	abs, err := resolveFilePath(path)
	if err != nil {
		return ctx.Reply(fmt.Sprintf("*Error:* %v", err))
	}

	if strings.HasSuffix(path, "/") {
		if err := os.MkdirAll(abs, 0755); err != nil {
			return ctx.Reply(fmt.Sprintf("*Error:* %v", err))
		}
		return ctx.Reply(fmt.Sprintf("_Direktori dibuat:_ `%s`", abs))
	}

	if info, err := os.Stat(abs); err == nil {
		if info.IsDir() {
			return ctx.Reply(fmt.Sprintf("*Error:* `%s` adalah direktori.", abs))
		}
		if !flags["f"] {
			return ctx.Reply(fmt.Sprintf("*Error:* `%s` sudah ada. Gunakan `-f`.", abs))
		}
	} else if !os.IsNotExist(err) {
		return ctx.Reply(fmt.Sprintf("*Error:* %v", err))
	}

	if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
		return ctx.Reply(fmt.Sprintf("*Error:* %v", err))
	}

	data := []byte(stripFileFence(content))
	if err := os.WriteFile(abs, data, 0644); err != nil {
		return ctx.Reply(fmt.Sprintf("*Error:* %v", err))
	}

	return ctx.Reply(fmt.Sprintf(
		"_File dibuat:_ `%s` (%s)",
		abs,
		fileSize(int64(len(data))),
	))
}

func fileRead(ctx *command.Context, rest string) error {
	_, path, _ := parseFileArgs(rest)
	if path == "" {
		return ctx.Reply("*Usage:* `file read <path>`")
	}

	abs, err := resolveFilePath(path)
	if err != nil {
		return ctx.Reply(fmt.Sprintf("*Error:* %v", err))
	}

	info, err := os.Stat(abs)
	if err != nil {
		return ctx.Reply(fmt.Sprintf("*Error:* %v", err))
	}

	if info.IsDir() {
		return ctx.Reply(readDirectory(abs))
	}

	f, err := os.Open(abs)
	if err != nil {
		return ctx.Reply(fmt.Sprintf("*Error:* %v", err))
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, maxFileOutput+1))
	if err != nil {
		return ctx.Reply(fmt.Sprintf("*Error:* %v", err))
	}

	header := fmt.Sprintf("`%s` · %s", filepath.Base(abs), fileSize(info.Size()))

	switch {
	case bytes.IndexByte(data, 0) >= 0:
		return ctx.Reply(header + "\n_File biner, tidak ditampilkan._")
	case len(data) == 0:
		return ctx.Reply(header + "\n_File kosong._")
	case len(data) > maxFileOutput:
		data = append(data[:maxFileOutput], []byte("\n... (output dipotong)")...)
	}

	return ctx.Reply(formatFileOutput(header, string(data)))
}

func fileDelete(ctx *command.Context, rest string) error {
	flags, path, _ := parseFileArgs(rest)
	if path == "" {
		return ctx.Reply("*Usage:* `file delete [-r] <path>`")
	}

	abs, err := resolveFilePath(path)
	if err != nil {
		return ctx.Reply(fmt.Sprintf("*Error:* %v", err))
	}

	if protectedFilePath(abs) {
		return ctx.Reply(fmt.Sprintf("*Ditolak:* `%s` adalah root, working directory, atau home.", abs))
	}

	info, err := os.Lstat(abs)
	if err != nil {
		return ctx.Reply(fmt.Sprintf("*Error:* %v", err))
	}

	if info.IsDir() {
		if !flags["r"] {
			return ctx.Reply(fmt.Sprintf(
				"*Error:* `%s` adalah direktori. Gunakan `-r`.",
				abs,
			))
		}
		err = os.RemoveAll(abs)
	} else {
		err = os.Remove(abs)
	}

	if err != nil {
		return ctx.Reply(fmt.Sprintf("*Error:* %v", err))
	}

	return ctx.Reply(fmt.Sprintf("_Dihapus:_ `%s`", abs))
}

func readDirectory(path string) string {
	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Sprintf("*Error:* %v", err)
	}

	const limit = 100

	header := fmt.Sprintf("`%s` · %d item", path, len(entries))
	if len(entries) == 0 {
		return header
	}

	var lines []string

	for i, entry := range entries {
		if i >= limit {
			lines = append(lines, fmt.Sprintf("... (+%d lainnya)", len(entries)-limit))
			break
		}

		name := entry.Name()
		if entry.IsDir() {
			lines = append(lines, name+"/")
			continue
		}

		info, err := entry.Info()
		if err == nil {
			name += " (" + fileSize(info.Size()) + ")"
		}

		lines = append(lines, name)
	}

	return formatFileOutput(header, strings.Join(lines, "\n"))
}

func parseFileArgs(rest string) (map[string]bool, string, string) {
	flags := make(map[string]bool)

	for {
		head, next := splitHead(rest)
		if len(head) == 2 && head[0] == '-' &&
			strings.ContainsRune("fr", rune(head[1])) {
			flags[head[1:]] = true
			rest = next
			continue
		}
		break
	}

	rest = strings.TrimLeft(rest, " \t\r\n")

	if strings.HasPrefix(rest, `"`) {
		if end := strings.Index(rest[1:], `"`); end >= 0 {
			return flags, rest[1 : end+1], dropSeparator(rest[end+2:])
		}
	}

	path, tail := splitHead(rest)
	return flags, path, tail
}

func splitHead(s string) (string, string) {
	s = strings.TrimLeft(s, " \t\r\n")

	i := strings.IndexAny(s, " \t\r\n")
	if i < 0 {
		return s, ""
	}

	return s[:i], dropSeparator(s[i:])
}

func dropSeparator(s string) string {
	s = strings.TrimLeft(s, " \t")
	if strings.HasPrefix(s, "\r\n") {
		return s[2:]
	}
	return strings.TrimLeft(s, "\r\n")
}

func resolveFilePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("path kosong")
	}

	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~"))
	}

	return filepath.Abs(path)
}

func protectedFilePath(path string) bool {
	if filepath.Dir(path) == path {
		return true
	}

	if cwd, err := os.Getwd(); err == nil && pathContains(path, cwd) {
		return true
	}

	if home, err := os.UserHomeDir(); err == nil && pathContains(path, home) {
		return true
	}

	return false
}

func pathContains(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil &&
		rel != ".." &&
		!strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func stripFileFence(s string) string {
	s = strings.TrimSpace(s)

	if strings.HasPrefix(s, "```") {
		if i := strings.IndexByte(s, '\n'); i >= 0 {
			s = s[i+1:]
		}
	}

	return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "```"))
}

func formatFileOutput(header, output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return header
	}

	if len(output) > maxFileOutput {
		output = output[:maxFileOutput] + "\n... (output dipotong)"
	}

	return fmt.Sprintf("%s\n```text\n%s\n```", header, output)
}

func fileSize(size int64) string {
	switch {
	case size < 1024:
		return fmt.Sprintf("%d B", size)
	case size < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
	}
}
