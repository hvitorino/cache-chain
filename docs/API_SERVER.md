# HTTP API Server

The HTTP API Server is an optional feature that provides REST endpoints for cache inspection, monitoring, and observability. It's designed to be lightweight and non-intrusive, enabling external monitoring without requiring custom integration code.

## Features

- **Health Checks**: Simple endpoint for load balancers and monitoring systems
- **Status Monitoring**: Detailed server status with uptime
- **Metrics Export**: Prometheus-compatible metrics in text or JSON format
- **Cache Inspection**: Read-only cache queries
- **Statistics**: Cache statistics and performance data
- **Graceful Shutdown**: Proper cleanup with configurable timeout

## Quick Start

```go
package main

import (
    "context"
    "log"
    "time"

    "cache-chain/pkg/api"
    "cache-chain/pkg/cache/memory"
    "cache-chain/pkg/chain"
    "cache-chain/pkg/metrics/memory"
)

func main() {
    // Create cache chain
    l1 := memory.NewMemoryCache(memory.MemoryCacheConfig{
        Name:    "L1",
        MaxSize: 100,
    })

    c, err := chain.New(l1)
    if err != nil {
        log.Fatal(err)
    }
    defer c.Close()

    // Create metrics collector
    metrics := memorycollector.NewMemoryCollector()

    // Configure and start server
    config := api.DefaultServerConfig()
    config.Address = ":8080"

    server := api.NewServer(c, metrics, config)
    
    if err := server.Start(); err != nil {
        log.Fatal(err)
    }

    // Your application logic here...

    // Graceful shutdown
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    server.Stop(ctx)
}
```

## Configuration

### ServerConfig

```go
type ServerConfig struct {
    // Address to listen on (default: ":8080")
    Address string

    // Read timeout for requests (default: 5s)
    ReadTimeout time.Duration

    // Write timeout for responses (default: 10s)
    WriteTimeout time.Duration

    // Enable pprof endpoints (default: false)
    EnablePprof bool
}
```

### Default Configuration

```go
config := api.DefaultServerConfig()
// Returns:
// Address:      ":8080"
// ReadTimeout:  5 * time.Second
// WriteTimeout: 10 * time.Second
// EnablePprof:  false
```

### Custom Configuration

```go
config := api.ServerConfig{
    Address:      ":9090",
    ReadTimeout:  3 * time.Second,
    WriteTimeout: 15 * time.Second,
    EnablePprof:  true,
}

server := api.NewServer(chain, metrics, config)
```

## API Endpoints

### GET /health

Health check endpoint for load balancers and monitoring systems.

**Response:**
```json
{
  "status": "healthy",
  "timestamp": 1704067200
}
```

**Status Codes:**
- `200 OK`: Server is healthy

**Example:**
```bash
curl http://localhost:8080/health
```

---

### GET /status

Detailed server status with uptime information.

**Response:**
```json
{
  "status": "running",
  "timestamp": 1704067200,
  "uptime": "2h15m30s"
}
```

**Status Codes:**
- `200 OK`: Server is running

**Example:**
```bash
curl http://localhost:8080/status
```

---

### GET /metrics

Prometheus-compatible metrics in text format (if supported by collector).

**Response:** (Prometheus text format)
```
# HELP cache_hits_total Total number of cache hits
# TYPE cache_hits_total counter
cache_hits_total{layer="L1"} 150
cache_hits_total{layer="L2"} 75
...
```

**Status Codes:**
- `200 OK`: Metrics available
- `501 Not Implemented`: Collector doesn't support Prometheus export

**Example:**
```bash
curl http://localhost:8080/metrics
```

---

### GET /metrics/json

Metrics snapshot in JSON format.

**Response:**
```json
{
  "hits": 225,
  "misses": 50,
  "hit_ratio": 0.82,
  "timestamp": 1704067200
}
```

**Status Codes:**
- `200 OK`: Metrics snapshot available
- `501 Not Implemented`: Collector doesn't support snapshots

**Example:**
```bash
curl http://localhost:8080/metrics/json
```

---

### GET /cache/get

Read-only cache lookup by key.

**Query Parameters:**
- `key` (required): The cache key to retrieve

**Response (Success):**
```json
{
  "key": "user:123",
  "value": "Alice",
  "found": true
}
```

**Response (Not Found):**
```json
{
  "key": "user:999",
  "found": false
}
```

**Status Codes:**
- `200 OK`: Key found in cache
- `400 Bad Request`: Missing key parameter
- `404 Not Found`: Key not found in cache
- `500 Internal Server Error`: Cache error

**Examples:**
```bash
# Successful lookup
curl "http://localhost:8080/cache/get?key=user:123"

# Key not found
curl "http://localhost:8080/cache/get?key=nonexistent"

# Missing key parameter
curl "http://localhost:8080/cache/get"
```

---

### GET /cache/stats

Cache statistics and performance data.

**Response:**
```json
{
  "timestamp": 1704067200
}
```

**Status Codes:**
- `200 OK`: Statistics available

**Example:**
```bash
curl http://localhost:8080/cache/stats
```

## Server Lifecycle

### Starting the Server

```go
server := api.NewServer(chain, metrics, config)

if err := server.Start(); err != nil {
    log.Fatalf("Failed to start server: %v", err)
}
```

The `Start()` method:
- Starts the HTTP server in a background goroutine
- Returns immediately after starting
- Returns error if server fails to start

### Stopping the Server

