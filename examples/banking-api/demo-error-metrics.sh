#!/bin/bash

# Script para demonstrar métricas de erro diferenciadas por tipo
# Este script simula diferentes tipos de erros e mostra como eles aparecem nas métricas

set -e

BANKING_API_URL="http://localhost:8080"
PROMETHEUS_URL="http://localhost:9090"

echo "🔍 Demonstração de Métricas de Erro Diferenciadas"
echo "=================================================="
echo ""

# Função para verificar se os serviços estão rodando
check_services() {
    echo "✓ Verificando serviços..."
    
    if ! curl -s "$BANKING_API_URL/health" > /dev/null 2>&1; then
        echo "❌ Banking API não está respondendo em $BANKING_API_URL"
        echo "   Execute: docker-compose up -d"
        exit 1
    fi
    
    if ! curl -s "$PROMETHEUS_URL/-/ready" > /dev/null 2>&1; then
        echo "❌ Prometheus não está respondendo em $PROMETHEUS_URL"
        echo "   Execute: docker-compose up -d"
        exit 1
    fi
    
    echo "✓ Todos os serviços estão rodando"
    echo ""
}

# Função para consultar métricas do Prometheus
query_metrics() {
    local query="$1"
    local name="$2"
    
    echo "📊 $name"
    echo "   Query: $query"
    
    result=$(curl -s -G "$PROMETHEUS_URL/api/v1/query" \
        --data-urlencode "query=$query" | \
        jq -r '.data.result[] | "\(.metric | to_entries | map("\(.key)=\(.value)") | join(", ")): \(.value[1])"' 2>/dev/null)
    
    if [ -z "$result" ]; then
        echo "   ⚠️  Nenhum dado disponível ainda"
    else
        echo "$result" | while IFS= read -r line; do
            echo "   → $line"
        done
    fi
    echo ""
}

# Aguardar um pouco para garantir que temos dados
wait_for_metrics() {
    echo "⏳ Aguardando coleta de métricas (10 segundos)..."
    sleep 10
    echo ""
}

# Verificar serviços
check_services

# Aguardar métricas
wait_for_metrics

echo "════════════════════════════════════════════════"
echo "1️⃣  DISTRIBUIÇÃO DE ERROS POR TIPO"
echo "════════════════════════════════════════════════"
echo ""
query_metrics \
    "sum by(error_type) (rate(banking_api_cache_errors_total[1m]))" \
    "Taxa de erros por tipo (últimos 1min)"

echo "════════════════════════════════════════════════"
echo "2️⃣  ERROS POR CAMADA E TIPO"
echo "════════════════════════════════════════════════"
echo ""
query_metrics \
    "sum by(layer, error_type) (rate(banking_api_cache_errors_total[1m]))" \
    "Erros detalhados por camada e tipo"

echo "════════════════════════════════════════════════"
echo "3️⃣  CIRCUIT BREAKER ERRORS"
echo "════════════════════════════════════════════════"
echo ""
query_metrics \
    "rate(banking_api_cache_errors_total{error_type=\"circuit_breaker_open\"}[1m])" \
    "Taxa de erros de circuit breaker"

echo "════════════════════════════════════════════════"
echo "4️⃣  TIMEOUT ERRORS"
echo "════════════════════════════════════════════════"
echo ""
query_metrics \
    "rate(banking_api_cache_errors_total{error_type=\"timeout\"}[1m])" \
    "Taxa de erros de timeout"

echo "════════════════════════════════════════════════"
echo "5️⃣  CONNECTION ERRORS"
echo "════════════════════════════════════════════════"
echo ""
query_metrics \
    "rate(banking_api_cache_errors_total{error_type=\"connection\"}[1m])" \
    "Taxa de erros de conexão"

echo "════════════════════════════════════════════════"
echo "6️⃣  KEY NOT FOUND (Cache Misses Tratados como Erro)"
echo "════════════════════════════════════════════════"
echo ""
query_metrics \
    "rate(banking_api_cache_errors_total{error_type=\"key_not_found\"}[1m])" \
    "Taxa de key_not_found"

