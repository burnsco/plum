package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestDoCachedJSONRequest_UsesPersistentCacheUntilExpiry(t *testing.T) {
	cache := &inMemoryProviderCache{entries: make(map[string]*ProviderCacheEntry)}
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		call := calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int32{"value": call})
	}))
	defer server.Close()

	ctx := context.Background()
	first, err := doCachedJSONRequest(ctx, http.DefaultClient, cache, "tmdb", http.MethodGet, server.URL+"/search?q=one", nil, nil, time.Hour, 1)
	if err != nil {
		t.Fatalf("first request: %v", err)
	}
	second, err := doCachedJSONRequest(ctx, http.DefaultClient, cache, "tmdb", http.MethodGet, server.URL+"/search?q=one", nil, nil, time.Hour, 1)
	if err != nil {
		t.Fatalf("second request: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected one upstream call, got %d", calls.Load())
	}
	if string(first.Body) != string(second.Body) {
		t.Fatalf("expected cached body match")
	}

	shortTTL, err := doCachedJSONRequest(ctx, http.DefaultClient, cache, "tmdb", http.MethodGet, server.URL+"/search?q=two", nil, nil, 10*time.Millisecond, 1)
	if err != nil {
		t.Fatalf("short ttl request: %v", err)
	}
	_ = shortTTL
	time.Sleep(25 * time.Millisecond)
	if _, err := doCachedJSONRequest(ctx, http.DefaultClient, cache, "tmdb", http.MethodGet, server.URL+"/search?q=two", nil, nil, 10*time.Millisecond, 1); err != nil {
		t.Fatalf("expired request: %v", err)
	}
	if calls.Load() < 3 {
		t.Fatalf("expected cache expiry refetch, calls=%d", calls.Load())
	}
}

func TestDoCachedJSONRequest_DoesNotCacheUnsuccessfulResponses(t *testing.T) {
	cache := &inMemoryProviderCache{entries: make(map[string]*ProviderCacheEntry)}
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := calls.Add(1)
		if call == 1 {
			http.Error(w, "temporary failure", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int32{"value": call})
	}))
	defer server.Close()

	ctx := context.Background()
	if _, err := doCachedJSONRequest(ctx, http.DefaultClient, cache, "tmdb", http.MethodGet, server.URL+"/search?q=failure", nil, nil, time.Hour, 1); err == nil {
		t.Fatal("expected provider error for first request")
	} else {
		var providerErr *ProviderError
		if !errors.As(err, &providerErr) {
			t.Fatalf("expected provider error, got %T", err)
		}
		if providerErr.StatusCode != http.StatusInternalServerError {
			t.Fatalf("first status = %d", providerErr.StatusCode)
		}
		if !providerErr.Retryable {
			t.Fatal("expected 500 error to be retryable")
		}
	}
	if len(cache.entries) != 0 {
		t.Fatalf("expected no cache entry after failure, got %d", len(cache.entries))
	}

	second, err := doCachedJSONRequest(ctx, http.DefaultClient, cache, "tmdb", http.MethodGet, server.URL+"/search?q=failure", nil, nil, time.Hour, 1)
	if err != nil {
		t.Fatalf("second request: %v", err)
	}
	if second.StatusCode != http.StatusOK {
		t.Fatalf("second status = %d", second.StatusCode)
	}
	if calls.Load() != 2 {
		t.Fatalf("expected two upstream calls, got %d", calls.Load())
	}
	if len(cache.entries) != 1 {
		t.Fatalf("expected one cached success, got %d", len(cache.entries))
	}
}

func TestDoCachedJSONRequest_Classifies429AsRetryable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "slow down", http.StatusTooManyRequests)
	}))
	defer server.Close()

	_, err := doCachedJSONRequest(context.Background(), http.DefaultClient, nil, "tmdb", http.MethodGet, server.URL, nil, nil, time.Hour, 1)
	if err == nil {
		t.Fatal("expected provider error")
	}
	var providerErr *ProviderError
	if !errors.As(err, &providerErr) {
		t.Fatalf("expected provider error, got %T", err)
	}
	if providerErr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d", providerErr.StatusCode)
	}
	if !providerErr.Retryable {
		t.Fatal("expected 429 to be retryable")
	}
}

func TestDoCachedJSONRequest_ClassifiesTimeoutAsRetryable(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			return nil, context.DeadlineExceeded
		}),
	}

	_, err := doCachedJSONRequest(context.Background(), client, nil, "tmdb", http.MethodGet, "https://example.com/test", nil, nil, time.Hour, 1)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !IsRetryableProviderError(err) {
		t.Fatalf("expected retryable provider error, got %v", err)
	}
}

func TestDoCachedJSONRequest_ClassifiesConnectionResetAsRetryable(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			return nil, &url.Error{
				Op:  "Get",
				URL: "https://example.com/test",
				Err: &os.SyscallError{Syscall: "read", Err: syscall.ECONNRESET},
			}
		}),
	}

	_, err := doCachedJSONRequest(context.Background(), client, nil, "tmdb", http.MethodGet, "https://example.com/test", nil, nil, time.Hour, 1)
	if err == nil {
		t.Fatal("expected transport error")
	}
	if !IsRetryableProviderError(err) {
		t.Fatalf("expected retryable provider error, got %v", err)
	}
}

func TestIsRetryableTransportError_RecognizesTemporaryNetError(t *testing.T) {
	err := &url.Error{
		Op:  "Get",
		URL: "https://example.com/test",
		Err: tempNetError{msg: "temporary DNS failure"},
	}
	if !isRetryableTransportError(err) {
		t.Fatalf("expected temporary net error to be retryable")
	}
}

func TestTMDBClientSearchMovie_ReturnsErrorForNon2xxResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "provider unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewTMDBClient("test-key")
	client.baseURL = server.URL

	results, err := client.SearchMovie(context.Background(), "Blade")
	if err == nil {
		t.Fatal("expected tmdb search error")
	}
	if len(results) != 0 {
		t.Fatalf("expected no results, got %#v", results)
	}
	if !IsRetryableProviderError(err) {
		t.Fatalf("expected retryable provider error, got %v", err)
	}
}

type inMemoryProviderCache struct {
	mu      sync.Mutex
	entries map[string]*ProviderCacheEntry
}

type roundTripFunc func(*http.Request) (*http.Response, error)

type tempNetError struct {
	msg string
}

func (e tempNetError) Error() string   { return e.msg }
func (e tempNetError) Timeout() bool   { return false }
func (e tempNetError) Temporary() bool { return true }

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func (c *inMemoryProviderCache) cacheKey(key ProviderCacheKey) string {
	return key.Provider + "|" + key.Method + "|" + key.URLPath + "|" + key.QueryHash + "|" + key.BodyHash
}

func (c *inMemoryProviderCache) Get(_ context.Context, key ProviderCacheKey, _ time.Time) (*ProviderCacheEntry, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[c.cacheKey(key)]
	if !ok {
		return nil, false, nil
	}
	cp := *entry
	return &cp, true, nil
}

func (c *inMemoryProviderCache) Put(_ context.Context, key ProviderCacheKey, entry ProviderCacheEntry, _ time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	cp := entry
	c.entries[c.cacheKey(key)] = &cp
	return nil
}
