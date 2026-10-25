package icon

import (
	"container/list"
	"sync"
	"time"
)

const (
	defaultIconCacheSize  = 100
	defaultIconCacheTTL   = 10 * time.Minute
	defaultIconCacheBytes = 8 << 20
)

type cacheEntry struct {
	expires time.Time
	icon    *Icon
	key     string
}

// Cached icons are immutable, validated PNGs. Both entry count and total image bytes are bounded.
type iconCache struct {
	entries map[string]*list.Element
	order   *list.List
	now     func() time.Time
	bytes   int
	lock    sync.Mutex
}

func newIconCache() *iconCache {
	return &iconCache{
		entries: make(map[string]*list.Element),
		order:   list.New(),
		now:     time.Now,
		bytes:   0,
		lock:    sync.Mutex{},
	}
}

func InitIconCache() {
	defaultProxy.lock.Lock()
	defer defaultProxy.lock.Unlock()

	defaultProxy.cache = newIconCache()
}

func (cache *iconCache) get(key string) (*Icon, bool) {
	cache.lock.Lock()
	defer cache.lock.Unlock()

	entry, ok := cache.entries[key]
	if !ok {
		return nil, false
	}

	value, ok := entry.Value.(cacheEntry)
	if !ok {
		panic("invalid icon cache entry")
	}

	if !cache.now().Before(value.expires) {
		cache.remove(entry)

		return nil, false
	}

	cache.order.MoveToFront(entry)

	return value.icon, true
}

func (cache *iconCache) add(key string, value *Icon) {
	if len(value.Data) > maxIconBytes || cap(value.Data) > defaultIconCacheBytes {
		return
	}

	cache.lock.Lock()
	defer cache.lock.Unlock()

	if existing, ok := cache.entries[key]; ok {
		cache.remove(existing)
	}

	for cache.order.Len() >= defaultIconCacheSize || cache.bytes+cap(value.Data) > defaultIconCacheBytes {
		cache.remove(cache.order.Back())
	}

	entry := cache.order.PushFront(
		cacheEntry{key: key, icon: value, expires: cache.now().Add(defaultIconCacheTTL)},
	)
	cache.entries[key] = entry
	cache.bytes += cap(value.Data)
}

func (cache *iconCache) remove(entry *list.Element) {
	value, ok := entry.Value.(cacheEntry)
	if !ok {
		panic("invalid icon cache entry")
	}

	delete(cache.entries, value.key)
	cache.bytes -= cap(value.icon.Data)
	cache.order.Remove(entry)
}
