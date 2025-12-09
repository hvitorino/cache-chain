# Banking API - Monitoring Dashboard

Complete monitoring solution for the Banking API with Prometheus and Grafana.

## 🎯 Features

- **Real-time Metrics**: Monitor API performance during load tests
- **Cache Performance**: Track hit rates across all 3 layers (Memory, Redis, PostgreSQL)
- **Latency Tracking**: P50, P95, P99 latencies for all operations
- **Error Monitoring**: Track errors and failures across the stack
- **Resource Usage**: Monitor Redis, PostgreSQL, and system resources

## 🚀 Quick Start

### Option 1: Using docker-compose directly

```bash
# Start all services (API, PostgreSQL, Redis, Prometheus, Grafana)
docker-compose up -d

# Check services are running
docker-compose ps

# View logs
docker-compose logs -f
```

All services start automatically, including the monitoring stack!

### Option 2: Using the helper script

```bash
./dashboard.sh
```

This does the same as Option 1, but also:
- Waits for all services to be healthy
- Displays all service URLs nicely formatted

Open [http://localhost:3000](http://localhost:3000) in your browser.

- **Username**: `admin`
- **Password**: `admin`

### Access Grafana

Open [http://localhost:3000](http://localhost:3000) in your browser.

- **Username**: `admin`
- **Password**: `admin`

The dashboard "Banking API - Cache Chain Performance" is automatically provisioned.

### Run Load Test

```bash
./loadtest.sh
```

Watch the metrics update in real-time in Grafana!

## 📊 Dashboard Panels

### Top Row: Request Metrics
- **Request Rate**: Requests per second (total, success, errors)
- **Response Time**: P50, P95, P99 latencies

### Middle Row: Cache Performance
- **Cache Hit Rate by Layer**: L1 (Memory), L2 (Redis), L3 (PostgreSQL)
- **Cache Operations Rate**: Hits and misses per layer

### Bottom Row: System Metrics
- **Cache Layer Latency**: Operation latencies per layer
- **Error Rate**: HTTP 5xx and cache errors
- **Resource Usage**: Memory cache size, Redis ops, PostgreSQL connections

### Stats: Key Metrics
- **Overall Cache Efficiency**: Global hit rate percentage
- **Current RPS**: Real-time requests per second
- **Avg Response Time**: Average API response time
- **Error Rate**: Percentage of failed requests

## 📈 Metrics Available

### API Metrics
- `api_http_requests_total` - Total HTTP requests by method, endpoint, status
- `api_http_request_duration_seconds` - Request duration histogram

### Cache Metrics
- `banking_api_cache_hits_total` - Cache hits by layer
- `banking_api_cache_misses_total` - Cache misses by layer
- `banking_api_cache_operation_duration_seconds` - Operation latencies

### Infrastructure Metrics
- Redis metrics via `redis-exporter`
- PostgreSQL metrics via `postgres-exporter`

## 🔍 Monitoring Endpoints

| Service | URL | Description |
|---------|-----|-------------|
| API | http://localhost:8080 | Banking API |
| Metrics | http://localhost:8080/metrics | Prometheus metrics endpoint |
| Grafana | http://localhost:3000 | Monitoring dashboard |
| Prometheus | http://localhost:9090 | Prometheus UI |

## 🛠️ Useful Commands

### View Logs
```bash
# API logs
docker-compose logs -f api

# All services
docker-compose logs -f

# Prometheus logs
docker-compose logs -f prometheus

# Grafana logs
docker-compose logs -f grafana
```

### Restart Services
```bash
# Restart all
docker-compose restart

# Restart specific service
docker-compose restart api
```

### Stop Services
```bash
docker-compose down

# Stop and remove volumes (clean state)
docker-compose down -v
```

### Check Service Health
```bash
# API
curl http://localhost:8080/health

# Prometheus
curl http://localhost:9090/-/healthy

# Grafana
curl http://localhost:3000/api/health
```

## 🧪 Load Testing with Monitoring

1. **Start all services**:
   ```bash
   docker-compose up -d
   # or use: ./dashboard.sh
   ```

2. **Open Grafana** at http://localhost:3000

3. **Run load test**:
   ```bash
   # Basic test (30s, 100 connections)
   ./loadtest.sh
   
   # Heavy test (60s, 200 connections)
   THREADS=8 CONNECTIONS=200 DURATION=60s ./loadtest.sh
   ```

4. **Watch metrics** in real-time:
   - Cache hit rates increasing as cache warms up
   - Response times improving with cache hits
   - Layer-specific performance (L1 << L2 << L3)

## 📝 Interpreting Results

### Good Performance Indicators
- ✅ Cache hit rate > 80% (after warmup)
- ✅ P95 latency < 50ms for cache hits
- ✅ Error rate < 1%
- ✅ L1 (Memory) hit rate > 60%

### Performance Issues to Watch
- ⚠️ High L3 (PostgreSQL) hit rate → L1/L2 cache too small
- ⚠️ Increasing error rate → System overload
- ⚠️ Rising P99 latency → Capacity issues
- ⚠️ Low overall cache hit rate → Cache not warming up properly

## 🏗️ Architecture

```
┌─────────────┐
│   wrk       │ Load Testing Tool
│  (Client)   │
└─────┬───────┘
      │
      ▼
┌─────────────────────────────────────┐
│   Banking API (Port 8080)           │
│   ┌─────────────────────────────┐   │
│   │  Prometheus Middleware      │   │
│   │  (Metrics Collection)       │   │
│   └─────────────────────────────┘   │
│   ┌─────────────────────────────┐   │
│   │  3-Layer Cache Chain        │   │
│   │  L1: Memory                 │   │
│   │  L2: Redis                  │   │
│   │  L3: PostgreSQL             │   │
│   └─────────────────────────────┘   │
└──────────┬──────────────────────────┘
           │ /metrics
           ▼
┌─────────────────────┐
│   Prometheus        │
│   (Port 9090)       │
│   - Scrapes metrics │
│   - Stores TSDB     │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│   Grafana           │
│   (Port 3000)       │
│   - Visualizations  │
│   - Dashboards      │
└─────────────────────┘
```

## 🎨 Customizing the Dashboard

The dashboard is auto-provisioned from `grafana-dashboard.json`. To customize:

1. Edit the dashboard in Grafana UI
2. Export it via "Share" → "Export" → "Save to file"
3. Replace `grafana-dashboard.json` with the new version
4. Restart: `docker-compose restart grafana`

## 🐛 Troubleshooting

### Grafana shows "No data"
- Check Prometheus is scraping: http://localhost:9090/targets
- Verify API is exposing metrics: http://localhost:8080/metrics
- Check Prometheus data source in Grafana settings

### Services not starting
```bash
# Check logs
docker-compose logs

# Recreate containers
docker-compose down
docker-compose up -d
```

### Metrics not updating
```bash
# Restart Prometheus
docker-compose restart prometheus

# Check API logs
docker-compose logs -f api
```

## 📚 Further Reading

- [Prometheus Documentation](https://prometheus.io/docs/)
- [Grafana Documentation](https://grafana.com/docs/)
- [Cache Chain Architecture](../../README.md)
