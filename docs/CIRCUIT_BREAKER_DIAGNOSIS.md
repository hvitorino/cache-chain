# Diagnóstico: Circuit Breakers Abrindo Muito Rapidamente

## 🔴 Problema Observado

Circuit breakers estão abrindo em **menos de 1 segundo** após receberem carga, causando rejeição de 99% das requisições.

**Evidência dos logs:**
```
15:11:18.386 - L1-Memory:  closed → open
15:11:19.291 - L2-Redis:   closed → open
15:11:28.389 - L1-Memory:  open → half-open → closed → open (ciclo rápido!)
15:11:29.294 - L2-Redis:   open → half-open → open (falha na recuperação)
15:11:53.497 - PostgreSQL: closed → open (cascata completa)
```

---

## 🔍 Análise da Configuração Atual

### Configuração dos Circuit Breakers

```
L1-Memory:   timeout=100ms, max_requests=1, interval=0s, cb_timeout=10s
L2-Redis:    timeout=1s,    max_requests=1, interval=0s, cb_timeout=10s  
PostgreSQL:  timeout=1s,    max_requests=1, interval=0s, cb_timeout=10s
```

### Função ReadyToTrip (Código)

```go
// pkg/resilience/config.go - linha 52
ReadyToTrip: func(counts Counts) bool {
    return counts.ConsecutiveFailures >= 5  // ← Threshold muito baixo!
},
```

---

## 🎯 Causas Raiz Identificadas

### 1. **Threshold Muito Baixo (5 falhas consecutivas)**

**Problema:** Com apenas **5 falhas consecutivas**, o circuit breaker abre.

**Cenário real:**
```
Request 1: timeout (timeout=100ms é muito agressivo para L1-Memory)
Request 2: timeout
Request 3: timeout  
Request 4: timeout
Request 5: timeout
→ Circuit breaker ABRE após 500ms de carga!
```

**Por que é problemático:**
- Em cenários de alta concorrência, 5 requisições chegam em **milissegundos**
- Uma pequena latência causa abertura imediata
- Não há "tempo de respiração" para recuperação transitória

**Recomendação:**
```go
ReadyToTrip: func(counts Counts) bool {
    failureRate := float64(counts.TotalFailures) / float64(counts.Requests)
    return counts.Requests >= 20 && failureRate > 0.5  // 50% de erro em 20+ req
},
```

---

### 2. **Interval=0s (Contador Nunca Reseta)**

**Problema:** `Interval: 0` significa que o contador de falhas **nunca** é limpo.

**Comportamento:**
```
Tempo 0s:  5 falhas → CB abre
Tempo 10s: CB tenta half-open
           1 falha → CB abre novamente (contador acumulado!)
Tempo 20s: CB tenta half-open
           1 falha → CB abre (falhas anteriores ainda contam!)
```

**Por que é problemático:**
- Falhas antigas continuam contando indefinidamente
- Uma rajada inicial de erros "condena" o circuit breaker permanentemente
- Não há "perdão" para recuperação

**Recomendação:**
```go
Interval: 60 * time.Second,  // Reseta contadores a cada 60s
```

---

### 3. **MaxRequests=1 (Half-Open Muito Frágil)**

**Problema:** No estado **half-open**, apenas **1 requisição** é permitida para testar recuperação.

**Comportamento:**
```
CB em half-open: permite 1 request
→ Se essa request falhar (mesmo por timeout transitório) → CB volta para open
→ Espera mais 10s antes de tentar novamente
```

**Por que é problemático:**
- **Sample size muito pequeno:** 1 requisição não é estatisticamente significativo
- Qualquer latência momentânea reabre o circuit breaker
- Dificulta recuperação em sistemas com jitter natural

**Exemplo do log:**
```
15:11:28.389 - L1-Memory: half-open → closed → open (em milissegundos!)
```
↳ Conseguiu fechar, mas reabriu imediatamente porque o próximo request falhou

**Recomendação:**
```go
MaxRequests: 5,  // Permite 5 requests em half-open
```

---

### 4. **Timeout=100ms para L1-Memory (Muito Agressivo)**

**Problema:** Timeout de **100ms** para operações de memória é **ultrarrápido**.

**Realidade:**
```
Operação de memória normal: 10-50μs (microsegundos)
Timeout configurado:        100ms (100,000μs)
```

**Porém, sob carga:**
- **GC pause:** pode levar 10-50ms
- **Lock contention:** múltiplas goroutines competindo
- **System scheduler:** atraso de CPU

