package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cache-chain/pkg/cache/memory"
	"cache-chain/pkg/chain"
	memorycollector "cache-chain/pkg/metrics/memory"
)

func setupTestServer(t *testing.T) (*Server, *chain.Chain) {
	l1 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L1", MaxSize: 100})
	l2 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L2", MaxSize: 100})

	c, err := chain.New(l1, l2)
	if err != nil {
		t.Fatalf("Failed to create chain: %v", err)
	}

	metrics := memorycollector.NewMemoryCollector()
	config := DefaultServerConfig()
	server := NewServer(c, metrics, config)

	return server, c
}

func TestServer_Health(t *testing.T) {
	server, c := setupTestServer(t)
	defer c.Close()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	server.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.NewDecoder(w.Body).Decode(&response)

	if response["status"] != "healthy" {
		t.Errorf("Expected status healthy, got %v", response["status"])
	}
}

func TestServer_Status(t *testing.T) {
	server, c := setupTestServer(t)
	defer c.Close()

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	w := httptest.NewRecorder()

	server.handleStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.NewDecoder(w.Body).Decode(&response)

	if response["status"] != "running" {
		t.Errorf("Expected status running, got %v", response["status"])
	}
}

func TestServer_CacheGet_Success(t *testing.T) {
	server, c := setupTestServer(t)
	defer c.Close()

	// Set a value in cache
	ctx := context.Background()
	c.Set(ctx, "test-key", "test-value", time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/cache/get?key=test-key", nil)
	w := httptest.NewRecorder()

	server.handleCacheGet(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.NewDecoder(w.Body).Decode(&response)

	if response["key"] != "test-key" {
		t.Errorf("Expected key test-key, got %v", response["key"])
	}
	if response["value"] != "test-value" {
		t.Errorf("Expected value test-value, got %v", response["value"])
	}
	if response["found"] != true {
		t.Errorf("Expected found true, got %v", response["found"])
	}
}

func TestServer_CacheGet_NotFound(t *testing.T) {
	server, c := setupTestServer(t)
	defer c.Close()

	req := httptest.NewRequest(http.MethodGet, "/cache/get?key=missing-key", nil)
	w := httptest.NewRecorder()

	server.handleCacheGet(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}

	var response map[string]interface{}
	json.NewDecoder(w.Body).Decode(&response)

	if response["key"] != "missing-key" {
		t.Errorf("Expected key missing-key, got %v", response["key"])
	}
	if response["error"] == nil {
		t.Error("Expected error field")
	}
}

func TestServer_CacheGet_MissingKey(t *testing.T) {
	server, c := setupTestServer(t)
	defer c.Close()

	req := httptest.NewRequest(http.MethodGet, "/cache/get", nil)
	w := httptest.NewRecorder()

	server.handleCacheGet(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var response map[string]interface{}
	json.NewDecoder(w.Body).Decode(&response)

	if response["error"] != "key parameter is required" {
		t.Errorf("Unexpected error message: %v", response["error"])
	}
}

func TestServer_MetricsJSON(t *testing.T) {
	server, c := setupTestServer(t)
	defer c.Close()

	// Generate some metrics
	ctx := context.Background()
	c.Set(ctx, "key1", "value1", time.Hour)
	c.Get(ctx, "key1")

	req := httptest.NewRequest(http.MethodGet, "/metrics/json", nil)
	w := httptest.NewRecorder()

	server.handleMetricsJSON(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.NewDecoder(w.Body).Decode(&response)

	// Memory collector should return snapshot
	if response == nil {
		t.Error("Expected non-nil response")
	}
}

func TestServer_CacheStats(t *testing.T) {
	server, c := setupTestServer(t)
	defer c.Close()

	req := httptest.NewRequest(http.MethodGet, "/cache/stats", nil)
	w := httptest.NewRecorder()

	server.handleCacheStats(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.NewDecoder(w.Body).Decode(&response)

	if response["timestamp"] == nil {
		t.Error("Expected timestamp in response")
	}
}

func TestServer_MethodNotAllowed(t *testing.T) {
	server, c := setupTestServer(t)
	defer c.Close()

	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	w := httptest.NewRecorder()

	server.handleHealth(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestServer_StartStop(t *testing.T) {
	server, c := setupTestServer(t)
	defer c.Close()

	// Start server
	err := server.Start()
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Give it time to start
	time.Sleep(50 * time.Millisecond)

	// Stop server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = server.Stop(ctx)
	if err != nil {
		t.Errorf("Failed to stop server: %v", err)
	}
}

func TestDefaultServerConfig(t *testing.T) {
	config := DefaultServerConfig()

	if config.Address != ":8080" {
		t.Errorf("Expected address :8080, got %s", config.Address)
	}
	if config.ReadTimeout != 5*time.Second {
		t.Errorf("Expected read timeout 5s, got %v", config.ReadTimeout)
	}
	if config.WriteTimeout != 10*time.Second {
		t.Errorf("Expected write timeout 10s, got %v", config.WriteTimeout)
	}
	if config.EnablePprof {
		t.Error("Expected pprof disabled by default")
	}
}
