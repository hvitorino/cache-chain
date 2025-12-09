# Cache Errors - Log Visibility Issue Fixed

## Problema Identificado

Os erros de cache estavam sendo registrados nas **métricas do Prometheus** mas **não apareciam nos logs** porque:

### 1. **ResilientLayer silenciava erros**
```go
// ANTES - Erros não eram logados
if err != nil {
    if err == gobreaker.ErrOpenState {
        return cache.ErrCircuitOpen  // Sem log
    }
    if ctx.Err() == context.DeadlineExceeded {
        return cache.ErrTimeout  // Sem log
    }
    return err  // ❌ Erro retornado mas não logado!
}
```

### 2. **Chain silenciava erros de fallback**
```go
// ANTES - Erros de layers eram silenciados
value, err := layer.Get(ctx, key)
if err != nil {
    if cache.IsNotFound(err) {
        lastErr = err
        continue  // OK - cache miss é esperado
    }
    lastErr = err
    continue  // ❌ Erro não logado, apenas continua!
}
```

## Solução Implementada

### 1. **ResilientLayer agora loga todos os erros**

#### Get Operation
```go
if err != nil {
    if err == gobreaker.ErrOpenState {
        rl.logger.Warn("circuit breaker open - request rejected",
            zap.String("operation", "get"),
            zap.String("key", key),
        )
        return nil, cache.ErrCircuitOpen
    }
    if ctx.Err() == context.DeadlineExceeded {
        rl.logger.Warn("operation timeout",
            zap.String("operation", "get"),
            zap.String("key", key),
            zap.Duration("timeout", rl.timeout),
            zap.Duration("elapsed", duration),
        )
        return nil, cache.ErrTimeout
    }
    // ✅ Agora loga outros erros
    rl.logger.Error("get operation failed",
        zap.String("operation", "get"),
        zap.String("key", key),
        zap.Duration("duration", duration),
        zap.Error(err),
    )
    return nil, err
}
```

#### Set Operation
```go
if err != nil {
    if err == gobreaker.ErrOpenState {
        rl.logger.Warn("circuit breaker open - request rejected",
            zap.String("operation", "set"),
        )
        return cache.ErrCircuitOpen
    }
    if ctx.Err() == context.DeadlineExceeded {
        rl.logger.Warn("operation timeout",
            zap.String("operation", "set"),
            zap.Duration("timeout", rl.timeout),
            zap.Duration("elapsed", duration),
        )
        return cache.ErrTimeout
    }
    // ✅ Agora loga erros de set
    rl.logger.Error("set operation failed",
        zap.String("operation", "set"),
        zap.Duration("ttl", ttl),
        zap.Duration("duration", duration),
        zap.Error(err),
    )
    return err
}
```

#### Delete Operation
```go
if err != nil {
    if err == gobreaker.ErrOpenState {
        rl.logger.Warn("circuit breaker open - request rejected",
            zap.String("operation", "delete"),
            zap.String("key", key),
        )
        return cache.ErrCircuitOpen
    }
    if ctx.Err() == context.DeadlineExceeded {
        rl.logger.Warn("operation timeout",
            zap.String("operation", "delete"),
            zap.String("key", key),
            zap.Duration("timeout", rl.timeout),
            zap.Duration("elapsed", duration),
        )
        return cache.ErrTimeout
    }
    // ✅ Agora loga erros de delete
    rl.logger.Error("delete operation failed",
        zap.String("operation", "delete"),
        zap.String("key", key),
        zap.Duration("duration", duration),
        zap.Error(err),
    )
    return err
}
```

### 2. **Chain agora diferencia cache miss de erros reais**

```go
value, err := layer.Get(ctx, key)
if err != nil {
    // Cache miss é esperado - apenas debug
    if cache.IsNotFound(err) {
        c.logger.Debug("layer miss",
            zap.String("key", key),
            zap.Int("layer_index", i),
            zap.String("layer_name", layer.Name()),
        )
        lastErr = err
        continue
    }
    // ✅ Erros reais agora são logados como WARN
    c.logger.Warn("layer error - falling back to next",
        zap.String("key", key),
        zap.Int("layer_index", i),
        zap.String("layer_name", layer.Name()),
        zap.Error(err),
    )
    lastErr = err
    continue
}
```

## Tipos de Erros Agora Visíveis nos Logs

### Nível ERROR
- **Falhas de operação** (get/set/delete que falharam)
- **Erros de conexão** (Redis, PostgreSQL)
- **Erros de serialização** (JSON marshal/unmarshal)
- **Erros inesperados** (qualquer erro não tratado especificamente)

### Nível WARN
- **Circuit breaker aberto** (proteção ativada)
- **Timeouts** (operação demorou demais)
- **Layer fallback** (erro em uma camada, tentando próxima)