**Resultado:** Timeouts falsos sob carga moderada.

**Recomendação:**
```go
// L1 (Memory)
timeout: 500 * time.Millisecond,

// L2 (Redis)  
timeout: 2 * time.Second,

// L3 (PostgreSQL)
timeout: 5 * time.Second,
```

---

### 5. **CB Timeout=10s (Recuperação Muito Rápida)**

**Problema:** Circuit breaker tenta reabrir a cada **10 segundos**.

**Ciclo observado:**
```
T=0s:  CB abre (5 falhas)
T=10s: CB tenta half-open
       → 1 falha → reabre
T=20s: CB tenta half-open
       → 1 falha → reabre
T=30s: CB tenta half-open
       → 1 falha → reabre
```

**Por que é problemático:**
- Sistema sob stress não tem tempo para se estabilizar
- Backend sobrecarregado não teve 10s de "descanso" suficiente
- Cria ciclo de "flapping" (abre/fecha rapidamente)

**Recomendação:**
```go
Timeout: 30 * time.Second,  // Aguarda 30s antes de tentar novamente
```

---

## 🧪 Simulação: Por Que Abre Tão Rápido?

### Cenário: Load Test com 10 req/s

**Configuração Atual:**
- Threshold: 5 falhas consecutivas
- Interval: 0s (nunca reseta)
- MaxRequests: 1
- Timeout: 100ms (L1)

**Timeline:**
```
T=0.000s: Request 1 → timeout (100ms) → Falha 1
T=0.100s: Request 2 → timeout (100ms) → Falha 2
T=0.200s: Request 3 → timeout (100ms) → Falha 3
T=0.300s: Request 4 → timeout (100ms) → Falha 4
T=0.400s: Request 5 → timeout (100ms) → Falha 5
T=0.500s: 🔴 CIRCUIT BREAKER ABRE!

T=0.501s: Request 6  → Rejeitado (CB open)
T=0.502s: Request 7  → Rejeitado (CB open)
...
T=10.5s:  CB tenta half-open
          Request 50 → 1 falha → 🔴 REABRE!
```

**Resultado:** Circuit breaker aberto **99% do tempo** após 500ms de carga!

---

## 📊 Comparação: Atual vs Recomendado

| Parâmetro | Atual | Recomendado | Impacto |
|-----------|-------|-------------|---------|
| **Threshold** | 5 falhas consecutivas | 50% error rate em 20+ req | 4x mais resiliente |
| **Interval** | 0s (nunca reseta) | 60s | Permite recuperação |
| **MaxRequests** | 1 | 5 | 5x mais confiança |
| **Timeout L1** | 100ms | 500ms | 5x mais tolerante |
| **Timeout L2** | 1s | 2s | 2x mais tolerante |
| **CB Timeout** | 10s | 30s | 3x mais tempo para estabilizar |

---

## ✅ Solução Recomendada

### Opção 1: Configuração Conservadora (Produção)

```go
// pkg/resilience/config.go
func DefaultResilientConfig() ResilientConfig {
    return ResilientConfig{
        Timeout: 5 * time.Second,
        CircuitBreakerConfig: CircuitBreakerConfig{
            MaxRequests: 5,                    // ← 5 requests em half-open
            Interval:    60 * time.Second,     // ← Reseta a cada 60s
            Timeout:     30 * time.Second,     // ← Aguarda 30s antes de reabrir
            ReadyToTrip: func(counts Counts) bool {
                // Abre se >50% de erro após 20+ requisições
                if counts.Requests < 20 {
                    return false
                }
                failureRate := float64(counts.TotalFailures) / float64(counts.Requests)
                return failureRate > 0.5
            },
        },
    }
}
```

**Timeouts por camada (chain.go):**
```go
if i == 0 {
    // L1 - Memory
    resConfig = resConfig.WithTimeout(500 * time.Millisecond)
} else if i == 1 {
    // L2 - Redis
    resConfig = resConfig.WithTimeout(2 * time.Second)
} else {
    // L3+ - PostgreSQL, etc
    resConfig = resConfig.WithTimeout(5 * time.Second)
}
```

---

### Opção 2: Configuração Agressiva (Performance Crítica)

Para cenários onde preferência é **fail fast** com recuperação rápida:

