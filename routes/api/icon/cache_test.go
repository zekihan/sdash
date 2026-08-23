package icon

import (
	"bytes"
	"testing"
)

func TestIconCache(t *testing.T) {
	InitIconCache()

	t.Run("add and get", func(t *testing.T) {
		testIcon := &Icon{
			URL:         "http://example.com/test.png",
			Data:        []byte("test-data"),
			ContentType: "image/png",
		}

		cache.add("test-key", testIcon)

		retrieved, ok := cache.get("test-key")
		if !ok {
			t.Fatal("expected to find cached item")
		}

		if retrieved.URL != testIcon.URL {
			t.Errorf("expected url %s, got %s", testIcon.URL, retrieved.URL)
		}

		if !bytes.Equal(retrieved.Data, testIcon.Data) {
			t.Errorf("expected data %s, got %s", testIcon.Data, retrieved.Data)
		}

		if retrieved.ContentType != testIcon.ContentType {
			t.Errorf("expected contentType %s, got %s", testIcon.ContentType, retrieved.ContentType)
		}
	})

	t.Run("get non-existent key", func(t *testing.T) {
		_, ok := cache.get("non-existent-key")
		if ok {
			t.Error("expected not to find non-existent key")
		}
	})

	t.Run("cache overwrite", func(t *testing.T) {
		key := "overwrite-test"

		cache.add(key, testIcon([]byte("original")))
		cache.add(key, testIcon([]byte("updated")))

		retrieved, ok := cache.get(key)
		if !ok {
			t.Fatal("expected to find cached item")
		}

		if string(retrieved.Data) != "updated" {
			t.Errorf("expected updated data, got %s", retrieved.Data)
		}
	})
}

func TestInitIconCache(t *testing.T) {
	// Test that initialization doesn't panic and creates a working cache
	InitIconCache()

	if cache == nil {
		t.Fatal("expected cache to be initialized")
	}

	if cache.cache == nil {
		t.Fatal("expected cache.cache to be initialized")
	}

	// Test that cache works after initialization
	icon := testIcon([]byte("init-test"))
	cache.add("init-test", icon)

	retrieved, ok := cache.get("init-test")
	if !ok {
		t.Error("cache should work after initialization")
	}

	if string(retrieved.Data) != "init-test" {
		t.Error("cache data should be preserved")
	}
}

func testIcon(data []byte) *Icon {
	icon := new(Icon)
	icon.Data = data

	return icon
}

func TestIconCacheGetMiss(t *testing.T) {
	InitIconCache()

	_, ok := cache.get("non-existent-key")
	if ok {
		t.Fatal("expected cache miss for non-existent key")
	}
}
