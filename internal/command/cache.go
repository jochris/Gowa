package command

import (
	"sync"
	"time"
)

type SentCache struct {
	mu  sync.RWMutex
	ids map[string]time.Time
}

func NewSentCache() *SentCache {
	sc := &SentCache{
		ids: make(map[string]time.Time),
	}
	go sc.cleanupLoop()
	return sc
}

func (sc *SentCache) Add(id string) {
	if id == "" {
		return
	}
	sc.mu.Lock()
	sc.ids[id] = time.Now().Add(2 * time.Minute)
	sc.mu.Unlock()
}

func (sc *SentCache) Has(id string) bool {
	if id == "" {
		return false
	}
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	expiry, exists := sc.ids[id]
	if !exists {
		return false
	}
	return time.Now().Before(expiry)
}

func (sc *SentCache) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		now := time.Now()
		sc.mu.Lock()
		for id, expiry := range sc.ids {
			if now.After(expiry) {
				delete(sc.ids, id)
			}
		}
		sc.mu.Unlock()
	}
}
