#!/bin/bash

# Banking API Dashboard Setup Script
# Starts all services including Prometheus and Grafana for monitoring

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  Banking API - Monitoring Dashboard${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo -e "${RED}✗ Docker is not running${NC}"
    echo "  Please start Docker and try again"
    exit 1
fi

# Start services
echo -e "${YELLOW}→${NC} Starting services with docker-compose..."
docker-compose up -d

# Wait for services to be healthy
echo ""
echo -e "${YELLOW}→${NC} Waiting for services to be ready..."

# Wait for API
echo -n "  Waiting for API..."
for i in {1..30}; do
    if curl -s http://localhost:8080/health > /dev/null 2>&1; then
        echo -e " ${GREEN}✓${NC}"
        break
    fi
    echo -n "."
    sleep 1
done

# Wait for Prometheus
echo -n "  Waiting for Prometheus..."
for i in {1..30}; do
    if curl -s http://localhost:9090/-/healthy > /dev/null 2>&1; then
        echo -e " ${GREEN}✓${NC}"
        break
    fi
    echo -n "."
    sleep 1
done

# Wait for Grafana
echo -n "  Waiting for Grafana..."
for i in {1..30}; do
    if curl -s http://localhost:3000/api/health > /dev/null 2>&1; then
        echo -e " ${GREEN}✓${NC}"
        break
    fi
    echo -n "."
    sleep 1
done

echo ""
echo -e "${GREEN}✓ All services are running!${NC}"
echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  Service URLs${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "  ${GREEN}API:${NC}               http://localhost:8080"
echo -e "  ${GREEN}Grafana Dashboard:${NC} http://localhost:3000"
echo -e "    └─ User:     admin"
echo -e "    └─ Password: admin"
echo -e "  ${GREEN}Prometheus:${NC}        http://localhost:9090"
echo -e "  ${GREEN}Metrics:${NC}           http://localhost:8080/metrics"
echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  Docker Services${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
docker-compose ps
echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${YELLOW}Next Steps:${NC}"
echo ""
echo "  1. Open Grafana: http://localhost:3000"
echo "  2. Login with admin/admin"
echo "  3. Navigate to 'Banking API - Cache Chain Performance' dashboard"
echo "  4. Run load test: ./loadtest.sh"
echo "  5. Watch metrics update in real-time!"
echo ""
echo -e "${YELLOW}Useful Commands:${NC}"
echo "  View logs:     docker-compose logs -f api"
echo "  Stop services: docker-compose down"
echo "  Restart:       docker-compose restart"
echo ""
