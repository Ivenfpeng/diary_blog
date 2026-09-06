package site

import (
	"container/list"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	defaultCacheMaxEntries = 256
	defaultCacheMaxBytes   = 8 * 1024 * 1024
)

const (
	articleKeyPrefix  = "article:"
	categoryKeyPrefix = "category:"
	tagKeyPrefix      = "tag:"
	homeKey           = "home"
	archiveKey        = "archive"
	rssKey            = "rss"
	sitemapKey        = "sitemap"
)

type cacheEntry struct {
	value     []byte
	expiresAt time.Time
	element   *list.Element
}

// Cache is a process-local TTL cache for public response data.
type Cache struct {
	mu         sync.Mutex
	ttl        time.Duration
	now        func() time.Time
	entries    map[string]*cacheEntry
	recency    *list.List
	maxEntries int
	maxBytes   int
	totalBytes int
	generation uint64
}

func NewCache(ttl time.Duration) *Cache { return NewCacheWithClock(ttl, time.Now) }

func NewCacheWithClock(ttl time.Duration, clock func() time.Time) *Cache {
	return NewCacheWithLimits(ttl, clock, defaultCacheMaxEntries, defaultCacheMaxBytes)
}

func NewCacheWithLimits(ttl time.Duration, clock func() time.Time, maxEntries, maxBytes int) *Cache {
	if clock == nil {
		clock = time.Now
	}
	if maxEntries < 1 {
		maxEntries = 1
	}
	if maxBytes < 1 {
		maxBytes = 1
	}
	return &Cache{
		ttl: ttl, now: clock, entries: make(map[string]*cacheEntry), recency: list.New(),
		maxEntries: maxEntries, maxBytes: maxBytes,
	}
}

func ArticleCacheKey(slug string) string  { return articleKeyPrefix + slug }
func HomeCacheKey() string                { return homeKey }
func ArchiveCacheKey() string             { return archiveKey }
func CategoryCacheKey(slug string) string { return categoryKeyPrefix + slug }
func TagCacheKey(slug string) string      { return tagKeyPrefix + slug }
func RSSCacheKey() string                 { return rssKey }
func SitemapCacheKey() string             { return sitemapKey }

func HomeCacheKeyForPage(page int) string {
	if page == 1 {
		return HomeCacheKey()
	}
	return fmt.Sprintf("%s:%d", homeKey, page)
}

func ArchiveCacheKeyForPage(page int) string {
	if page == 1 {
		return ArchiveCacheKey()
	}
	return fmt.Sprintf("%s:%d", archiveKey, page)
}

func CategoryCacheKeyForPage(slug string, page int) string {
	if page == 1 {
		return CategoryCacheKey(slug)
	}
	return fmt.Sprintf("%s:%d", CategoryCacheKey(slug), page)
}

func TagCacheKeyForPage(slug string, page int) string {
	if page == 1 {
		return TagCacheKey(slug)
	}
	return fmt.Sprintf("%s:%d", TagCacheKey(slug), page)
}

func (c *Cache) Get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.removeExpiredLocked(c.now())
	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	c.recency.MoveToFront(entry.element)
	return append([]byte(nil), entry.value...), true
}

func (c *Cache) Set(key string, value []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setLocked(key, value, c.now())
}

// Generation returns the current invalidation boundary for coordinated cache
// fills. A response may only be inserted if the generation is unchanged after
// its rendering work completes.
func (c *Cache) Generation() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.generation
}

func (c *Cache) SetIfGeneration(key string, value []byte, generation uint64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if generation != c.generation {
		return false
	}
	return c.setLocked(key, value, c.now())
}

func (c *Cache) EntryCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.removeExpiredLocked(c.now())
	return len(c.entries)
}

func (c *Cache) setLocked(key string, value []byte, now time.Time) bool {
	c.removeExpiredLocked(now)
	if existing, ok := c.entries[key]; ok {
		c.removeLocked(key, existing)
	}
	if len(value) > c.maxBytes {
		return false
	}
	entry := &cacheEntry{value: append([]byte(nil), value...), expiresAt: now.Add(c.ttl)}
	entry.element = c.recency.PushFront(key)
	c.entries[key] = entry
	c.totalBytes += len(entry.value)
	for len(c.entries) > c.maxEntries || c.totalBytes > c.maxBytes {
		oldest := c.recency.Back()
		if oldest == nil {
			break
		}
		oldestKey := oldest.Value.(string)
		c.removeLocked(oldestKey, c.entries[oldestKey])
	}
	return true
}

func (c *Cache) removeExpiredLocked(now time.Time) {
	for key, entry := range c.entries {
		if !now.Before(entry.expiresAt) {
			c.removeLocked(key, entry)
		}
	}
}

func (c *Cache) removeLocked(key string, entry *cacheEntry) {
	if entry == nil {
		return
	}
	delete(c.entries, key)
	c.recency.Remove(entry.element)
	c.totalBytes -= len(entry.value)
}

// InvalidatePublication removes all public data that can contain a published
// article. Article, category, and tag keys are invalidated by prefix because a
// post may have been moved or renamed between publication states.
func (c *Cache) InvalidatePublication(slug string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.generation++
	for key, entry := range c.entries {
		if strings.HasPrefix(key, articleKeyPrefix) || strings.HasPrefix(key, categoryKeyPrefix) || strings.HasPrefix(key, tagKeyPrefix) {
			c.removeLocked(key, entry)
			continue
		}
		switch {
		case key == HomeCacheKey(), strings.HasPrefix(key, HomeCacheKey()+":"),
			key == ArchiveCacheKey(), strings.HasPrefix(key, ArchiveCacheKey()+":"),
			key == RSSCacheKey(), key == SitemapCacheKey():
			c.removeLocked(key, entry)
		}
	}
}

func (c *Cache) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.generation++
	c.entries = make(map[string]*cacheEntry)
	c.recency.Init()
	c.totalBytes = 0
}
