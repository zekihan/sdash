package icon

import (
	"sync"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
)

var cache *iconCache

const (
	defaultIconCacheSize = 100
	defaultIconCacheTTL  = 10 * time.Minute
)

type iconCache struct {
	cache *expirable.LRU[string, Icon]
	lock  sync.RWMutex
}

func InitIconCache() {
	cache = new(iconCache)
	cache.cache = expirable.NewLRU[string, Icon](defaultIconCacheSize, nil, defaultIconCacheTTL)
}

func (c *iconCache) get(key string) (*Icon, bool) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	if value, ok := c.cache.Get(key); ok {
		return &value, true
	}

	return nil, false
}

func (c *iconCache) add(key string, value *Icon) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.cache.Add(key, *value)
}