```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

if err := server.Stop(ctx); err != nil {
    log.Printf("Error stopping server: %v", err)
}
```

The `Stop()` method:
- Initiates graceful shutdown
- Waits for active requests to complete
- Respects the context timeout
- Returns error if shutdown times out

### Signal Handling

```go
sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

// Block until signal received
<-sigCh

// Graceful shutdown
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

server.Stop(ctx)
```

## Security Considerations

### Read-Only Operations

The `/cache/get` endpoint is **read-only** by design. It does not support:
- Cache writes
- Cache deletions
- Cache invalidation
- Configuration changes

This prevents accidental or malicious cache manipulation via the API.

### Network Security

The server binds to the configured address without TLS/HTTPS. For production use:

1. **Use a reverse proxy** (nginx, Traefik) for TLS termination
2. **Bind to localhost** (`:127.0.0.1:8080`) and use port forwarding
3. **Use firewall rules** to restrict access to monitoring systems only

Example with reverse proxy:
```go
// Bind only to localhost
config := api.DefaultServerConfig()
config.Address = "127.0.0.1:8080"
```

### Pprof Endpoints

When `EnablePprof` is true, the following endpoints are available:
- `/debug/pprof/`
- `/debug/pprof/cmdline`
- `/debug/pprof/profile`
- `/debug/pprof/symbol`
- `/debug/pprof/trace`

**Warning**: Pprof endpoints expose internal runtime data. Only enable in development or secure internal networks.

## Integration Examples

### With Prometheus

```go
import (
    "cache-chain/pkg/api"
    promcollector "cache-chain/pkg/metrics/prometheus"
)

// Use Prometheus metrics collector
registry := prometheus.NewRegistry()
metrics := promcollector.NewPrometheusCollector(registry)

// Start API server
server := api.NewServer(chain, metrics, api.DefaultServerConfig())
server.Start()

// Prometheus can now scrape http://localhost:8080/metrics
```

### With Health Check Service

```bash
#!/bin/bash
# health-check.sh

response=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/health)

if [ "$response" -eq 200 ]; then
    echo "Service is healthy"
    exit 0
else
    echo "Service is unhealthy"
    exit 1
fi
```

### With Load Balancer

Configure your load balancer to use `/health` endpoint:

```yaml
# Example: Kubernetes liveness probe
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 30

readinessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10
```

### With Grafana Dashboard

Query the `/metrics/json` endpoint to build custom dashboards:

```javascript
// Grafana JSON datasource query
{
  "target": "cache_metrics",
  "endpoint": "http://localhost:8080/metrics/json"
}
```

## Performance

The API server is designed to have minimal impact on cache performance:

- **Non-blocking**: Runs in a separate goroutine
- **Timeouts**: Configurable read/write timeouts prevent hung connections
- **Read-only**: Cache queries don't modify state or trigger warm-ups
- **Lightweight**: No external dependencies beyond standard library

Typical latencies (localhost):
- `/health`: < 1ms
- `/status`: < 1ms
- `/cache/get`: < 5ms (depends on cache layer)
- `/metrics/json`: < 10ms (depends on collector)

## Testing

The API server includes comprehensive tests covering all endpoints:

```bash
# Run API tests
go test ./pkg/api/... -v

# Run with coverage
go test ./pkg/api/... -cover
```

Test coverage: **67.6%** (all critical paths covered)

## Example Application

See `examples/api/main.go` for a complete working example:

```bash
# Run the example
go run examples/api/main.go

# In another terminal, test the endpoints
curl http://localhost:8080/health
curl http://localhost:8080/status
curl "http://localhost:8080/cache/get?key=user:123"
curl http://localhost:8080/metrics/json
```

## Troubleshooting

### Server Won't Start

**Error**: `listen tcp :8080: bind: address already in use`

**Solution**: Change the port or stop the conflicting process:
```go
config := api.DefaultServerConfig()
config.Address = ":9090"  // Use different port
```

### Context Deadline Exceeded

**Error**: `context deadline exceeded` during shutdown

**Solution**: Increase shutdown timeout:
```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
server.Stop(ctx)
```

### Empty Metrics Response

**Issue**: `/metrics/json` returns empty or minimal data

**Cause**: Using `NoOpCollector` which doesn't collect metrics

**Solution**: Use `MemoryCollector` or `PrometheusCollector`:
```go
import memorycollector "cache-chain/pkg/metrics/memory"

metrics := memorycollector.NewMemoryCollector()
server := api.NewServer(chain, metrics, config)
```

## Best Practices

1. **Always use timeouts** for graceful shutdown
2. **Bind to localhost** in production environments
3. **Disable pprof** unless debugging in secure networks
4. **Monitor `/health`** with your infrastructure
5. **Use Prometheus** for production metrics collection
6. **Test endpoint responses** in your CI/CD pipeline
7. **Document custom endpoints** if you extend the API

## Future Enhancements

Potential features for future releases:
- Write operations (`/cache/set`, `/cache/delete`) with authentication
- Layer-specific inspection (`/cache/layer/{name}`)
- CORS support for web-based dashboards
- Authentication/authorization middleware
- Rate limiting per endpoint
- Custom middleware support
- WebSocket support for real-time metrics streaming

## See Also

- [Metrics and Observability](METRICS.md)
- [TTL Integration](TTL_INTEGRATION.md)
- [Circuit Breaker and Timeout](CIRCUIT_BREAKER_TIMEOUT.md)
