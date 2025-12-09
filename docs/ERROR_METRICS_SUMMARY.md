# Sumário: Métricas de Erro Diferenciadas por Tipo

## 🎯 Objetivo

Diferenciar os tipos de erros nas métricas de cache para melhorar observabilidade e facilitar diagnóstico de problemas específicos.

## 📊 Mudanças Implementadas

### 1. **Métrica Prometheus Atualizada**

**Antes:**
```
banking_api_cache_errors_total{layer="L1-Memory", operation="get"}
```

**Depois:**
```
banking_api_cache_errors_total{layer="L1-Memory", operation="get", error_type="circuit_breaker_open"}
```

**Novo Label:** `error_type` com 10 valores possíveis:
- `circuit_breaker_open` - Circuit breaker aberto
- `timeout` - Timeout de operação
- `key_not_found` - Chave não encontrada
- `connection` - Erro de conexão com backend
- `serialization` - Erro de serialização/deserialização
- `backend` - Erro específico do backend
- `invalid_key` - Chave inválida
- `invalid_value` - Valor inválido
- `unavailable` - Camada indisponível
- `other` - Outros erros não classificados

### 2. **Nova Função de Classificação**

**Arquivo:** `pkg/cache/errors.go`

```go
// ClassifyError returns a string classification of the error type for metrics
func ClassifyError(err error) string
```

Classifica erros usando:
1. Comparação direta com erros padrão (`errors.Is`)
2. Pattern matching no texto do erro
3. Fallback para "other"

### 3. **Novo Método na Interface MetricsCollector**

**Arquivo:** `pkg/metrics/metrics.go`

```go
// RecordError records a typed cache error
RecordError(layer, operation, errorType string)
```

Implementado em:
- `PrometheusCollector.RecordError()`
- `NoOpCollector.RecordError()`

### 4. **Integração na Camada de Resiliência**

**Arquivo:** `pkg/resilience/layer.go`

Todas as operações (Get, Set, Delete) agora:
1. Classificam o erro usando `cache.ClassifyError()`
2. Registram com `metrics.RecordError()`
3. Incluem `error_type` nos logs

**Exemplo:**
```go
if err == gobreaker.ErrOpenState {
    rl.metrics.RecordError(layerName, "get", "circuit_breaker_open")
    rl.logger.Warn("circuit breaker open - request rejected", ...)
    return nil, cache.ErrCircuitOpen
}
```

### 5. **Dashboard Grafana Atualizado**

**Arquivo:** `examples/banking-api/grafana-dashboard.json`

#### Painéis Novos:

**a) Error Distribution by Type (Pie Chart)**
- Localização: Row 10, posição (0, 62)
- Query: `sum by(error_type) (rate(banking_api_cache_errors_total[5m]))`
- Mostra: Distribuição percentual dos tipos de erro

**b) Errors by Type and Layer (Stacked Bars)**
- Localização: Row 10, posição (12, 62)
- Query: `sum by(error_type, layer) (rate(banking_api_cache_errors_total[1m]))`
- Mostra: Série temporal de erros empilhados por tipo e camada

#### Painéis Atualizados:

**c) Cache Errors by Type (Time Series)**
- ID: 8
- Legend: `{{layer}} - {{operation}} - {{error_type}}`
- Agora mostra o tipo de erro na legenda

### 6. **Documentação Completa**

**Arquivo:** `docs/ERROR_METRICS.md`

Conteúdo:
- Descrição detalhada de cada tipo de erro
- Severidade e ações recomendadas
- Queries Prometheus de exemplo
- Regras de alerting recomendadas
- Guia de troubleshooting
- Best practices

### 7. **Script de Demonstração**

**Arquivo:** `examples/banking-api/demo-error-metrics.sh`

Funcionalidades:
- Verifica serviços (Banking API + Prometheus)
- Consulta 10 métricas diferentes
- Mostra distribuição de erros
- Fornece exemplos de queries de log
- Links para Grafana

**Uso:**
```bash
cd examples/banking-api
./demo-error-metrics.sh
```

## 🔄 Compatibilidade

### Backward Compatible ✅

- **Métricas antigas continuam funcionando**: Queries sem `error_type` retornam soma de todos os tipos
- **Interface estendida**: `RecordError()` é adicional, não substitui métodos existentes
- **RecordSet/RecordDelete**: Continuam registrando erros com `error_type="other"` para compatibilidade

### Sem Breaking Changes

- Labels existentes mantidos: `layer`, `operation`
- Nome da métrica inalterado: `banking_api_cache_errors_total`
- Dashboards existentes continuam funcionando

## 📈 Impacto

### Cardinality
- **Antes**: 3 layers × 3 operations = 9 séries temporais
- **Depois**: 3 layers × 3 operations × ~5 error types (média) = ~45 séries temporais
- **Impacto**: Baixo (~5KB memória adicional)

