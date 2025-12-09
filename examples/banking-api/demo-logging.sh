#!/bin/bash

# Demo script to show different log levels in action

echo "=== Cache-Chain Logging Demo ==="
echo ""
echo "This demo shows how logs work at different levels"
echo ""

cd /Users/hamonvitorino/workspace/cache-chain/examples/banking-api

# Build
echo "Building..."
go build -o banking-api . 2>/dev/null

# Check if services are running
if ! docker ps | grep -q banking-redis; then
    echo "Starting redis and postgres..."
    docker compose up -d redis postgres
    sleep 5
fi

echo ""
echo "========================================="
echo "DEMO 1: INFO Level (Production Default)"
echo "========================================="
echo ""
export LOG_LEVEL=info
export LOG_FORMAT=console
./banking-api &
PID=$!
sleep 2

echo "Creating a transaction..."
TXID=$(curl -s -X POST http://localhost:8080/transactions \
    -H "Content-Type: application/json" \
    -d '{"account_id":"demo-account","amount":250.00,"type":"deposit"}' | \
    jq -r '.id')

echo "Transaction ID: $TXID"
sleep 1

echo "Getting transaction (first time - will populate caches)..."
curl -s http://localhost:8080/transactions/$TXID > /dev/null
sleep 1

echo "Getting transaction (second time - should hit L1 cache)..."
curl -s http://localhost:8080/transactions/$TXID > /dev/null
sleep 1

kill $PID 2>/dev/null
wait $PID 2>/dev/null
sleep 1

echo ""
echo "========================================="
echo "DEMO 2: DEBUG Level (Detailed)"
echo "========================================="
echo ""
export LOG_LEVEL=debug
export LOG_FORMAT=console
./banking-api &
PID=$!
sleep 2

echo "Getting transaction (should hit L1 cache - see detailed logs)..."
curl -s http://localhost:8080/transactions/$TXID > /dev/null
sleep 1

echo "Getting same transaction again..."
curl -s http://localhost:8080/transactions/$TXID > /dev/null
sleep 1

kill $PID 2>/dev/null
wait $PID 2>/dev/null
sleep 1

echo ""
echo "========================================="
echo "DEMO 3: JSON Format (Production)"
echo "========================================="
echo ""
export LOG_LEVEL=info
export LOG_FORMAT=json
./banking-api &
PID=$!
sleep 2

echo "Getting transaction with JSON logs..."
curl -s http://localhost:8080/transactions/$TXID > /dev/null
sleep 1

kill $PID 2>/dev/null
wait $PID 2>/dev/null

echo ""
echo "========================================="
echo "Demo Complete!"
echo "========================================="
echo ""
echo "Key Observations:"
echo "  1. INFO level shows initialization and important events"
echo "  2. DEBUG level shows every cache operation (hit/miss/set)"
echo "  3. JSON format is structured for log aggregation tools"
echo "  4. Console format is human-readable for development"
echo ""
echo "Try it yourself:"
echo "  LOG_LEVEL=debug LOG_FORMAT=console docker compose up"
echo ""
