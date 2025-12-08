package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cache-chain/pkg/api"
	"cache-chain/pkg/cache/memory"
	"cache-chain/pkg/chain"
	memorycollector "cache-chain/pkg/metrics/memory"
)

func main() {
	fmt.Println("=== Cache Chain HTTP API Demo ===")
	fmt.Println()

	// 1. Create cache layers
	l1 := memory.NewMemoryCache(memory.MemoryCacheConfig{
		Name:    "L1-Memory",
		MaxSize: 100,
	})

	l2 := memory.NewMemoryCache(memory.MemoryCacheConfig{
		Name:    "L2-Redis",
		MaxSize: 1000,
	})

	// 2. Create chain with metrics
	c, err := chain.New(l1, l2)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	metrics := memorycollector.NewMemoryCollector()

	// 3. Populate some data
	ctx := context.Background()
	c.Set(ctx, "user:123", "Alice", time.Hour)
	c.Set(ctx, "user:456", "Bob", time.Hour)
	c.Set(ctx, "product:789", "Widget", time.Hour)

	fmt.Println("✓ Created cache chain with sample data")
	fmt.Println("  - user:123 = Alice")
	fmt.Println("  - user:456 = Bob")
	fmt.Println("  - product:789 = Widget")
	fmt.Println()

	// 4. Configure and start API server
	config := api.DefaultServerConfig()
	config.Address = ":8080"

	server := api.NewServer(c, metrics, config)

	if err := server.Start(); err != nil {
		log.Fatal(err)
	}

	fmt.Println("🚀 HTTP API Server started on http://localhost:8080")
	fmt.Println()
	fmt.Println("Available endpoints:")
	fmt.Println("  GET /health              - Health check")
	fmt.Println("  GET /status              - Server status")
	fmt.Println("  GET /metrics             - Prometheus metrics")
	fmt.Println("  GET /metrics/json        - JSON metrics snapshot")
	fmt.Println("  GET /cache/get?key=...   - Get cache value")
	fmt.Println("  GET /cache/stats         - Cache statistics")
	fmt.Println()
	fmt.Println("Try these commands:")
	fmt.Println("  curl http://localhost:8080/health")
	fmt.Println("  curl http://localhost:8080/status")
	fmt.Println("  curl http://localhost:8080/cache/get?key=user:123")
	fmt.Println("  curl http://localhost:8080/metrics/json")
	fmt.Println()
	fmt.Println("Press Ctrl+C to stop...")

	// 5. Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\n\nShutting down gracefully...")

	// 6. Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Stop(shutdownCtx); err != nil {
		log.Printf("Error stopping server: %v", err)
	}

	fmt.Println("✓ Server stopped")
}
