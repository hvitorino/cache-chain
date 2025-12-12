package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cache-chain/examples/banking-api/handlers"
	"cache-chain/examples/banking-api/postgres"
	"cache-chain/pkg/cache/memory"
	"cache-chain/pkg/cache/redis"
	"cache-chain/pkg/chain"
	"cache-chain/pkg/logging"
	promMetrics "cache-chain/pkg/metrics/prometheus"
	"cache-chain/pkg/resilience"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger from environment
	logger, err := logging.NewLoggerFromEnv()
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	// Set global logger
	logging.SetGlobal(logger)

	logger.Info("🚀 Starting Banking API with 3-Layer Cache...")

	// Layer 1: Memory Cache (fastest)
	memConfig := memory.MemoryCacheConfig{
		Name:            "L1-Memory",
		MaxSize:         1000,
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
		Logger:          logger,
	}
	memCache := memory.NewMemoryCache(memConfig)
	logger.Info("✓ Layer 1 (Memory) initialized")

	// Layer 2: Redis Cache (fast)
	redisConfig := redis.DefaultRedisCacheConfig()
	redisConfig.Name = "L2-Redis"
	redisConfig.Addr = getEnv("REDIS_ADDR", "localhost:6379")
	redisConfig.Logger = logger
	redisCache, err := redis.NewRedisCache(redisConfig)
	if err != nil {
		logger.Fatal("Failed to connect to Redis", zap.Error(err))
	}
	defer redisCache.Close()
	logger.Info("✓ Layer 2 (Redis) initialized")

	// Layer 3: PostgreSQL (slowest, persistent)
	pgConfig := postgres.Config{
		Host:     getEnv("POSTGRES_HOST", "localhost"),
		Port:     5432,
		User:     getEnv("POSTGRES_USER", "postgres"),
		Password: getEnv("POSTGRES_PASSWORD", "postgres"),
		Database: getEnv("POSTGRES_DB", "banking_db"),
		SSLMode:  "disable",
	}
	pgAdapter, err := postgres.NewPostgresAdapter(pgConfig)
	if err != nil {
		logger.Fatal("Failed to connect to PostgreSQL", zap.Error(err))
	}
	defer pgAdapter.Close()
	logger.Info("✓ Layer 3 (PostgreSQL) initialized")

	// Setup Prometheus metrics collector
	metricsCollector := promMetrics.NewPrometheusCollector("banking_api")

	// Register cache metrics with Prometheus
	// We need to register each collector individually with the default registry
	prometheus.MustRegister(metricsCollector)
	logger.Info("✓ Prometheus metrics initialized and registered")

	// Create 3-layer cache chain with metrics: Memory -> Redis -> PostgreSQL
	cacheChain, err := chain.NewWithConfig(
		chain.ChainConfig{
			ResilientConfigs: []resilience.ResilientConfig{
				// L1 (Memory): Super tolerant - almost never opens
				{
					Timeout: 2 * time.Second,
					CircuitBreakerConfig: resilience.CircuitBreakerConfig{
						MaxRequests: 10,
						Interval:    30 * time.Second,
						Timeout:     60 * time.Second,
						ReadyToTrip: func(counts resilience.Counts) bool {
							if counts.Requests < 100 {
								return false
							}
							failureRate := float64(counts.TotalFailures) / float64(counts.Requests)
							return failureRate >= 0.15
						},
					},
				},
				// L2 (Redis): Very tolerant
				{
					Timeout: 3 * time.Second,
					CircuitBreakerConfig: resilience.CircuitBreakerConfig{
						MaxRequests: 10,
						Interval:    45 * time.Second,
						Timeout:     90 * time.Second,
						ReadyToTrip: func(counts resilience.Counts) bool {
							if counts.Requests < 50 {
								return false
							}
							failureRate := float64(counts.TotalFailures) / float64(counts.Requests)
							return failureRate >= 0.7
						},
					},
				},
				// L3 (PostgreSQL): Extremely tolerant - last resort
				{
					Timeout: 10 * time.Second,
					CircuitBreakerConfig: resilience.CircuitBreakerConfig{
						MaxRequests: 15,
						Interval:    60 * time.Second,
						Timeout:     120 * time.Second,
						ReadyToTrip: func(counts resilience.Counts) bool {
							if counts.Requests < 30 {
								return false
							}
							failureRate := float64(counts.TotalFailures) / float64(counts.Requests)
							return failureRate >= 0.15
						},
					},
				},
			},
			Metrics: metricsCollector,
			Logger:  logger,
		},
		memCache, redisCache, pgAdapter,
	)
	if err != nil {
		logger.Fatal("Failed to create cache chain", zap.Error(err))
	}
	logger.Info("✓ 3-Layer cache chain created with metrics")
	logger.Info("  Cache flow: Memory (L1) → Redis (L2) → PostgreSQL (L3)")

	// Setup handlers
	handler := handlers.NewTransactionHandler(pgAdapter, cacheChain)

	// Setup router
	r := mux.NewRouter()

	// Wrap router with metrics middleware
	r.Use(prometheusMiddleware())

	r.HandleFunc("/health", handler.HealthCheck).Methods("GET")
	r.HandleFunc("/transactions", handler.CreateTransaction).Methods("POST")
	r.HandleFunc("/transactions/{id}", handler.GetTransaction).Methods("GET")
	r.Handle("/metrics", promhttp.Handler()).Methods("GET")

	// Setup server
	port := getEnv("PORT", "8080")
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in background
	go func() {
		logger.Info("🌐 Server listening", zap.String("port", port))
		logger.Info("\n📚 Available endpoints:")
		logger.Info("  POST   /transactions       - Create new transaction")
		logger.Info("  GET    /transactions/{id}  - Get transaction by ID")
		logger.Info("  GET    /health             - Health check")
		logger.Info("  GET    /metrics            - Prometheus metrics")
		logger.Info("")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server failed", zap.Error(err))
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("\n🛑 Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("Server shutdown error", zap.Error(err))
	}

	logger.Info("✓ Server stopped gracefully")
}

// HTTP metrics
var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "api_http_request_duration_seconds",
			Help:    "HTTP request latencies in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)
)

func init() {
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(httpRequestDuration)
}

// prometheusMiddleware wraps HTTP handlers to collect metrics
func prometheusMiddleware() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Capture status code
			srw := &statusResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// Call next handler
			next.ServeHTTP(srw, r)

			// Record metrics
			duration := time.Since(start).Seconds()
			endpoint := getEndpoint(r)

			httpRequestsTotal.WithLabelValues(
				r.Method,
				endpoint,
				http.StatusText(srw.statusCode),
			).Inc()

			httpRequestDuration.WithLabelValues(
				r.Method,
				endpoint,
			).Observe(duration)
		})
	}
}

// statusResponseWriter captures the status code
type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// getEndpoint returns a normalized endpoint path for metrics
func getEndpoint(r *http.Request) string {
	route := mux.CurrentRoute(r)
	if route == nil {
		return r.URL.Path
	}

	pathTemplate, err := route.GetPathTemplate()
	if err != nil {
		return r.URL.Path
	}

	return pathTemplate
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
