package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Application is a tenant (e.g. uPayment, uCuan, uKasir).
type Application struct {
	ID        string    `json:"id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Type      string    `json:"type"` // web | mobile | backend
	APIKey    string    `json:"api_key"`
	SecretKey string    `json:"secret_key,omitempty"` // returned only on create/rotate
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// ErrNotFound is returned when no application matches the lookup.
var ErrNotFound = errors.New("application not found")

// Repository persists and queries applications.
type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Create inserts a new application and returns it with freshly-generated keys.
func (r *Repository) Create(ctx context.Context, code, name, typ string) (*Application, error) {
	apiKey, err := randomToken("ak_", 24)
	if err != nil {
		return nil, err
	}
	secret, err := randomToken("sk_", 48)
	if err != nil {
		return nil, err
	}
	app := &Application{
		ID:        uuid.NewString(),
		Code:      code,
		Name:      name,
		Type:      typ,
		APIKey:    apiKey,
		SecretKey: secret,
		Status:    "active",
		CreatedAt: time.Now().UTC(),
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO applications (id, code, name, type, api_key, secret_key, status, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`, app.ID, app.Code, app.Name, app.Type, app.APIKey, app.SecretKey, app.Status, app.CreatedAt)
	if err != nil {
		return nil, err
	}
	return app, nil
}

// GetByAPIKey returns the application owning the given api_key.
func (r *Repository) GetByAPIKey(ctx context.Context, apiKey string) (*Application, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, code, name, type, api_key, secret_key, status, created_at
		FROM applications WHERE api_key = $1 AND status = 'active'
	`, apiKey)
	a := &Application{}
	err := row.Scan(&a.ID, &a.Code, &a.Name, &a.Type, &a.APIKey, &a.SecretKey, &a.Status, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return a, err
}

// List returns all applications (secrets omitted).
func (r *Repository) List(ctx context.Context) ([]Application, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, code, name, type, api_key, status, created_at
		FROM applications ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Application
	for rows.Next() {
		a := Application{}
		if err := rows.Scan(&a.ID, &a.Code, &a.Name, &a.Type, &a.APIKey, &a.Status, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// RotateKey generates a new api_key + secret_key for the given application.
func (r *Repository) RotateKey(ctx context.Context, id string) (*Application, error) {
	apiKey, err := randomToken("ak_", 24)
	if err != nil {
		return nil, err
	}
	secret, err := randomToken("sk_", 48)
	if err != nil {
		return nil, err
	}
	row := r.pool.QueryRow(ctx, `
		UPDATE applications SET api_key = $2, secret_key = $3
		WHERE id = $1
		RETURNING id, code, name, type, api_key, secret_key, status, created_at
	`, id, apiKey, secret)
	a := &Application{}
	err = row.Scan(&a.ID, &a.Code, &a.Name, &a.Type, &a.APIKey, &a.SecretKey, &a.Status, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return a, err
}

// Revoke marks an application as revoked (soft delete).
func (r *Repository) Revoke(ctx context.Context, id string) error {
	cmd, err := r.pool.Exec(ctx, `UPDATE applications SET status = 'revoked' WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func randomToken(prefix string, byteLen int) (string, error) {
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(buf), nil
}