echo "════════════════════════════════════════════════"
echo "7️⃣  TOP 5 TIPOS DE ERRO"
echo "════════════════════════════════════════════════"
echo ""
query_metrics \
    "topk(5, sum by(error_type) (rate(banking_api_cache_errors_total[5m])))" \
    "Top 5 tipos de erro mais frequentes"

echo "════════════════════════════════════════════════"
echo "8️⃣  ERROS POR OPERAÇÃO"
echo "════════════════════════════════════════════════"
echo ""
query_metrics \
    "sum by(operation, error_type) (rate(banking_api_cache_errors_total[1m]))" \
    "Erros agrupados por operação (get/set/delete)"

echo "════════════════════════════════════════════════"
echo "9️⃣  TAXA DE ERRO GERAL POR CAMADA"
echo "════════════════════════════════════════════════"
echo ""
query_metrics \
    "sum by(layer) (rate(banking_api_cache_errors_total[1m]))" \
    "Taxa total de erros por camada"

echo "════════════════════════════════════════════════"
echo "🔟 COMPARAÇÃO: ERROS vs OPERAÇÕES TOTAIS"
echo "════════════════════════════════════════════════"
echo ""

echo "📊 Taxa de erro percentual por camada"
echo "   Query: sum by(layer) (rate(banking_api_cache_errors_total[5m])) / (sum by(layer) (rate(banking_api_cache_hits_total[5m])) + sum by(layer) (rate(banking_api_cache_misses_total[5m]))) * 100"
echo ""

result=$(curl -s -G "$PROMETHEUS_URL/api/v1/query" \
    --data-urlencode "query=sum by(layer) (rate(banking_api_cache_errors_total[5m])) / (sum by(layer) (rate(banking_api_cache_hits_total[5m])) + sum by(layer) (rate(banking_api_cache_misses_total[5m]))) * 100" | \
    jq -r '.data.result[] | "\(.metric.layer): \(.value[1] | tonumber | . * 100 | round / 100)%"' 2>/dev/null)

if [ -z "$result" ]; then
    echo "   ⚠️  Nenhum dado disponível ainda"
else
    echo "$result" | while IFS= read -r line; do
        echo "   → $line"
    done
fi
echo ""

echo "════════════════════════════════════════════════"
echo "📈 VISUALIZAÇÃO NO GRAFANA"
echo "════════════════════════════════════════════════"
echo ""
echo "Acesse o dashboard para visualização interativa:"
echo "   http://localhost:3000/d/banking-api"
echo ""
echo "Novos painéis disponíveis:"
echo "   • Error Distribution by Type (Pie Chart)"
echo "   • Errors by Type and Layer (Stacked Bars)"
echo "   • Cache Errors by Type (Time Series)"
echo ""

echo "════════════════════════════════════════════════"
echo "📋 LOGS ESTRUTURADOS COM ERROR_TYPE"
echo "════════════════════════════════════════════════"
echo ""
echo "Visualize logs com tipo de erro:"
echo ""
echo "# Todos os erros com tipo classificado"
echo "docker logs banking-api 2>&1 | jq -r 'select(.level==\"error\" and .error_type) | \"[\(.ts)] \(.logger) - \(.msg) | Type: \(.error_type) | Error: \(.error)\"'"
echo ""
echo "# Circuit breaker opens"
echo "docker logs banking-api 2>&1 | jq -r 'select(.error_type==\"circuit_breaker_open\") | \"[\(.ts)] \(.layer) - Circuit Breaker OPEN\"'"
echo ""
echo "# Timeouts"
echo "docker logs banking-api 2>&1 | jq -r 'select(.error_type==\"timeout\") | \"[\(.ts)] \(.logger) - Timeout after \(.duration)\"'"
echo ""
echo "# Connection errors"
echo "docker logs banking-api 2>&1 | jq -r 'select(.error_type==\"connection\") | \"[\(.ts)] \(.logger) - Connection Error: \(.error)\"'"
echo ""

echo "════════════════════════════════════════════════"
echo "✅ Demonstração Concluída"
echo "════════════════════════════════════════════════"
echo ""
echo "📚 Documentação completa: docs/ERROR_METRICS.md"
echo ""
