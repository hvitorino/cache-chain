#!/bin/bash

# Script para testar diferentes configurações de log

echo "=== Testing Cache-Chain Logging Configuration ==="
echo ""

# Função para testar um nível de log
test_log_level() {
    local level=$1
    local format=$2
    
    echo "----------------------------------------"
    echo "Testing: LOG_LEVEL=$level LOG_FORMAT=$format"
    echo "----------------------------------------"
    
    # Export environment
    export LOG_LEVEL=$level
    export LOG_FORMAT=$format
    export LOG_DEV=false
    
    # Start API in background
    ./banking-api &
    API_PID=$!
    
    # Wait for startup
    sleep 3
    
    # Make some requests to generate logs
    echo "Making test requests..."
    
    # Create transaction
    TXID=$(curl -s -X POST http://localhost:8080/transactions \
        -H "Content-Type: application/json" \
        -d '{"account_id":"test123","amount":100.50,"type":"deposit"}' | \
        jq -r '.id')
    
    echo "Created transaction: $TXID"
    
    # Get transaction (should hit L1 cache)
    curl -s http://localhost:8080/transactions/$TXID > /dev/null
    echo "First GET - should be cache miss, then populate caches"
    
    # Get again (should hit cache)
    curl -s http://localhost:8080/transactions/$TXID > /dev/null
    echo "Second GET - should be cache hit on L1"
    
    # Wait a bit to see logs
    sleep 2
    
    # Kill API
    kill $API_PID 2>/dev/null
    wait $API_PID 2>/dev/null
    
    echo ""
}

# Build first
echo "Building banking-api..."
go build -o banking-api .
if [ $? -ne 0 ]; then
    echo "Build failed!"
    exit 1
fi

# Make sure services are running
echo "Checking if redis and postgres are running..."
if ! docker ps | grep -q banking-redis; then
    echo "Starting redis and postgres..."
    docker compose up -d redis postgres
    sleep 5
fi

echo ""
echo "Running tests with different log configurations..."
echo ""

# Test different configurations
test_log_level "info" "json"
test_log_level "debug" "json"
test_log_level "debug" "console"
test_log_level "warn" "console"

echo "=== All tests complete ==="
echo ""
echo "To run with Docker Compose:"
echo "  LOG_LEVEL=debug LOG_FORMAT=console docker compose up"
echo ""
echo "To view logs:"
echo "  docker logs banking-api"
echo ""
