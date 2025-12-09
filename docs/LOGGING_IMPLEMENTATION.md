# Cache-Chain Structured Logging Implementation

## ✅ Implementação Completa

Sistema de logs estruturados usando **zap** foi adicionado a todas as camadas do cache-chain com nível de log configurável via variáveis de ambiente.

## 📦 Arquivos Criados/Modificados

### Novos Arquivos
1. **`pkg/logging/logger.go`** - Sistema de logging com zap
   - Config para nível, formato e modo de desenvolvimento
   - Logger global e factory methods
   - Suporte a variáveis de ambiente (LOG_LEVEL, LOG_FORMAT, LOG_DEV)

2. **`examples/banking-api/LOGGING.md`** - Documentação completa
   - Configuração via environment variables
   - Exemplos de uso
   - Estrutura dos logs
   - Dicas de análise e performance

3. **`examples/banking-api/test-logging.sh`** - Script de teste
   - Testa diferentes configurações de log
   - Validação prática

### Arquivos Modificados

#### Core Library
1. **`pkg/cache/memory/memory.go`**
   - Logger field na struct MemoryCache
   - Logger config field no MemoryCacheConfig
   - Logs de: inicialização, hits, misses, sets, deletes, evictions

2. **`pkg/cache/redis/redis.go`**
   - Logger field na struct RedisCache
   - Logger config field no RedisCacheConfig
   - Logs de: inicialização, conexão, hits, misses, sets, deletes, erros

3. **`pkg/chain/chain.go`**
   - Logger field na struct Chain
   - Logger config field no ChainConfig
   - Logs de: inicialização, chain get started/completed, warming layers

4. **`pkg/resilience/layer.go`**
   - Logger field na struct ResilientLayer
   - Logs de: inicialização, circuit breaker state changes, timeouts

#### Banking API Example
5. **`examples/banking-api/main.go`**
   - Inicialização do logger via NewLoggerFromEnv()
   - SetGlobal() para logger global
   - Passar logger para todas as camadas
   - Substituir log.Println por logger.Info/Error/Fatal

6. **`examples/banking-api/docker-compose.yml`**
   - Variáveis de ambiente: LOG_LEVEL, LOG_FORMAT, LOG_DEV
   - Valores padrão configuráveis

7. **`examples/banking-api/README.md`**
   - Seção sobre configuração de logs
   - Exemplos de uso
   - Link para LOGGING.md

## 🎯 Funcionalidades

### Níveis de Log
- `debug` - Logs detalhados (cache hits/misses, operações)
- `info` - Informações gerais (inicialização, estado)
- `warn` - Avisos (circuit breaker, timeouts)
- `error` - Erros (falhas de operação)

### Formatos
- `json` - Estruturado para produção e análise
- `console` - Legível para desenvolvimento

### Configuração
```bash
# Via environment variables
export LOG_LEVEL=debug
export LOG_FORMAT=console
export LOG_DEV=true

# Via Docker Compose
LOG_LEVEL=debug docker compose up
```

## 📊 Logs por Camada

### Memory Cache (L1)
```json
{
  "level": "debug",
  "logger": "L1-Memory",
  "msg": "cache hit",
  "key": "txn:123",
  "ttl_remaining": "4m30s"
}
```

### Redis Cache (L2)
```json
{
  "level": "debug",
  "logger": "L2-Redis",
  "msg": "cache set",
  "key": "txn:123",
  "ttl": "5m0s",
  "value_size": 256
}
```

### Chain
```json
{
  "level": "debug",
  "logger": "cache-chain",
  "msg": "chain get completed",
  "key": "txn:123",
  "hit_layer": 0,
  "layer_name": "L1-Memory",
  "duration": "123µs"
}
```

### Resilience Layer
```json
{
  "level": "warn",
  "logger": "resilience.L2-Redis",
  "msg": "circuit breaker state changed",
  "layer": "L2-Redis",
  "from": "closed",
  "to": "open"
}
```

## 🔍 Exemplos de Análise

### Ver cache hits/misses
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

## 🚀 Como Usar

### Desenvolvimento Local
```bash
cd examples/banking-api

# Com logs debug em console
LOG_LEVEL=debug LOG_FORMAT=console go run main.go

# Modo desenvolvimento
LOG_DEV=true go run main.go
```

### Docker Compose
```bash
# Produção (padrão: info + json)
docker compose up

# Debug
LOG_LEVEL=debug docker compose up

# Desenvolvimento
LOG_DEV=true docker compose up

# Ver logs
docker logs -f banking-api
```

### Load Testing com Logs
```bash
# Inicie com debug
LOG_LEVEL=debug docker compose up -d

# Execute load test
./loadtest.sh

# Analise os logs
docker logs banking-api 2>&1 | grep "cache hit" | wc -l
docker logs banking-api 2>&1 | grep "cache miss" | wc -l
```

## 📚 Dependências Adicionadas

```bash
go get go.uber.org/zap
```

Já incluída no go.mod do projeto.

## ✨ Benefícios

1. **Observabilidade**: Visibilidade completa de todas as operações
2. **Debug**: Facilita troubleshooting de problemas de cache
3. **Performance**: Logs estruturados com zero-allocation
4. **Configurável**: Nível e formato ajustáveis sem recompilação
5. **Produção-ready**: Formato JSON para integração com ferramentas
6. **Desenvolvimento-friendly**: Modo console legível

## 🎓 Referências

- [Zap Documentation](https://pkg.go.dev/go.uber.org/zap)
- [LOGGING.md](examples/banking-api/LOGGING.md) - Documentação detalhada
- [README.md](examples/banking-api/README.md) - Banking API docs