### Performance
- **CPU**: Negligível (classificação de erro é simples string matching)
- **Latência**: Zero (só ocorre em caminhos de erro)
- **Throughput**: Sem impacto

## 🎨 Queries Úteis

### Erros por tipo (taxa)
```promql
sum by(error_type) (rate(banking_api_cache_errors_total[5m]))
```

### Top 5 tipos de erro
```promql
topk(5, sum by(error_type) (rate(banking_api_cache_errors_total[5m])))
```

### Circuit breaker opens por camada
```promql
rate(banking_api_cache_errors_total{error_type="circuit_breaker_open"}[1m])
```

### Percentual de timeouts
```promql
sum(rate(banking_api_cache_errors_total{error_type="timeout"}[5m])) 
/ 
sum(rate(banking_api_cache_errors_total[5m])) * 100
```

### Erros de conexão vs outros
```promql
sum by(error_type) (
  rate(banking_api_cache_errors_total{error_type=~"connection|timeout|circuit_breaker_open"}[5m])
)
```

## 🚨 Alertas Recomendados

### Circuit Breaker Opens
```yaml
alert: HighCircuitBreakerErrors
expr: sum by(layer) (rate(banking_api_cache_errors_total{error_type="circuit_breaker_open"}[5m])) > 1
severity: critical
```

### Connection Failures
```yaml
alert: CacheConnectionFailures
expr: sum by(layer) (rate(banking_api_cache_errors_total{error_type="connection"}[5m])) > 0.5
severity: critical
```

### High Timeout Rate
```yaml
alert: HighTimeoutRate
expr: sum by(layer) (rate(banking_api_cache_errors_total{error_type="timeout"}[5m])) > 2
severity: warning
```

## 📝 Logs Correlacionados

Todos os logs de erro agora incluem campo `error_type`:

```json
{
  "level": "error",
  "ts": "2025-12-09T14:37:43.622Z",
  "logger": "resilience.PostgreSQL",
  "msg": "get operation failed",
  "operation": "get",
  "key": "transaction:abc123",
  "duration": "11.770125ms",
  "error_type": "key_not_found",
  "error": "cache: key not found"
}
```

### Queries de Log

**Circuit breaker opens:**
```bash
docker logs banking-api 2>&1 | jq -r 'select(.error_type=="circuit_breaker_open")'
```

**Timeouts:**
```bash
docker logs banking-api 2>&1 | jq -r 'select(.error_type=="timeout")'
```

**Connection errors:**
```bash
docker logs banking-api 2>&1 | jq -r 'select(.error_type=="connection")'
```

## ✅ Testes

### Compilação
```bash
go build ./pkg/...  # ✓ Success
go build examples/banking-api  # ✓ Success
```

### Validação JSON
```bash
python3 -m json.tool grafana-dashboard.json > /dev/null  # ✓ Valid
```

### Verificação de Métricas
Após deploy, verificar:
1. Prometheus mostra novo label `error_type`
2. Grafana exibe novos painéis
3. Logs incluem campo `error_type`

## 🔗 Arquivos Modificados

### Core
- `pkg/cache/errors.go` - Nova função `ClassifyError()`
- `pkg/metrics/metrics.go` - Nova interface `RecordError()`
- `pkg/metrics/prometheus/prometheus.go` - Implementação `RecordError()`
- `pkg/resilience/layer.go` - Uso de `RecordError()` em Get/Set/Delete

### Dashboard
- `examples/banking-api/grafana-dashboard.json` - 2 painéis novos + 1 atualizado

### Documentação
- `docs/ERROR_METRICS.md` - Documentação completa (novo)
- `examples/banking-api/demo-error-metrics.sh` - Script de demo (novo)
- `docs/ERROR_METRICS_SUMMARY.md` - Este arquivo (novo)

## 🎯 Próximos Passos

1. **Testar em ambiente real**: Executar loadtest e observar métricas diferenciadas
2. **Configurar alertas**: Implementar regras de alerting por tipo de erro
3. **Refinar classificação**: Adicionar mais padrões de erro conforme necessário
4. **Dashboards customizados**: Criar views específicas por tipo de erro

## 📚 Referências

- [ERROR_METRICS.md](./ERROR_METRICS.md) - Documentação detalhada
- [LOGGING_IMPLEMENTATION.md](./LOGGING_IMPLEMENTATION.md) - Sistema de logging
- [CACHE_ERROR_LOGGING_FIX.md](./CACHE_ERROR_LOGGING_FIX.md) - Fix de visibilidade de erros
- [phase-6-metrics-observability.md](../rules/phase-6-metrics-observability.md) - Design original
