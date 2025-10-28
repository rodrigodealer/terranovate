package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	tmpDir := t.TempDir()
	ttl := 1 * time.Hour

	cache, err := New(tmpDir, ttl)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if cache == nil {
		t.Fatal("New() returned nil cache")
	}

	if cache.memoryOnly {
		t.Error("New() created memory-only cache, want disk-backed")
	}

	if cache.ttl != ttl {
		t.Errorf("cache.ttl = %v, want %v", cache.ttl, ttl)
	}

	if cache.cacheDir != tmpDir {
		t.Errorf("cache.cacheDir = %s, want %s", cache.cacheDir, tmpDir)
	}
}

func TestNew_DefaultCacheDir(t *testing.T) {
	cache, err := New("", 1*time.Hour)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if cache.cacheDir == "" {
		t.Error("New() did not set default cache directory")
	}

	// Verify cache directory was created
	if _, err := os.Stat(cache.cacheDir); os.IsNotExist(err) {
		t.Error("New() did not create cache directory")
	}
}

func TestNewMemoryOnly(t *testing.T) {
	ttl := 1 * time.Hour
	cache := NewMemoryOnly(ttl)

	if cache == nil {
		t.Fatal("NewMemoryOnly() returned nil")
	}

	if !cache.memoryOnly {
		t.Error("NewMemoryOnly() created disk-backed cache, want memory-only")
	}

	if cache.ttl != ttl {
		t.Errorf("cache.ttl = %v, want %v", cache.ttl, ttl)
	}

	if cache.cacheDir != "" {
		t.Errorf("cache.cacheDir = %s, want empty", cache.cacheDir)
	}
}

func TestGetSet(t *testing.T) {
	cache := NewMemoryOnly(1 * time.Hour)

	// Test Get on empty cache
	tags, found := cache.Get("hashicorp/terraform")
	if found {
		t.Error("Get() found entry in empty cache")
	}
	if tags != nil {
		t.Error("Get() returned non-nil tags for missing entry")
	}

	// Test Set and Get
	testTags := []string{"v1.0.0", "v1.1.0", "v1.2.0"}
	cache.Set("hashicorp/terraform", testTags)

	tags, found = cache.Get("hashicorp/terraform")
	if !found {
		t.Fatal("Get() did not find entry after Set()")
	}

	if len(tags) != len(testTags) {
		t.Fatalf("Get() returned %d tags, want %d", len(tags), len(testTags))
	}

	for i, tag := range tags {
		if tag != testTags[i] {
			t.Errorf("Get() tag[%d] = %s, want %s", i, tag, testTags[i])
		}
	}
}

