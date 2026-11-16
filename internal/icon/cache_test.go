package icon

import (
	"strconv"
	"testing"
	"time"
)

func TestIconCache(t *testing.T) {
	cache := newIconCache()
	cache.add("key", &Icon{Data: []byte("original")})
	cache.add("key", &Icon{Data: []byte("updated")})

	value, ok := cache.get("key")
	if !ok || string(value.Data) != "updated" || cache.bytes != len(value.Data) ||
		cache.order.Len() != 1 {
		t.Fatal("cache overwrite/accounting failed")
	}

	if _, ok := cache.get("missing"); ok {
		t.Fatal("unexpected cache hit")
	}
}

func TestIconCacheByteBudget(t *testing.T) {
	cache := newIconCache()
	for index := range 20 {
		cache.add(strconv.Itoa(index), &Icon{Data: make([]byte, maxIconBytes)})

		if cache.bytes > defaultIconCacheBytes {
			t.Fatalf("cache budget exceeded: %d", cache.bytes)
		}
	}

	if cache.order.Len() != defaultIconCacheBytes/maxIconBytes {
		t.Fatal("expected byte-budget eviction")
	}

	before := cache.bytes
	cache.add("oversized", &Icon{Data: make([]byte, maxIconBytes+1)})

	if _, ok := cache.get("oversized"); ok || cache.bytes != before {
		t.Fatal("oversized icon entered cache")
	}
}

func TestIconCacheEntryBudgetAndLRU(t *testing.T) {
	cache := newIconCache()
	for index := range defaultIconCacheSize {
		cache.add(strconv.Itoa(index), &Icon{Data: []byte{1}})
	}

	if _, ok := cache.get("0"); !ok {
		t.Fatal("missing oldest icon")
	}

	cache.add("new", &Icon{Data: []byte{1}})

	if _, ok := cache.get("1"); ok {
		t.Fatal("least recently used icon was not evicted")
	}

	if _, ok := cache.get("0"); !ok || cache.order.Len() != defaultIconCacheSize {
		t.Fatal("cache access did not preserve entry/count budget")
	}
}

func TestIconCacheExpiration(t *testing.T) {
	cache := newIconCache()
	now := time.Now()
	cache.now = func() time.Time { return now }
	cache.add("key", &Icon{Data: []byte{1}})

	now = now.Add(defaultIconCacheTTL)

	if _, ok := cache.get("key"); ok || cache.bytes != 0 || cache.order.Len() != 0 {
		t.Fatal("expired entry retained or byte accounting incorrect")
	}
}

func TestInitIconCache(t *testing.T) {
	InitIconCache()

	if defaultProxy.cache == nil || defaultProxy.cache.entries == nil {
		t.Fatal("cache not initialized")
	}
}

func TestIconCacheCountsRetainedCapacity(t *testing.T) {
	cache := newIconCache()

	for index := range 20 {
		data := make([]byte, 1, maxIconBytes)
		cache.add(strconv.Itoa(index), &Icon{Data: data})
	}

	if cache.bytes != defaultIconCacheBytes ||
		cache.order.Len() != defaultIconCacheBytes/maxIconBytes {
		t.Fatal("cache must count retained capacity, not just the image length")
	}
}
