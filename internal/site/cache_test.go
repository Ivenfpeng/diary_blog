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

func TestCacheEvictsLeastRecentlyUsedEntryAtEntryLimit(t *testing.T) {
	clock := time.Now()
	cache := NewCacheWithLimits(time.Minute, func() time.Time { return clock }, 2, 100)
	cache.Set("first", []byte("111"))
	clock = clock.Add(time.Second)
	cache.Set("second", []byte("22"))
	if _, ok := cache.Get("first"); !ok {
		t.Fatal("expected first entry before eviction")
	}
	clock = clock.Add(time.Second)
	cache.Set("third", []byte("333"))

	if _, ok := cache.Get("second"); ok {
		t.Fatal("cache retained the least recently used entry")
	}
	if _, ok := cache.Get("first"); !ok {
		t.Fatal("cache evicted the recently used entry")
	}
	if _, ok := cache.Get("third"); !ok {
		t.Fatal("cache did not retain the newest entry")
	}
}

func TestCacheEvictsLeastRecentlyUsedEntryAtByteLimit(t *testing.T) {
	cache := NewCacheWithLimits(time.Minute, time.Now, 10, 5)
	cache.Set("first", []byte("111"))
	cache.Set("second", []byte("222"))

	if _, ok := cache.Get("first"); ok {
		t.Fatal("cache retained an entry beyond its byte limit")
	}
	if _, ok := cache.Get("second"); !ok {
		t.Fatal("cache discarded the newest entry while enforcing its byte limit")
	}
}

func TestCacheSetCleansExpiredEntriesAndRejectsOversizedValues(t *testing.T) {
	clock := time.Now()
	cache := NewCacheWithLimits(time.Minute, func() time.Time { return clock }, 10, 4)
	cache.Set("expired", []byte("old"))
	clock = clock.Add(time.Minute)
	cache.Set("fresh", []byte("new"))
	cache.Set("oversized", []byte("12345"))

	if cache.EntryCount() != 1 {
		t.Fatalf("entry count = %d, want only the fresh entry", cache.EntryCount())
	}
	if _, ok := cache.Get("fresh"); !ok {
		t.Fatal("cache discarded the fresh entry")
	}
	if _, ok := cache.Get("oversized"); ok {
		t.Fatal("cache retained an oversized entry")
	}
}

func TestCacheGenerationPreventsStaleFillAfterInvalidation(t *testing.T) {
	cache := NewCache(time.Minute)
	generation := cache.Generation()
	cache.InvalidatePublication("old")

	if cache.SetIfGeneration(HomeCacheKey(), []byte("stale"), generation) {
		t.Fatal("stale fill was inserted after invalidation")
	}
	if _, ok := cache.Get(HomeCacheKey()); ok {
		t.Fatal("stale fill resurrected invalidated content")
	}
}
