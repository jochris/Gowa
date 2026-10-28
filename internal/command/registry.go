package command

import (
	"sort"
	"strings"
	"sync"
)

var (
	mu          sync.RWMutex
	commands    = make(map[string]*Command)
	allCommands []*Command
)

func Register(cmd *Command) {
	mu.Lock()
	defer mu.Unlock()

	cmd.Cmd = strings.ToLower(strings.TrimSpace(cmd.Cmd))
	cmd.Categ = strings.ToLower(strings.TrimSpace(cmd.Categ))
	if cmd.Categ == "" {
		cmd.Categ = "other"
	}

	allCommands = append(allCommands, cmd)
	commands[cmd.Cmd] = cmd

	for _, alias := range cmd.Alias {
		cleanedAlias := strings.ToLower(strings.TrimSpace(alias))
		if cleanedAlias != "" {
			commands[cleanedAlias] = cmd
		}
	}
}

func Find(name string) (*Command, bool) {
	mu.RLock()
	defer mu.RUnlock()

	cmd, ok := commands[strings.ToLower(strings.TrimSpace(name))]
	return cmd, ok
}

func GetAll() []*Command {
	mu.RLock()
	defer mu.RUnlock()

	res := make([]*Command, len(allCommands))
	copy(res, allCommands)
	return res
}

func GetCategories() []string {
	mu.RLock()
	defer mu.RUnlock()

	catMap := make(map[string]struct{})
	for _, cmd := range allCommands {
		catMap[cmd.Categ] = struct{}{}
	}

	cats := make([]string, 0, len(catMap))
	for cat := range catMap {
		cats = append(cats, cat)
	}
	sort.Strings(cats)
	return cats
}

func GetByCategory(categ string) []*Command {
	mu.RLock()
	defer mu.RUnlock()

	var result []*Command
	norm := strings.ToLower(strings.TrimSpace(categ))
	for _, cmd := range allCommands {
		if strings.EqualFold(cmd.Categ, norm) {
			result = append(result, cmd)
		}
	}
	return result
}