### Nível DEBUG
- **Cache miss** (chave não encontrada - comportamento esperado)
- **Layer miss** (miss em uma camada específica)

## Como Ver os Erros Agora

### 1. Logs em tempo real
```bash
# Ver todos os erros
docker logs -f banking-api 2>&1 | grep -E '"level":"(error|warn)"'

# Ver apenas errors críticos
docker logs -f banking-api 2>&1 | grep '"level":"error"'

# Ver erros de operação específica
docker logs -f banking-api 2>&1 | grep '"operation":"get"' | grep error

# Ver circuit breaker events
docker logs -f banking-api 2>&1 | grep "circuit breaker"
```

### 2. Análise com jq (formato JSON)
```bash
# Contar erros por tipo
docker logs banking-api 2>&1 | jq -r 'select(.level=="error") | .msg' | sort | uniq -c

# Ver erros com contexto completo
docker logs banking-api 2>&1 | jq 'select(.level=="error")'

# Erros por layer
docker logs banking-api 2>&1 | jq -r 'select(.level=="error") | .logger' | sort | uniq -c

# Erros por operação
docker logs banking-api 2>&1 | jq -r 'select(.level=="error") | .operation' | sort | uniq -c
```

### 3. Console format (desenvolvimento)
```bash
LOG_FORMAT=console docker compose up

# Logs ficam mais legíveis:
# 2025-12-09T10:30:45.123Z  ERROR  resilience.L2-Redis  get operation failed  
#   {"operation": "get", "key": "txn:123", "duration": "1.234s", "error": "dial tcp: connection refused"}
```

## Correlação Logs ↔ Métricas

### Métricas no Grafana
```promql
# Taxa de erros por layer
rate(banking_api_cache_errors_total[1m])

# Erros por operação
rate(banking_api_cache_errors_total{operation="get"}[1m])
```

### Logs Correspondentes
```bash
# Ver logs dos erros que aparecem nas métricas
docker logs banking-api 2>&1 | grep '"level":"error"' | grep '"operation":"get"'
```

## Próximos Passos Recomendados

1. **Agregação de Logs**: Considere usar ferramentas como:
   - **Loki + Grafana** para correlacionar logs e métricas
   - **Elasticsearch + Kibana** para análise avançada
   - **CloudWatch Logs** se em AWS

2. **Alertas**: Configure alertas baseados em:
   - Taxa de erros > threshold
   - Circuit breaker aberto por muito tempo
   - Timeouts frequentes em uma layer

3. **Análise de Padrões**:
   ```bash
   # Ver erros mais comuns
   docker logs banking-api 2>&1 | jq -r 'select(.level=="error") | .error' | sort | uniq -c | sort -nr
   
   # Ver keys que mais falham
   docker logs banking-api 2>&1 | jq -r 'select(.level=="error") | .key' | sort | uniq -c | sort -nr
   ```

## Exemplo de Output Esperado

### Durante Load Test com Erros
```json
{"level":"error","ts":"2025-12-09T10:30:45.123Z","logger":"resilience.L2-Redis","msg":"get operation failed","operation":"get","key":"txn:abc123","duration":"1.234s","error":"dial tcp 127.0.0.1:6379: connect: connection refused"}

{"level":"warn","ts":"2025-12-09T10:30:45.234Z","logger":"cache-chain","msg":"layer error - falling back to next","key":"txn:abc123","layer_index":1,"layer_name":"L2-Redis","error":"dial tcp 127.0.0.1:6379: connect: connection refused"}

{"level":"error","ts":"2025-12-09T10:30:46.123Z","logger":"resilience.L2-Redis","msg":"set operation failed","operation":"set","ttl":"5m0s","duration":"1.045s","error":"i/o timeout"}
```

### Console Format (mais legível)
```
2025-12-09T10:30:45.123Z  ERROR  resilience.L2-Redis  get operation failed  
  {"operation": "get", "key": "txn:abc123", "duration": "1.234s", "error": "connection refused"}

2025-12-09T10:30:45.234Z  WARN   cache-chain  layer error - falling back to next  
  {"key": "txn:abc123", "layer_index": 1, "layer_name": "L2-Redis", "error": "connection refused"}
```

## Resumo

✅ **Problema resolvido**: Erros agora são visíveis nos logs  
✅ **Níveis adequados**: ERROR para falhas, WARN para timeouts/circuit breaker  
✅ **Contexto completo**: Key, layer, operation, duration, error message  
✅ **Correlacionável**: Logs podem ser correlacionados com métricas do Prometheus  

Agora você pode ver nos logs **exatamente** quais erros estão causando os problemas que aparecem no Grafana! 🎯
