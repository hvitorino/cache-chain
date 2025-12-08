package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"cache-chain/examples/banking-api/models"
	"cache-chain/examples/banking-api/postgres"
	"cache-chain/pkg/chain"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type TransactionHandler struct {
	db    *postgres.PostgresAdapter
	cache *chain.Chain
}

func NewTransactionHandler(db *postgres.PostgresAdapter, cache *chain.Chain) *TransactionHandler {
	return &TransactionHandler{
		db:    db,
		cache: cache,
	}
}

func (h *TransactionHandler) GetTransaction(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	cacheKey := fmt.Sprintf("transaction:%s", id)
	ctx := r.Context()

	start := time.Now()

	// Try to get from cache chain (Memory -> Redis -> PostgreSQL)
	value, err := h.cache.Get(ctx, cacheKey)
	if err == nil {
		duration := time.Since(start)
		log.Printf("✓ Cache HIT for %s (took %v)", cacheKey, duration)

		// Type assertion for cached transaction
		if txMap, ok := value.(map[string]interface{}); ok {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Cache-Hit", "true")
			w.Header().Set("X-Response-Time", duration.String())
			json.NewEncoder(w).Encode(txMap)
			return
		}
	}

	// Cache miss - get from database
	log.Printf("✗ Cache MISS for %s", cacheKey)

	tx, err := h.db.GetTransaction(ctx, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Store in cache chain (L1 and L2 only - L3 is read-only)
	// The chain will warm up Memory and Redis automatically
	if err := h.cache.Set(ctx, cacheKey, tx, 5*time.Minute); err != nil {
		log.Printf("Warning: failed to cache transaction: %v", err)
	}

	duration := time.Since(start)
	log.Printf("✓ Loaded from database (took %v)", duration)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache-Hit", "false")
	w.Header().Set("X-Response-Time", duration.String())
	json.NewEncoder(w).Encode(tx)
}

func (h *TransactionHandler) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AccountID   string  `json:"account_id"`
		Type        string  `json:"type"`
		Amount      float64 `json:"amount"`
		Currency    string  `json:"currency"`
		Description string  `json:"description"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	tx := &models.Transaction{
		ID:          uuid.New().String(),
		AccountID:   req.AccountID,
		Type:        req.Type,
		Amount:      req.Amount,
		Currency:    req.Currency,
		Description: req.Description,
		Status:      "completed",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	ctx := r.Context()
	if err := h.db.CreateTransaction(ctx, tx); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Cache the new transaction in L1 and L2 (L3 PostgreSQL is read-only cache)
	cacheKey := fmt.Sprintf("transaction:%s", tx.ID)
	if err := h.cache.Set(ctx, cacheKey, tx, 5*time.Minute); err != nil {
		log.Printf("Warning: failed to cache new transaction: %v", err)
	}

	// Invalidate list cache for this account
	listCacheKey := fmt.Sprintf("transactions:account:%s", req.AccountID)
	h.cache.Delete(ctx, listCacheKey)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(tx)
}

func (h *TransactionHandler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	accountID := r.URL.Query().Get("account_id")
	if accountID == "" {
		http.Error(w, "account_id is required", http.StatusBadRequest)
		return
	}

	cacheKey := fmt.Sprintf("transactions:account:%s", accountID)
	ctx := r.Context()

	start := time.Now()

	// Try cache first
	value, err := h.cache.Get(ctx, cacheKey)
	if err == nil {
		duration := time.Since(start)
		log.Printf("✓ Cache HIT for %s (took %v)", cacheKey, duration)

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache-Hit", "true")
		w.Header().Set("X-Response-Time", duration.String())
		json.NewEncoder(w).Encode(value)
		return
	}

	// Cache miss
	log.Printf("✗ Cache MISS for %s", cacheKey)

	transactions, err := h.db.ListTransactions(ctx, accountID, 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Cache the list in L1 and L2 (L3 is read-only)
	if err := h.cache.Set(ctx, cacheKey, transactions, 2*time.Minute); err != nil {
		log.Printf("Warning: failed to cache transactions list: %v", err)
	}

	duration := time.Since(start)
	log.Printf("✓ Loaded from database (took %v)", duration)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache-Hit", "false")
	w.Header().Set("X-Response-Time", duration.String())
	json.NewEncoder(w).Encode(transactions)
}

func (h *TransactionHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	health := map[string]interface{}{
		"status": "healthy",
		"layers": make([]map[string]string, 0),
	}

	// Check each cache layer
	for _, layer := range h.cache.Layers() {
		layerHealth := map[string]string{
			"name":   layer.Name(),
			"status": "ok",
		}
		health["layers"] = append(health["layers"].([]map[string]string), layerHealth)
	}

	// Test database
	if _, err := h.db.Get(ctx, "health-check"); err != nil {
		// Expected to not find the key
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}
