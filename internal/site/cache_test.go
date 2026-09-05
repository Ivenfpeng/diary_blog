package site

import (
	"testing"
	"time"
)

func TestCacheInvalidatesPublicationAndDependentKeys(t *testing.T) {
	cache := NewCache(time.Minute)
	keys := []string{
		ArticleCacheKey("operational-cache"),
		HomeCacheKey(),
		ArchiveCacheKey(),
		CategoryCacheKey("go"),
		TagCacheKey("testing"),
		RSSCacheKey(),
		SitemapCacheKey(),
	}
	for _, key := range keys {
		cache.Set(key, []byte(key))
	}

	cache.InvalidatePublication("operational-cache")

	for _, key := range keys {
		if _, ok := cache.Get(key); ok {
			t.Errorf("cache retained %q after publication invalidation", key)
		}
	}
}

func TestCacheExpiresEntries(t *testing.T) {
	clock := time.Now()
	cache := NewCacheWithClock(time.Minute, func() time.Time { return clock })
	cache.Set(HomeCacheKey(), []byte("home"))
	clock = clock.Add(time.Minute)
	if _, ok := cache.Get(HomeCacheKey()); ok {
		t.Fatal("cache retained expired entry")
	}
}
