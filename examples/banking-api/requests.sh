#!/bin/bash

BASE_URL="http://localhost:8080"

echo "========================================="
echo "Banking API - Test Requests"
echo "========================================="
echo ""

echo "1. Health Check"
echo "-----------------------------------------"
curl -s $BASE_URL/health | jq .
echo ""
echo ""

echo "2. Create Transaction #1 (Debit)"
echo "-----------------------------------------"
TX1=$(curl -s -X POST $BASE_URL/transactions \
  -H "Content-Type: application/json" \
  -d '{
    "account_id": "ACC001",
    "type": "debit",
    "amount": 150.50,
    "currency": "USD",
    "description": "Coffee shop payment"
  }' | jq -r '.id')
echo "Created transaction: $TX1"
echo ""

echo "3. Create Transaction #2 (Credit)"
echo "-----------------------------------------"
TX2=$(curl -s -X POST $BASE_URL/transactions \
  -H "Content-Type: application/json" \
  -d '{
    "account_id": "ACC001",
    "type": "credit",
    "amount": 1000.00,
    "currency": "USD",
    "description": "Salary deposit"
  }' | jq -r '.id')
echo "Created transaction: $TX2"
echo ""

echo "4. Get Transaction (First call - Cache MISS)"
echo "-----------------------------------------"
curl -s -i $BASE_URL/transactions/$TX1 | head -20
echo ""

echo "5. Get Transaction (Second call - Cache HIT)"
echo "-----------------------------------------"
curl -s -i $BASE_URL/transactions/$TX1 | head -20
echo ""

echo "6. List Account Transactions"
echo "-----------------------------------------"
curl -s "$BASE_URL/transactions?account_id=ACC001" | jq .
echo ""

echo "7. Get Transaction Again (Should be very fast)"
echo "-----------------------------------------"
time curl -s $BASE_URL/transactions/$TX2 > /dev/null
echo ""

echo "========================================="
echo "Test Complete!"
echo "========================================="
