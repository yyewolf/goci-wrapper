package main

import "sync"

// Cache provides thread-safe caching for wrapped images
type Cache struct {
	mu    sync.RWMutex
	cache map[string]bool
}

// NewCache creates a new cache instance
func NewCache() *Cache {
	return &Cache{
		cache: make(map[string]bool),
	}
}

// Has checks if a key exists in the cache
func (c *Cache) Has(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, exists := c.cache[key]
	return exists
}

// Set adds a key to the cache
func (c *Cache) Set(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[key] = true
}

// Key generates a cache key from upstream and target images
func (c *Cache) Key(upstream, target string) string {
	return upstream + "||" + target
}