func TestGet_Expiration(t *testing.T) {
	cache := NewMemoryOnly(100 * time.Millisecond)

	testTags := []string{"v1.0.0", "v1.1.0"}
	cache.Set("hashicorp/terraform", testTags)

	// Should be found immediately
	_, found := cache.Get("hashicorp/terraform")
	if !found {
		t.Error("Get() did not find recently set entry")
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should be expired
	_, found = cache.Get("hashicorp/terraform")
	if found {
		t.Error("Get() found expired entry")
	}
}

func TestClear(t *testing.T) {
	cache := NewMemoryOnly(1 * time.Hour)

	// Add some entries
	cache.Set("repo1", []string{"v1.0.0"})
	cache.Set("repo2", []string{"v2.0.0"})
	cache.Set("repo3", []string{"v3.0.0"})

	// Verify entries exist
	if _, found := cache.Get("repo1"); !found {
		t.Error("Entry repo1 not found before clear")
	}

	// Clear cache
	err := cache.Clear()
	if err != nil {
		t.Errorf("Clear() error = %v", err)
	}

	// Verify all entries are gone
	if _, found := cache.Get("repo1"); found {
		t.Error("Entry repo1 found after clear")
	}
	if _, found := cache.Get("repo2"); found {
		t.Error("Entry repo2 found after clear")
	}
	if _, found := cache.Get("repo3"); found {
		t.Error("Entry repo3 found after clear")
	}
}

func TestStats(t *testing.T) {
	cache := NewMemoryOnly(100 * time.Millisecond)

	// Empty cache stats
	stats := cache.Stats()
	if stats.TotalEntries != 0 {
		t.Errorf("Stats() TotalEntries = %d, want 0", stats.TotalEntries)
	}
	if stats.ValidEntries != 0 {
		t.Errorf("Stats() ValidEntries = %d, want 0", stats.ValidEntries)
	}
	if !stats.MemoryOnly {
		t.Error("Stats() MemoryOnly = false, want true")
	}

	// Add entries
	cache.Set("repo1", []string{"v1.0.0"})
	cache.Set("repo2", []string{"v2.0.0"})

	stats = cache.Stats()
	if stats.TotalEntries != 2 {
		t.Errorf("Stats() TotalEntries = %d, want 2", stats.TotalEntries)
	}
	if stats.ValidEntries != 2 {
		t.Errorf("Stats() ValidEntries = %d, want 2", stats.ValidEntries)
	}
	if stats.ExpiredEntries != 0 {
		t.Errorf("Stats() ExpiredEntries = %d, want 0", stats.ExpiredEntries)
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	stats = cache.Stats()
	if stats.TotalEntries != 2 {
		t.Errorf("Stats() TotalEntries = %d, want 2", stats.TotalEntries)
	}
	if stats.ValidEntries != 0 {
		t.Errorf("Stats() ValidEntries = %d, want 0", stats.ValidEntries)
	}
	if stats.ExpiredEntries != 2 {
		t.Errorf("Stats() ExpiredEntries = %d, want 2", stats.ExpiredEntries)
	}
}

func TestIsMemoryOnly(t *testing.T) {
	memCache := NewMemoryOnly(1 * time.Hour)
	if !memCache.IsMemoryOnly() {
		t.Error("Memory-only cache IsMemoryOnly() = false, want true")
	}

	tmpDir := t.TempDir()
	diskCache, err := New(tmpDir, 1*time.Hour)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if diskCache.IsMemoryOnly() {
		t.Error("Disk-backed cache IsMemoryOnly() = true, want false")
	}
}

func TestSaveLoad(t *testing.T) {
	tmpDir := t.TempDir()
	ttl := 1 * time.Hour

	// Create cache and add entries
	cache1, err := New(tmpDir, ttl)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	testData := map[string][]string{
		"hashicorp/terraform": {"v1.0.0", "v1.1.0", "v1.2.0"},
		"hashicorp/aws":       {"v4.0.0", "v4.1.0"},
		"kubernetes/kubectl":  {"v1.28.0", "v1.29.0"},
	}

	for repo, tags := range testData {
		cache1.Set(repo, tags)
	}

	// Wait a bit to ensure save completes (it's async)
	time.Sleep(100 * time.Millisecond)

	// Create new cache with same directory (should load existing data)
	cache2, err := New(tmpDir, ttl)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Verify all entries were loaded
	for repo, expectedTags := range testData {
		tags, found := cache2.Get(repo)
		if !found {
			t.Errorf("Get(%s) not found after reload", repo)
			continue
		}

		if len(tags) != len(expectedTags) {
			t.Errorf("Get(%s) returned %d tags, want %d", repo, len(tags), len(expectedTags))
			continue
		}

		for i, tag := range tags {
			if tag != expectedTags[i] {
				t.Errorf("Get(%s) tag[%d] = %s, want %s", repo, i, tag, expectedTags[i])
			}
		}
	}
}

func TestLoad_ExpiredEntries(t *testing.T) {
	tmpDir := t.TempDir()
	ttl := 50 * time.Millisecond

	// Create cache and add entries
	cache1, err := New(tmpDir, ttl)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	cache1.Set("repo1", []string{"v1.0.0"})
	cache1.Set("repo2", []string{"v2.0.0"})

	// Wait for save
	time.Sleep(100 * time.Millisecond)

	// Wait for entries to expire
	time.Sleep(100 * time.Millisecond)

	// Create new cache (should load but filter expired entries)
	cache2, err := New(tmpDir, ttl)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Entries should not be found because they expired
	if _, found := cache2.Get("repo1"); found {
		t.Error("Get() found expired entry after reload")
	}
}

func TestConcurrentAccess(t *testing.T) {
	cache := NewMemoryOnly(1 * time.Hour)

	done := make(chan bool)

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			cache.Set("repo", []string{"v1.0.0"})
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for i := 0; i < 100; i++ {
			cache.Get("repo")
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done
}

func TestClearWithDiskCache(t *testing.T) {
	tmpDir := t.TempDir()
	cache, err := New(tmpDir, 1*time.Hour)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	cache.Set("repo1", []string{"v1.0.0"})
	time.Sleep(100 * time.Millisecond) // Wait for async save

	// Verify cache file exists
	cacheFile := filepath.Join(tmpDir, "repository-cache.json")
	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		t.Error("Cache file was not created")
	}

	// Clear cache
	err = cache.Clear()
	if err != nil {
		t.Errorf("Clear() error = %v", err)
	}

	// Verify cache file is removed
	if _, err := os.Stat(cacheFile); !os.IsNotExist(err) {
		t.Error("Cache file was not removed after Clear()")
	}
}

func TestMemoryOnlyNoSave(t *testing.T) {
	cache := NewMemoryOnly(1 * time.Hour)

	cache.Set("repo1", []string{"v1.0.0"})
	time.Sleep(100 * time.Millisecond) // Give time for potential save

	// Since it's memory-only, no cache directory should exist
	if cache.cacheDir != "" {
		t.Error("Memory-only cache should not have cacheDir set")
	}
}
