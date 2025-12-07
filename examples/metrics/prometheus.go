package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"cache-chain/pkg/cache/memory"
	"cache-chain/pkg/chain"
	"cache-chain/pkg/metrics/prometheus"

	promhttp "github.com/prometheus/client_golang/prometheus/promhttp"
	promRegistry "github.com/prometheus/client_golang/prometheus"
)

func main() {
	// Create Prometheus collector
	collector := prometheus.NewPrometheusCollector("cache_chain")

	// Create Prometheus registry
	registry := promRegistry.NewRegistry()
	if err := collector.Register(registry); err != nil {
		log.Fatalf("Failed to register metrics: %v", err)
	}

	// Create cache layers
	l1 := memory.NewMemoryCache(memory.MemoryCacheConfig{
		Name:    "L1-Memory",
		MaxSize: 1000,
	})

	l2 := memory.NewMemoryCache(memory.MemoryCacheConfig{
		Name:    "L2-Memory",
		MaxSize: 10000,
	})

	// Create chain with Prometheus metrics
	cacheChain, err := chain.NewWithConfig(chain.ChainConfig{
		Metrics: collector,
	}, l1, l2)
	if err != nil {
		log.Fatalf("Failed to create chain: %v", err)
	}
	defer cacheChain.Close()

	// Start metrics server
	go func() {
		http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
		fmt.Println("Metrics server listening on :9090")
		fmt.Println("Visit http://localhost:9090/metrics to see metrics")
		if err := http.ListenAndServe(":9090", nil); err != nil {
			log.Fatalf("Failed to start metrics server: %v", err)
		}
	}()

	ctx := context.Background()

	// Simulate cache operations
	fmt.Println("\nSimulating cache operations...")

	// Set some values
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := fmt.Sprintf("value-%d", i)
		cacheChain.Set(ctx, key, value, time.Hour)
	}

	// Generate cache hits
	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("key-%d", i)
		cacheChain.Get(ctx, key)
	}

	fmt.Println("\nMetrics available at http://localhost:9090/metrics")
	fmt.Println("Press Ctrl+C to exit...")

	// Keep running
	select {}
}
