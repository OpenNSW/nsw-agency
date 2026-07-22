package storage

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// KeySearcher abstracts searching for storage keys in stored application records.
type KeySearcher interface {
	HasStorageKey(ctx context.Context, key string) (bool, error)
}

// MemoryAndStoreKeyValidator validates storage keys against an in-memory pending upload cache and application store.
type MemoryAndStoreKeyValidator struct {
	searcher    KeySearcher
	mu          sync.RWMutex
	pendingKeys map[string]time.Time
	ttl         time.Duration
}

// NewMemoryAndStoreKeyValidator creates a KeyValidator backed by memory cache and KeySearcher.
func NewMemoryAndStoreKeyValidator(searcher KeySearcher, pendingTTL time.Duration) *MemoryAndStoreKeyValidator {
	if pendingTTL <= 0 {
		pendingTTL = 2 * time.Hour
	}
	return &MemoryAndStoreKeyValidator{
		searcher:    searcher,
		pendingKeys: make(map[string]time.Time),
		ttl:         pendingTTL,
	}
}

// TrackUpload registers a newly created upload key in memory with an expiration TTL.
func (v *MemoryAndStoreKeyValidator) TrackUpload(ctx context.Context, key string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.pendingKeys[key] = time.Now().Add(v.ttl)
	return nil
}

// KeyExists checks if the storage key is present in active pending uploads or the database.
func (v *MemoryAndStoreKeyValidator) KeyExists(ctx context.Context, key string) (bool, error) {
	v.mu.Lock()
	exp, exists := v.pendingKeys[key]
	if exists {
		if time.Now().Before(exp) {
			v.mu.Unlock()
			return true, nil
		}
		delete(v.pendingKeys, key)
	}
	v.mu.Unlock()

	if v.searcher == nil {
		return false, nil
	}

	found, err := v.searcher.HasStorageKey(ctx, key)
	if err != nil {
		return false, fmt.Errorf("storage key DB lookup failed: %w", err)
	}
	return found, nil
}
