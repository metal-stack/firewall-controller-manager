package cache

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type (
	FetchFunc[O any] func(ctx context.Context, key any) (O, error)

	Cache[O any] struct {
		expiration time.Duration
		fetch      FetchFunc[O]
		entries    sync.Map
	}

	entry[O any] struct {
		value     O
		expiresAt time.Time
	}
)

func New[O any](expiration time.Duration, fetch FetchFunc[O]) *Cache[O] {
	return &Cache[O]{
		expiration: expiration,
		fetch:      fetch,
		entries:    sync.Map{},
	}
}

func (c *Cache[O]) Get(ctx context.Context, key any) (O, error) {
	v, ok := c.entries.Load(key)
	if !ok {
		o, err := c.fetch(ctx, key)
		if err != nil {
			var zero O
			return zero, fmt.Errorf("error fetching cache entry: %w", err)
		}

		entry := newEntry(o, c.expiration)

		c.entries.Store(key, entry)

		return entry.value, nil
	}

	entry, ok := v.(*entry[O])
	if !ok {
		c.entries.Delete(key)
		var zero O
		return zero, fmt.Errorf("invalid cache entry, please retry")
	}

	if entry.expired() {
		o, err := c.fetch(ctx, key)
		if err != nil {
			var zero O
			return zero, fmt.Errorf("error fetching cache entry: %w", err)
		}

		entry.update(o, c.expiration)

		c.entries.Store(key, entry)
	}

	return entry.value, nil
}

func newEntry[O any](o O, expiration time.Duration) *entry[O] {
	return &entry[O]{
		value:     o,
		expiresAt: time.Now().Add(expiration),
	}
}

func (e *entry[O]) expired() bool {
	return time.Since(e.expiresAt) > 0
}

func (e *entry[O]) update(o O, expiration time.Duration) {
	e.value = o
	e.expiresAt = time.Now().Add(expiration)
}
