package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// RepositoryCache stores GitHub repository tag information
type RepositoryCache struct {
	mu           sync.RWMutex
	entries      map[string]*CacheEntry
	cacheDir     string
	ttl          time.Duration
	memoryOnly   bool
}

// CacheEntry represents a cached repository entry
type CacheEntry struct {
	// Repository owner/name (e.g., "hashicorp/terraform")
	Repository string `json:"repository"`

	// List of version tags
	Tags []string `json:"tags"`

	// Timestamp when this entry was cached
	CachedAt time.Time `json:"cached_at"`

	// Time to live for this entry
	TTL time.Duration `json:"ttl"`
}

// New creates a new repository cache with disk persistence
func New(cacheDir string, ttl time.Duration) (*RepositoryCache, error) {
	return newCache(cacheDir, ttl, false)
}

// NewMemoryOnly creates a new in-memory only cache (no disk persistence)
func NewMemoryOnly(ttl time.Duration) *RepositoryCache {
	return &RepositoryCache{
		entries:    make(map[string]*CacheEntry),
		ttl:        ttl,
		memoryOnly: true,
	}
}

// newCache creates a new repository cache
func newCache(cacheDir string, ttl time.Duration, memoryOnly bool) (*RepositoryCache, error) {
	if memoryOnly {
		return NewMemoryOnly(ttl), nil
	}

	if cacheDir == "" {
		// Default to user's cache directory
		userCacheDir, err := os.UserCacheDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user cache directory: %w", err)
		}
		cacheDir = filepath.Join(userCacheDir, "terranovate")
	}

	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	cache := &RepositoryCache{
		entries:    make(map[string]*CacheEntry),
		cacheDir:   cacheDir,
		ttl:        ttl,
		memoryOnly: false,
	}

	// Load existing cache from disk
	if err := cache.load(); err != nil {
		log.Warn().Err(err).Msg("failed to load cache from disk, starting fresh")
	}

	return cache, nil
}

// Get retrieves tags for a repository from cache
func (c *RepositoryCache) Get(repo string) ([]string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[repo]
	if !exists {
		return nil, false
	}

	// Check if entry has expired
	if time.Since(entry.CachedAt) > entry.TTL {
		log.Debug().Str("repository", repo).Msg("cache entry expired")
		return nil, false
	}

	log.Debug().Str("repository", repo).Int("tags", len(entry.Tags)).Msg("cache hit")
	return entry.Tags, true
}

// Set stores tags for a repository in cache
func (c *RepositoryCache) Set(repo string, tags []string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[repo] = &CacheEntry{
		Repository: repo,
		Tags:       tags,
		CachedAt:   time.Now(),
		TTL:        c.ttl,
	}

	log.Debug().Str("repository", repo).Int("tags", len(tags)).Msg("cached repository tags")

	// Persist to disk asynchronously (only if not memory-only)
	if !c.memoryOnly {
		go func() {
			if err := c.save(); err != nil {
				log.Warn().Err(err).Msg("failed to save cache to disk")
			}
		}()
	}
}

// Clear removes all entries from cache
func (c *RepositoryCache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*CacheEntry)

	// Remove cache file
	cacheFile := filepath.Join(c.cacheDir, "repository-cache.json")
	if err := os.Remove(cacheFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove cache file: %w", err)
	}

	log.Info().Msg("cache cleared")
	return nil
}

// load reads cache from disk
func (c *RepositoryCache) load() error {
	// Skip loading if memory-only
	if c.memoryOnly {
		return nil
	}

	cacheFile := filepath.Join(c.cacheDir, "repository-cache.json")

	data, err := os.ReadFile(cacheFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No cache file yet, not an error
		}
		return fmt.Errorf("failed to read cache file: %w", err)
	}

	var entries map[string]*CacheEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("failed to parse cache file: %w", err)
	}

	// Remove expired entries during load
	validEntries := 0
	for repo, entry := range entries {
		if time.Since(entry.CachedAt) <= entry.TTL {
			c.entries[repo] = entry
			validEntries++
		}
	}

	log.Debug().
		Int("total", len(entries)).
		Int("valid", validEntries).
		Int("expired", len(entries)-validEntries).
		Msg("loaded cache from disk")

	return nil
}

// save writes cache to disk
func (c *RepositoryCache) save() error {
	// Skip saving if memory-only
	if c.memoryOnly {
		return nil
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	cacheFile := filepath.Join(c.cacheDir, "repository-cache.json")

	data, err := json.MarshalIndent(c.entries, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache: %w", err)
	}

	if err := os.WriteFile(cacheFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

// Stats returns cache statistics
func (c *RepositoryCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := CacheStats{
		TotalEntries: len(c.entries),
		MemoryOnly:   c.memoryOnly,
	}

	for _, entry := range c.entries {
		if time.Since(entry.CachedAt) <= entry.TTL {
			stats.ValidEntries++
		} else {
			stats.ExpiredEntries++
		}
	}

	return stats
}

// IsMemoryOnly returns true if cache is memory-only (no disk persistence)
func (c *RepositoryCache) IsMemoryOnly() bool {
	return c.memoryOnly
}

// CacheStats contains cache statistics
type CacheStats struct {
	TotalEntries   int
	ValidEntries   int
	ExpiredEntries int
	MemoryOnly     bool
}
