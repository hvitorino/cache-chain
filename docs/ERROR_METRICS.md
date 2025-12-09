# Cache Error Metrics - Differentiated by Type

## Overview

The cache-chain system now provides detailed error classification in Prometheus metrics, allowing precise identification of failure types across all cache layers.

## Error Types

### 1. **circuit_breaker_open**
- **Description**: Circuit breaker has opened due to consecutive failures
- **Cause**: High error rate exceeded threshold, circuit breaker protective mechanism activated
- **Action**: Wait for half-open state, investigate underlying cause
- **Severity**: 🔴 Critical (service degradation)

### 2. **timeout**
- **Description**: Operation exceeded configured timeout duration
- **Cause**: Slow backend response, network latency, resource contention
- **Action**: Review timeout configuration, check backend performance
- **Severity**: 🟡 Warning (intermittent failures)

### 3. **key_not_found**
- **Description**: Requested key does not exist in cache
- **Cause**: Cache miss, key expired, key never set
- **Action**: Normal operation for cache misses, check TTL configuration if excessive
- **Severity**: 🟢 Info (expected behavior)

### 4. **connection**
- **Description**: Network connection failure to backend (Redis, etc.)
- **Cause**: Backend down, network issues, DNS problems, firewall blocking
- **Action**: Check backend health, network connectivity, verify credentials
- **Severity**: 🔴 Critical (layer unavailable)

### 5. **serialization**
- **Description**: Failed to serialize/deserialize cache value
- **Cause**: Incompatible data type, encoding issues, corrupted data
- **Action**: Review data structures, check encoding format compatibility
- **Severity**: 🟡 Warning (data corruption possible)

### 6. **backend**
- **Description**: Backend-specific error (Redis command failed, etc.)
- **Cause**: Backend rejection, resource limits, protocol errors
- **Action**: Check backend logs, review resource limits (memory, connections)
- **Severity**: 🟡 Warning (backend issue)

### 7. **invalid_key**
- **Description**: Cache key validation failed
- **Cause**: Empty key, key too long, invalid characters
- **Action**: Review key generation logic
- **Severity**: 🟠 Error (application bug)

### 8. **invalid_value**
- **Description**: Cache value validation failed
- **Cause**: Nil value, unsupported type, value too large
- **Action**: Review value types and sizes
- **Severity**: 🟠 Error (application bug)

### 9. **unavailable**
- **Description**: Cache layer temporarily unavailable
- **Cause**: Maintenance, restarting, initializing
- **Action**: Wait for layer recovery, check deployment status
- **Severity**: 🟡 Warning (temporary)

### 10. **other**
- **Description**: Unclassified error type
- **Cause**: Unknown or rare error conditions
- **Action**: Check logs for detailed error messages
- **Severity**: 🟠 Error (requires investigation)

## Metric Structure

### Prometheus Metric
```
banking_api_cache_errors_total{layer="<layer>", operation="<operation>", error_type="<type>"}
```

**Labels:**
- `layer`: Cache layer name (L1-Memory, L2-Redis, PostgreSQL)
- `operation`: Cache operation (get, set, delete)
- `error_type`: Error classification (see types above)

### Example Queries

#### Error rate by type (last 5 minutes)
```promql
sum by(error_type) (rate(banking_api_cache_errors_total[5m]))
```

#### Circuit breaker errors by layer
```promql
rate(banking_api_cache_errors_total{error_type="circuit_breaker_open"}[1m])
```

#### Timeout errors percentage
```promql
sum(rate(banking_api_cache_errors_total{error_type="timeout"}[5m])) 
/ 
sum(rate(banking_api_cache_errors_total[5m])) * 100
```

#### Top 5 error types
```promql
topk(5, sum by(error_type) (rate(banking_api_cache_errors_total[5m])))
```

#### Errors by layer and type (heatmap data)
```promql
sum by(layer, error_type) (rate(banking_api_cache_errors_total[1m]))
```

## Grafana Dashboard

### New Panels Added

1. **Error Distribution by Type** (Pie Chart)
   - Shows percentage distribution of error types
   - Helps identify most common failure modes
   - Query: `sum by(error_type) (rate(banking_api_cache_errors_total[5m]))`

2. **Errors by Type and Layer** (Stacked Bar Chart)
   - Time-series view of errors stacked by type
   - Color-coded by error type
   - Shows which layers contribute to each error type
   - Query: `sum by(error_type, layer) (rate(banking_api_cache_errors_total[1m]))`

3. **Cache Errors by Type** (Time Series - Updated)
   - Original panel now includes error_type in legend
   - Legend format: `{{layer}} - {{operation}} - {{error_type}}`

### Dashboard Import

The updated dashboard is located at:
```
examples/banking-api/grafana-dashboard.json
```

