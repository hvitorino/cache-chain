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

	"github.com/gorilla/mux"
)

func main() {
	log.Println("🚀 Starting Banking API with 3-Layer Cache...")

	// Layer 1: Memory Cache (fastest)
	memConfig := memory.MemoryCacheConfig{
		Name:            "L1-Memory",
		MaxSize:         1000,
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	}
	memCache := memory.NewMemoryCache(memConfig)
	log.Println("✓ Layer 1 (Memory) initialized")

	// Layer 2: Redis Cache (fast)
	redisConfig := redis.DefaultRedisCacheConfig()
	redisConfig.Name = "L2-Redis"
	redisConfig.Addr = getEnv("REDIS_ADDR", "localhost:6379")
	redisCache, err := redis.NewRedisCache(redisConfig)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisCache.Close()
	log.Println("✓ Layer 2 (Redis) initialized")

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
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer pgAdapter.Close()
	log.Println("✓ Layer 3 (PostgreSQL) initialized")

	// Create 3-layer cache chain: Memory -> Redis -> PostgreSQL
	cacheChain, err := chain.New(memCache, redisCache, pgAdapter)
	if err != nil {
		log.Fatalf("Failed to create cache chain: %v", err)
	}
	log.Println("✓ 3-Layer cache chain created")
	log.Println("  Cache flow: Memory (L1) → Redis (L2) → PostgreSQL (L3)")

	// Setup handlers
	handler := handlers.NewTransactionHandler(pgAdapter, cacheChain)

	// Setup router
	r := mux.NewRouter()
	r.HandleFunc("/health", handler.HealthCheck).Methods("GET")
	r.HandleFunc("/transactions", handler.CreateTransaction).Methods("POST")
	r.HandleFunc("/transactions", handler.ListTransactions).Methods("GET")
	r.HandleFunc("/transactions/{id}", handler.GetTransaction).Methods("GET")

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
		log.Printf("🌐 Server listening on port %s", port)
		log.Println("\n📚 Available endpoints:")
		log.Println("  POST   /transactions              - Create new transaction")
		log.Println("  GET    /transactions?account_id=X - List account transactions")
		log.Println("  GET    /transactions/{id}         - Get transaction by ID")
		log.Println("  GET    /health                    - Health check")
		log.Println()
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("\n🛑 Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("✓ Server stopped gracefully")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