```go
CircuitBreakerConfig{
    MaxRequests: 3,
    Interval:    30 * time.Second,
    Timeout:     15 * time.Second,
    ReadyToTrip: func(counts Counts) bool {
        // Mais agressivo: 60% de erro em 10+ req
        if counts.Requests < 10 {
            return false
        }
        failureRate := float64(counts.TotalFailures) / float64(counts.Requests)
        return failureRate > 0.6
    },
}
```

---

### Opção 3: Configuração por Camada (Híbrida)

```go
// No main.go
l1Config := resilience.DefaultResilientConfig()
l1Config.Timeout = 500 * time.Millisecond
l1Config.CircuitBreakerConfig.Timeout = 20 * time.Second

l2Config := resilience.DefaultResilientConfig()
l2Config.Timeout = 2 * time.Second
l2Config.CircuitBreakerConfig.Timeout = 30 * time.Second

l3Config := resilience.DefaultResilientConfig()
l3Config.Timeout = 5 * time.Second
l3Config.CircuitBreakerConfig.Timeout = 60 * time.Second

cacheChain, err := chain.NewWithConfig(
    chain.ChainConfig{
        ResilientConfigs: []resilience.ResilientConfig{
            l1Config,
            l2Config,
            l3Config,
        },
        // ...
    },
    memCache, redisCache, pgAdapter,
)
```

---

## 🎯 Implementação Imediata

### Mudanças Mínimas (Quick Fix)

Editar apenas `pkg/resilience/config.go`:

```go
func DefaultResilientConfig() ResilientConfig {
    return ResilientConfig{
        Timeout: 5 * time.Second,
        CircuitBreakerConfig: CircuitBreakerConfig{
            MaxRequests: 5,                    // era: 1
            Interval:    60 * time.Second,     // era: 0
            Timeout:     30 * time.Second,     // era: 10s
            ReadyToTrip: func(counts Counts) bool {
                // Nova lógica: error rate em vez de falhas consecutivas
                if counts.Requests < 20 {
                    return false
                }
                failureRate := float64(counts.TotalFailures) / float64(counts.Requests)
                return failureRate > 0.5       // era: >= 5 falhas
            },
        },
    }
}
```

**Resultado esperado:**
- Circuit breaker aguarda 20+ requisições antes de decidir
- Tolera até 50% de falhas (10 em 20) antes de abrir
- Reseta contadores a cada 60s (permite recuperação de rajadas)
- Testa com 5 requisições em half-open (mais confiança)
- Aguarda 30s antes de retentar (sistema tem tempo de estabilizar)

---

## 📈 Validação Pós-Mudança

### Métricas para Monitorar

```promql
# Taxa de abertura de circuit breakers (deve reduzir drasticamente)
rate(banking_api_circuit_opens_total[5m])

# Estado dos circuit breakers (0=closed, 1=open, 2=half-open)
banking_api_circuit_state

# Taxa de erros (deve aumentar inicialmente, depois estabilizar)
rate(banking_api_cache_errors_total[1m])

# Latência (deve permanecer estável)
histogram_quantile(0.95, banking_api_cache_get_latency_seconds)
```

### Logs para Verificar

```bash
# Frequência de mudanças de estado (deve reduzir)
docker logs banking-api 2>&1 | grep "circuit breaker state changed" | wc -l

# Tempo entre open → half-open (deve ser 30s)
docker logs banking-api 2>&1 | grep "half-open" | tail -10

# Erros por tipo (identificar verdadeiros problemas)
docker logs banking-api 2>&1 | jq -r 'select(.error_type) | .error_type' | sort | uniq -c
```

---

## 🚨 Sinais de Sucesso

### Antes (Problema):
```
✗ Circuit breakers abrem em <1s sob carga
✗ Ciclo: open → half-open → open (flapping)
✗ 99% de requisições rejeitadas
✗ Recuperação impossível
```

### Depois (Corrigido):
```
✓ Circuit breakers permanecem closed sob carga normal
✓ Abrem apenas quando >50% de erro real (20+ req)
✓ Half-open testa com 5 requisições (sample size adequado)
✓ Recuperação gradual e estável
✓ Contadores resetam a cada 60s (permite recuperação)
```

---

## 📚 Referências

- **gobreaker Documentation**: https://github.com/sony/gobreaker
- **Circuit Breaker Pattern**: Martin Fowler - https://martinfowler.com/bliki/CircuitBreaker.html
- **SRE Best Practices**: Google SRE Book - Handling Overload
- **Código Fonte**: `pkg/resilience/config.go`, `pkg/resilience/layer.go`
