package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cache-chain/examples/banking-api/models"
	"cache-chain/pkg/cache"

	_ "github.com/lib/pq"
)

// PostgresAdapter wraps PostgreSQL database and implements cache.CacheLayer for the API.
//
// IMPORTANT: This adapter acts as a READ-ONLY L3 cache layer.
// - Get(): Reads transaction data directly from the transactions table
// - Set(): No-op (L3 doesn't cache writes)
// - Delete(): No-op (L3 doesn't cache writes)
//
// New transactions are written via CreateTransaction() method directly to the database,
// and then cached in L1 (Memory) and L2 (Redis) by the application handlers.
type PostgresAdapter struct {
	db   *sql.DB
	name string
}

// Config holds PostgreSQL connection configuration.
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string
}

// DefaultConfig returns default PostgreSQL configuration.
func DefaultConfig() Config {
	return Config{
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Password: "postgres",
		Database: "banking_db",
		SSLMode:  "disable",
	}
}

// NewPostgresAdapter creates a new PostgreSQL adapter with connection pool.
func NewPostgresAdapter(cfg Config) (*PostgresAdapter, error) {
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, cfg.SSLMode,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres connection: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	adapter := &PostgresAdapter{
		db:   db,
		name: "PostgreSQL",
	}

	if err := adapter.initTables(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to init tables: %w", err)
	}

	return adapter, nil
}

func (p *PostgresAdapter) initTables(ctx context.Context) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS transactions (
			id TEXT PRIMARY KEY,
			account_id TEXT NOT NULL,
			type TEXT NOT NULL,
			amount NUMERIC(15,2) NOT NULL,
			currency TEXT NOT NULL,
			description TEXT,
			status TEXT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL,
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_transactions_account_id ON transactions(account_id)`,
		`CREATE INDEX IF NOT EXISTS idx_transactions_created_at ON transactions(created_at DESC)`,
	}

	for _, query := range queries {
		if _, err := p.db.ExecContext(ctx, query); err != nil {
			return err
		}
	}

	return nil
}

// Implement cache.CacheLayer interface (READ-ONLY)
// L3 PostgreSQL layer only reads transaction data from the database
// and serves as the source of truth. It does not cache arbitrary key-value pairs.

func (p *PostgresAdapter) Get(ctx context.Context, key string) (interface{}, error) {
	// Extract transaction ID from cache key format: "transaction:ID"
	if len(key) < 13 || key[:12] != "transaction:" {
		return nil, cache.ErrKeyNotFound
	}

	txID := key[12:]
	tx, err := p.GetTransaction(ctx, txID)
	if err != nil {
		return nil, cache.ErrKeyNotFound
	}

	return tx, nil
}

func (p *PostgresAdapter) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	// L3 is read-only for caching purposes
	// Writes go directly through CreateTransaction method
	// This is intentionally a no-op to satisfy the CacheLayer interface
	return nil
}

func (p *PostgresAdapter) Delete(ctx context.Context, key string) error {
	// L3 is read-only for caching purposes
	// This is intentionally a no-op to satisfy the CacheLayer interface
	return nil
}

func (p *PostgresAdapter) Name() string {
	return p.name
}

func (p *PostgresAdapter) Close() error {
	return p.db.Close()
}

// Transaction-specific methods (not part of cache interface)

func (p *PostgresAdapter) GetTransaction(ctx context.Context, id string) (*models.Transaction, error) {
	query := `
		SELECT id, account_id, type, amount, currency, description, status, created_at, updated_at
		FROM transactions WHERE id = $1
	`

	var t models.Transaction
	err := p.db.QueryRowContext(ctx, query, id).Scan(
		&t.ID, &t.AccountID, &t.Type, &t.Amount, &t.Currency,
		&t.Description, &t.Status, &t.CreatedAt, &t.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("transaction not found")
	}
	if err != nil {
		return nil, fmt.Errorf("query transaction: %w", err)
	}

	return &t, nil
}

func (p *PostgresAdapter) CreateTransaction(ctx context.Context, t *models.Transaction) error {
	query := `
		INSERT INTO transactions (id, account_id, type, amount, currency, description, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := p.db.ExecContext(ctx, query,
		t.ID, t.AccountID, t.Type, t.Amount, t.Currency,
		t.Description, t.Status, t.CreatedAt, t.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create transaction: %w", err)
	}

	return nil
}

func (p *PostgresAdapter) ListTransactions(ctx context.Context, accountID string, limit int) ([]*models.Transaction, error) {
	query := `
		SELECT id, account_id, type, amount, currency, description, status, created_at, updated_at
		FROM transactions 
		WHERE account_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := p.db.QueryContext(ctx, query, accountID, limit)
	if err != nil {
		return nil, fmt.Errorf("query transactions: %w", err)
	}
	defer rows.Close()

	var transactions []*models.Transaction
	for rows.Next() {
		var t models.Transaction
		if err := rows.Scan(
			&t.ID, &t.AccountID, &t.Type, &t.Amount, &t.Currency,
			&t.Description, &t.Status, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan transaction: %w", err)
		}
		transactions = append(transactions, &t)
	}

	return transactions, nil
}
