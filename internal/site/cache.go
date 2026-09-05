package site

import (
	"strings"
	"sync"
	"time"
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
}

// Cache is a process-local TTL cache for public response data.
type Cache struct {
	mu      sync.Mutex
	ttl     time.Duration
	now     func() time.Time
	entries map[string]cacheEntry
}

func NewCache(ttl time.Duration) *Cache { return NewCacheWithClock(ttl, time.Now) }

func NewCacheWithClock(ttl time.Duration, clock func() time.Time) *Cache {
	if clock == nil {
		clock = time.Now
	}
	return &Cache{ttl: ttl, now: clock, entries: make(map[string]cacheEntry)}
}

func ArticleCacheKey(slug string) string  { return articleKeyPrefix + slug }
func HomeCacheKey() string                { return homeKey }
func ArchiveCacheKey() string             { return archiveKey }
func CategoryCacheKey(slug string) string { return categoryKeyPrefix + slug }
func TagCacheKey(slug string) string      { return tagKeyPrefix + slug }
func RSSCacheKey() string                 { return rssKey }
func SitemapCacheKey() string             { return sitemapKey }

func (c *Cache) Get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok || !c.now().Before(entry.expiresAt) {
		delete(c.entries, key)
		return nil, false
	}
	return append([]byte(nil), entry.value...), true
}

func (c *Cache) Set(key string, value []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = cacheEntry{value: append([]byte(nil), value...), expiresAt: c.now().Add(c.ttl)}
}

// InvalidatePublication removes all public data that can contain a published
// article. Article, category, and tag keys are invalidated by prefix because a
// post may have been moved or renamed between publication states.
func (c *Cache) InvalidatePublication(slug string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, HomeCacheKey())
	delete(c.entries, ArchiveCacheKey())
	delete(c.entries, RSSCacheKey())
	delete(c.entries, SitemapCacheKey())
	for key := range c.entries {
		if strings.HasPrefix(key, articleKeyPrefix) || strings.HasPrefix(key, categoryKeyPrefix) || strings.HasPrefix(key, tagKeyPrefix) {
			delete(c.entries, key)
		}
	}
}