To import:
1. Open Grafana UI (http://localhost:3000)
2. Go to Dashboards → Import
3. Upload `grafana-dashboard.json`
4. Select Prometheus datasource
5. Click Import

## Usage Examples

### Debugging Circuit Breaker Issues

When circuit breakers are opening frequently:

```bash
# View circuit breaker errors in logs with error type
docker logs banking-api 2>&1 | jq -r 'select(.level=="warn" and .msg | contains("circuit breaker")) | "\(.ts) [\(.logger)] \(.msg) - \(.error_type // "N/A")"'

# Check Grafana for error_type="circuit_breaker_open"
# Navigate to "Error Distribution by Type" panel
```

### Identifying Connection Issues

When experiencing backend connectivity problems:

```promql
# Connection error rate
rate(banking_api_cache_errors_total{error_type="connection"}[5m])

# Which layer has connection issues
sum by(layer) (rate(banking_api_cache_errors_total{error_type="connection"}[5m]))
```

### Monitoring Serialization Failures

Data corruption or encoding issues:

```promql
# Serialization error trend
rate(banking_api_cache_errors_total{error_type="serialization"}[5m])

# Alert if serialization errors exceed threshold
sum(rate(banking_api_cache_errors_total{error_type="serialization"}[5m])) > 0.1
```

## Alerting Rules

### Recommended Prometheus Alerts

```yaml
groups:
  - name: cache_errors
    interval: 30s
    rules:
      # Circuit breaker alerts
      - alert: HighCircuitBreakerErrors
        expr: |
          sum by(layer) (rate(banking_api_cache_errors_total{error_type="circuit_breaker_open"}[5m])) > 1
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "High circuit breaker errors on {{ $labels.layer }}"
          description: "Layer {{ $labels.layer }} has {{ $value }} circuit breaker errors/sec"

      # Connection alerts
      - alert: CacheConnectionFailures
        expr: |
          sum by(layer) (rate(banking_api_cache_errors_total{error_type="connection"}[5m])) > 0.5
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Connection failures on {{ $labels.layer }}"
          description: "{{ $labels.layer }} experiencing {{ $value }} connection errors/sec"

      # Timeout alerts
      - alert: HighTimeoutRate
        expr: |
          sum by(layer) (rate(banking_api_cache_errors_total{error_type="timeout"}[5m])) > 2
        for: 3m
        labels:
          severity: warning
        annotations:
          summary: "High timeout rate on {{ $labels.layer }}"
          description: "{{ $labels.layer }} has {{ $value }} timeouts/sec"

      # Serialization alerts
      - alert: SerializationErrors
        expr: |
          sum(rate(banking_api_cache_errors_total{error_type="serialization"}[5m])) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Serialization errors detected"
          description: "{{ $value }} serialization errors/sec - potential data corruption"

      # Overall error rate
      - alert: HighErrorRate
        expr: |
          (sum(rate(banking_api_cache_errors_total[5m])) / sum(rate(banking_api_cache_hits_total[5m]) + rate(banking_api_cache_misses_total[5m]))) * 100 > 5
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High cache error rate"
          description: "Overall cache error rate is {{ $value }}%"
```

## Implementation Details

### Error Classification Logic

The `cache.ClassifyError()` function in `pkg/cache/errors.go` uses the following priority:

1. **Exact match** with standard cache errors (ErrCircuitOpen, ErrTimeout, etc.)
2. **Error wrapping** check using `errors.Is()` for wrapped errors
3. **Pattern matching** on error message for common backend errors
4. **Fallback** to "other" for unrecognized errors

### Adding Custom Error Types

To add a new error type:

1. Define error constant in `pkg/cache/errors.go`:
```go
var ErrCustom = errors.New("cache: custom error")
```

2. Add case to `ClassifyError()`:
```go
case errors.Is(err, ErrCustom):
    return "custom"
```

3. Update documentation and Grafana dashboards

### Backward Compatibility

The new `RecordError()` method is additive:
- Existing `RecordSet()` and `RecordDelete()` continue to work
- Default error_type="other" for legacy compatibility
- Old metrics queries still function (ignoring error_type label)

## Performance Impact

- **Metric cardinality**: ~10 error types × 3 layers × 3 operations = ~90 time series
- **Memory overhead**: Negligible (~5KB additional metric storage)
- **CPU overhead**: Minimal (error classification is simple string matching)
- **No impact on happy path**: Classification only occurs on error conditions

## Best Practices

1. **Monitor error_type distribution**: Understand your failure modes
2. **Set up alerts per error type**: Different errors need different responses
3. **Correlate with logs**: Use error_type in log queries for faster troubleshooting
4. **Track trends over time**: Error pattern changes indicate systemic issues
5. **Document custom errors**: If adding new types, update this documentation

## Related Documentation

- [Logging Implementation](./LOGGING_IMPLEMENTATION.md)
- [Cache Error Logging Fix](./CACHE_ERROR_LOGGING_FIX.md)
- [Metrics & Observability](../rules/phase-6-metrics-observability.md)
- [Grafana Dashboard Guide](../examples/banking-api/README.md#monitoring)
