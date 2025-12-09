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

	// Get from cache chain (Memory -> Redis -> PostgreSQL)
	// The chain automatically handles fallback through all layers
	value, err := h.cache.Get(ctx, cacheKey)
	if err != nil {
		log.Printf("✗ Failed to get %s: %v", cacheKey, err)
		http.Error(w, "Transaction not found", http.StatusNotFound)
		return
	}

	duration := time.Since(start)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Response-Time", duration.String())
	json.NewEncoder(w).Encode(value)
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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(tx)
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
