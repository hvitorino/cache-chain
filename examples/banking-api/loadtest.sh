#!/bin/bash

# Load test script for Banking API
# Uses wrk to generate load and test cache performance

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
API_URL="${API_URL:-http://localhost:8080}"
DURATION="${DURATION:-30s}"
THREADS="${THREADS:-4}"
CONNECTIONS="${CONNECTIONS:-100}"

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  Banking API Load Test with wrk${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Check if wrk is installed
if ! command -v wrk &> /dev/null; then
    echo -e "${RED}✗ wrk is not installed${NC}"
    echo -e "${YELLOW}Install wrk:${NC}"
    echo "  macOS:   brew install wrk"
    echo "  Ubuntu:  sudo apt-get install wrk"
    echo "  Build:   git clone https://github.com/wg/wrk && cd wrk && make"
    exit 1
fi

# Check if API is running
echo -e "${YELLOW}→${NC} Checking if API is running at ${API_URL}..."
if ! curl -s -f "${API_URL}/health" > /dev/null; then
    echo -e "${RED}✗ API is not responding at ${API_URL}${NC}"
    echo -e "${YELLOW}Start the API with:${NC} make run"
    exit 1
fi
echo -e "${GREEN}✓${NC} API is running"
echo ""

# Prepare test data - create some transactions first
echo -e "${YELLOW}→${NC} Preparing test data..."
for i in {1..50}; do
    ACCOUNT_ID="ACC-$((RANDOM % 100 + 1))"
    AMOUNT=$((RANDOM % 1000 + 10))
    curl -s -X POST "${API_URL}/transactions" \
        -H "Content-Type: application/json" \
        -d "{
            \"account_id\": \"${ACCOUNT_ID}\",
            \"type\": \"debit\",
            \"amount\": ${AMOUNT}.00,
            \"currency\": \"USD\",
            \"description\": \"Test transaction ${i}\"
        }" > /dev/null
done
echo -e "${GREEN}✓${NC} Created 50 test transactions"
echo ""

# Display test configuration
echo -e "${BLUE}Test Configuration:${NC}"
echo "  URL:         ${API_URL}"
echo "  Duration:    ${DURATION}"
echo "  Threads:     ${THREADS}"
echo "  Connections: ${CONNECTIONS}"
echo "  Script:      loadtest.lua (20% writes, 80% reads)"
echo ""

# Warm up the cache
echo -e "${YELLOW}→${NC} Warming up cache..."
wrk -t2 -c10 -d5s -s loadtest.lua "${API_URL}" > /dev/null 2>&1 || true
echo -e "${GREEN}✓${NC} Cache warmed up"
echo ""

# Run the load test
echo -e "${YELLOW}→${NC} Starting load test..."
echo ""
wrk -t${THREADS} -c${CONNECTIONS} -d${DURATION} -s loadtest.lua "${API_URL}"

echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}✓ Load test completed${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "${YELLOW}Tips:${NC}"
echo "  • Check logs to see cache hit rates"
echo "  • Monitor Redis: redis-cli monitor"
echo "  • Adjust test params: THREADS=8 CONNECTIONS=200 DURATION=60s ./loadtest.sh"
echo ""
