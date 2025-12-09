#!/bin/bash

echo "Aguardando API inicializar..."
for i in {1..30}; do
    if curl -s http://localhost:8080/health > /dev/null 2>&1; then
        echo "✓ API está rodando"
        break
    fi
    sleep 1
done

echo ""
echo "Verificando métricas de cache:"
curl -s http://localhost:8080/metrics | grep "banking_api_cache" | head -20

echo ""
echo "Criando uma transação de teste..."
curl -s -X POST http://localhost:8080/transactions \
    -H "Content-Type: application/json" \
    -d '{
        "account_id": "ACC-TEST-001",
        "type": "debit",
        "amount": 100.00,
        "currency": "USD",
        "description": "Test transaction"
    }' | jq -r '.id' > /tmp/tx_id.txt

TX_ID=$(cat /tmp/tx_id.txt)
echo "✓ Transação criada: $TX_ID"

echo ""
echo "Lendo a transação (primeira vez - cache miss)..."
curl -s "http://localhost:8080/transactions/$TX_ID" > /dev/null

echo "Lendo a transação (segunda vez - cache hit)..."
curl -s "http://localhost:8080/transactions/$TX_ID" > /dev/null

echo ""
echo "Métricas de cache após operações:"
curl -s http://localhost:8080/metrics | grep "banking_api_cache_\(hits\|misses\)_total"
