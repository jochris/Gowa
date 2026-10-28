package command

import (
	"sync"
	"time"
)

type SentCache struct {
	mu     sync.RWMutex
	ids    map[string]time.Time
	stopCh chan struct{}
	once   sync.Once
}

func NewSentCache() *SentCache {
	sc := &SentCache{
		ids:    make(map[string]time.Time),
		stopCh: make(chan struct{}),
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
	expiry, exists := sc.ids[id]
	sc.mu.RUnlock()

	if !exists {
		return false
	}
	return time.Now().Before(expiry)
}

func (sc *SentCache) Stop() {
	sc.once.Do(func() {
		close(sc.stopCh)
	})
}

func (sc *SentCache) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now()
			sc.mu.Lock()
			for id, expiry := range sc.ids {
				if now.After(expiry) {
					delete(sc.ids, id)
				}
			}
			sc.mu.Unlock()
		case <-sc.stopCh:
			return
		}
	}
}
