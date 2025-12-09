# Logging Configuration

O cache-chain agora possui suporte completo a logs estruturados usando a biblioteca [zap](https://github.com/uber-go/zap).

## Configuração via Variáveis de Ambiente

O nível de log e formato podem ser configurados através de variáveis de ambiente:

### `LOG_LEVEL`
Define o nível mínimo de log a ser exibido.

**Valores aceitos:**
- `debug` - Logs detalhados de debug (cache hits/misses, operações)
- `info` - Informações gerais (inicialização, estado do sistema)
- `warn` - Avisos (circuit breaker opens, timeouts)
- `error` - Erros (falhas de operação)
- `dpanic`, `panic`, `fatal` - Níveis críticos

**Padrão:** `info`

### `LOG_FORMAT`
Define o formato de saída dos logs.

**Valores aceitos:**
- `json` - Formato JSON estruturado (recomendado para produção)
- `console` - Formato legível para humanos (recomendado para desenvolvimento)

**Padrão:** `json`

### `LOG_DEV`
Ativa o modo de desenvolvimento.

**Valores aceitos:**
- `true` - Ativa modo desenvolvimento (console format, debug level, caller info)
- `false` - Modo produção

**Padrão:** `false`

## Exemplos de Uso

### Docker Compose

**Modo produção (padrão):**
```bash
docker compose up
# Logs em JSON, nível info
```

**Modo debug:**
```bash
LOG_LEVEL=debug docker compose up
# Logs detalhados de todas as operações
```

**Modo desenvolvimento:**
```bash
LOG_DEV=true docker compose up
# Logs em formato console, com caller info e stack traces
```

**Customizado:**
```bash
LOG_LEVEL=warn LOG_FORMAT=console docker compose up
# Apenas warnings e errors em formato console
```

### Aplicação Local

```bash
# Debug com formato console
export LOG_LEVEL=debug
export LOG_FORMAT=console
go run main.go

# Produção
export LOG_LEVEL=info
export LOG_FORMAT=json
go run main.go
```

## Estrutura dos Logs

### Formato JSON (produção)
```json
{
  "level": "info",
  "ts": "2025-12-09T10:30:45.123Z",
  "logger": "L1-Memory",
  "msg": "cache hit",
  "key": "txn:123",
  "ttl_remaining": "4m30s"
}
```

### Formato Console (desenvolvimento)
```
2025-12-09T10:30:45.123Z  INFO  L1-Memory  cache hit  {"key": "txn:123", "ttl_remaining": "4m30s"}
```

## Logs por Camada

### Memory Cache (`L1-Memory`)
- **Info:** Inicialização, configuração
- **Debug:** Hits, misses, sets, deletes, evictions

### Redis Cache (`L2-Redis`)
- **Info:** Conexão, modo cluster/sentinel
- **Debug:** Hits, misses, sets, deletes
- **Error:** Falhas de conexão, serialização

### Chain
- **Info:** Inicialização, número de layers
- **Debug:** Início/fim de gets, layer hit
- **Warn:** Cache miss em todas as camadas

### Resilience Layer
- **Info:** Inicialização, configuração de timeout e circuit breaker
- **Warn:** Circuit breaker state changes, timeouts

## Campos Comuns nos Logs

| Campo | Descrição | Exemplo |
|-------|-----------|---------|
| `key` | Chave do cache | `"txn:abc123"` |
| `layer` | Nome da camada | `"L1-Memory"` |
| `duration` | Tempo de operação | `"1.234ms"` |
| `ttl` | Time-to-live | `"5m0s"` |
| `ttl_remaining` | TTL restante | `"4m30s"` |
| `hit_layer` | Camada que deu hit | `0` (L1), `1` (L2), etc |
| `error` | Mensagem de erro | `"context deadline exceeded"` |
| `from`/`to` | Estado do circuit breaker | `"closed"`, `"open"`, `"half-open"` |

## Integração com Aplicações

### Usando o Logger Global

```go
import "cache-chain/pkg/logging"

// Inicializar logger
logger, _ := logging.NewLoggerFromEnv()
logging.SetGlobal(logger)
defer logger.Sync()

// Usar em qualquer lugar
logging.L().Info("mensagem", zap.String("campo", "valor"))
```

### Configurando Camadas de Cache

```go
import (
    "cache-chain/pkg/logging"
    "cache-chain/pkg/cache/memory"
)

logger, _ := logging.NewLoggerFromEnv()

config := memory.MemoryCacheConfig{
    Name:   "L1-Memory",
    Logger: logger,  // Passar logger para a camada
}
```

### Configurando Chain

```go
cacheChain, _ := chain.NewWithConfig(
    chain.ChainConfig{
        Logger: logger,  // Logger será usado pela chain
    },
    memCache, redisCache, pgAdapter,
)
```

## Dicas de Performance

1. **Produção:** Use `LOG_LEVEL=info` e `LOG_FORMAT=json`
2. **Debug de problemas:** Use `LOG_LEVEL=debug` temporariamente
3. **Desenvolvimento local:** Use `LOG_DEV=true`
4. **Logs estruturados** permitem melhor análise em ferramentas como:
   - Elasticsearch + Kibana
   - Loki + Grafana
   - CloudWatch Logs
   - Datadog

## Análise de Logs

### Ver apenas cache hits/misses
```bash
docker logs banking-api 2>&1 | grep -E '"msg":"cache (hit|miss)"'
```

### Ver circuit breaker events
```bash
docker logs banking-api 2>&1 | grep "circuit breaker"
```

### Ver apenas erros
```bash
docker logs banking-api 2>&1 | jq 'select(.level=="error")'
```

### Contar operações por layer
```bash
docker logs banking-api 2>&1 | jq -r '.logger' | sort | uniq -c
```
