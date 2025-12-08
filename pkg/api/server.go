package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"cache-chain/pkg/chain"
	"cache-chain/pkg/metrics"
)

// Server provides HTTP endpoints for cache inspection and monitoring.
type Server struct {
	chain   *chain.Chain
	metrics metrics.MetricsCollector
	server  *http.Server
	config  ServerConfig
}

// ServerConfig holds configuration for the API server.
type ServerConfig struct {
	// Address to listen on (e.g., ":8080")
	Address string

	// ReadTimeout for HTTP requests
	ReadTimeout time.Duration

	// WriteTimeout for HTTP responses
	WriteTimeout time.Duration

	// EnablePprof enables Go profiling endpoints at /debug/pprof/*
	EnablePprof bool
}

// DefaultServerConfig returns a default configuration.
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Address:      ":8080",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		EnablePprof:  false,
	}
}

// NewServer creates a new API server for cache inspection.
func NewServer(c *chain.Chain, metrics metrics.MetricsCollector, config ServerConfig) *Server {
	s := &Server{
		chain:   c,
		metrics: metrics,
		config:  config,
	}

	mux := http.NewServeMux()

	// Health and status endpoints
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/status", s.handleStatus)

	// Metrics endpoints
	mux.HandleFunc("/metrics", s.handleMetrics)
	mux.HandleFunc("/metrics/json", s.handleMetricsJSON)

	// Cache inspection endpoints
	mux.HandleFunc("/cache/get", s.handleCacheGet)
	mux.HandleFunc("/cache/stats", s.handleCacheStats)

	// Optional pprof endpoints
	if config.EnablePprof {
		mux.HandleFunc("/debug/pprof/", handlePprof)
	}

	s.server = &http.Server{
		Addr:         config.Address,
		Handler:      mux,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
	}

	return s
}

// Start starts the HTTP server in a goroutine.
func (s *Server) Start() error {
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("API server error: %v\n", err)
		}
	}()
	return nil
}

// Stop gracefully shuts down the HTTP server.
func (s *Server) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// handleHealth returns a simple health check.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
	}

	writeJSON(w, http.StatusOK, response)
}

// handleStatus returns detailed status information.
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := map[string]interface{}{
		"status":    "running",
		"timestamp": time.Now().Unix(),
		"uptime":    time.Since(startTime).String(),
	}

	writeJSON(w, http.StatusOK, response)
}

// handleMetrics returns metrics in Prometheus text format.
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// If metrics collector supports Prometheus format
	if pm, ok := s.metrics.(interface{ WritePrometheus(w http.ResponseWriter) }); ok {
		pm.WritePrometheus(w)
		return
	}

	// Fallback to simple text format
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "# Metrics collector does not support Prometheus format\n")
}

// handleMetricsJSON returns metrics in JSON format.
func (s *Server) handleMetricsJSON(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Try to get snapshot from memory collector
	if mc, ok := s.metrics.(interface{ Snapshot() interface{} }); ok {
		writeJSON(w, http.StatusOK, mc.Snapshot())
		return
	}

	// Fallback response
	response := map[string]interface{}{
		"error": "Metrics collector does not support JSON snapshot",
	}
	writeJSON(w, http.StatusOK, response)
}

// handleCacheGet retrieves a value from the cache (read-only).
func (s *Server) handleCacheGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	key := r.URL.Query().Get("key")
	if key == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error": "key parameter is required",
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	value, err := s.chain.Get(ctx, key)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{
			"error": err.Error(),
			"key":   key,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"key":   key,
		"value": value,
		"found": true,
	})
}

// handleCacheStats returns cache statistics.
func (s *Server) handleCacheStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := map[string]interface{}{
		"timestamp": time.Now().Unix(),
	}

	writeJSON(w, http.StatusOK, response)
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// handlePprof is a placeholder for pprof endpoints.
func handlePprof(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "pprof not yet implemented", http.StatusNotImplemented)
}

var startTime = time.Now()
