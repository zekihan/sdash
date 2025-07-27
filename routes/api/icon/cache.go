package icon

import (
	"github.com/hashicorp/golang-lru/v2/expirable"
	"sync"
	"time"
)

var cache *iconCache

type iconCache struct {
	cache *expirable.LRU[string, Icon]
	lock  sync.RWMutex
}

func InitIconCache() {
	cache = &iconCache{
		cache: expirable.NewLRU[string, Icon](100, nil, 10*time.Minute),
	}
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
