package event

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRepo persists audit & error events.
type PostgresRepo struct {
	pool *pgxpool.Pool
}

func NewPostgresRepo(pool *pgxpool.Pool) *PostgresRepo {
	return &PostgresRepo{pool: pool}
}

// InsertAudit writes an audit event. Idempotent on event_id.
func (r *PostgresRepo) InsertAudit(ctx context.Context, e *Event) error {
	actor, _ := json.Marshal(e.Actor)
	resource, _ := json.Marshal(e.Resource)
	tags, _ := json.Marshal(e.Tags)
	ctxBytes, _ := json.Marshal(e.Context)

	_, err := r.pool.Exec(ctx, `
		INSERT INTO audit_events
		    (event_id, application_id, timestamp, action, severity, status,
		     actor, resource, payload, context, tags)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (event_id) DO NOTHING
	`, e.EventID, e.ApplicationID, e.Timestamp, e.Action, e.Severity, e.Status,
		actor, resource, e.Payload, ctxBytes, tags)
	return err
}

// InsertError writes an error event.
func (r *PostgresRepo) InsertError(ctx context.Context, e *Event, fingerprint string) error {
	actor, _ := json.Marshal(e.Actor)
	ctxBytes, _ := json.Marshal(e.Context)
	tags, _ := json.Marshal(e.Tags)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO error_events
		    (event_id, application_id, timestamp, action, severity, fingerprint,
		     actor, payload, context, tags)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (event_id) DO NOTHING
	`, e.EventID, e.ApplicationID, e.Timestamp, e.Action, e.Severity, fingerprint,
		actor, e.Payload, ctxBytes, tags)
	return err
}

// QueryFilter defines filters for listing audit events.
type QueryFilter struct {
	ApplicationID string
	ActorID       string
	Action        string
	ResourceType  string
	ResourceID    string
	Status        string
	FromTime      string
	ToTime        string
	AfterID       int64
	Limit         int
}

// ListAudit returns audit events matching filter (cursor pagination by id).
func (r *PostgresRepo) ListAudit(ctx context.Context, f QueryFilter) ([]map[string]any, error) {
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, event_id, application_id, timestamp, action, severity, status,
		       actor, resource, payload, context, tags
		FROM audit_events
		WHERE ($1::uuid IS NULL OR application_id = $1)
		  AND ($2::text IS NULL OR actor->>'id' = $2)
		  AND ($3::text IS NULL OR action = $3)
		  AND ($4::text IS NULL OR resource->>'type' = $4)
		  AND ($5::text IS NULL OR resource->>'id' = $5)
		  AND ($6::text IS NULL OR status = $6)
		  AND ($7::timestamptz IS NULL OR timestamp >= $7)
		  AND ($8::timestamptz IS NULL OR timestamp <= $8)
		  AND ($9::bigint IS NULL OR id > $9)
		ORDER BY id ASC
		LIMIT $10
	`, nullable(f.ApplicationID), nullable(f.ActorID), nullable(f.Action),
		nullable(f.ResourceType), nullable(f.ResourceID), nullable(f.Status),
		nullable(f.FromTime), nullable(f.ToTime), nullableInt(f.AfterID), f.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []map[string]any
	for rows.Next() {
		var (
			id                                      int64
			eventID                                 string
			appID                                   string
			ts                                      any
			action                                  string
			severity                                string
			status                                  *string
			actor, resource, payload, ctxJSON, tags []byte
		)
		if err := rows.Scan(&id, &eventID, &appID, &ts, &action, &severity, &status,
			&actor, &resource, &payload, &ctxJSON, &tags); err != nil {
			return nil, err
		}
		m := map[string]any{
			"id":             id,
			"event_id":       eventID,
			"application_id": appID,
			"timestamp":      ts,
			"action":         action,
			"severity":       severity,
			"status":         status,
			"actor":          rawJSON(actor),
			"resource":       rawJSON(resource),
			"payload":        rawJSON(payload),
			"context":        rawJSON(ctxJSON),
			"tags":           rawJSON(tags),
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullableInt(n int64) any {
	if n == 0 {
		return nil
	}
	return n
}

func rawJSON(b []byte) json.RawMessage {
	if len(b) == 0 {
		return nil
	}
	return json.RawMessage(b)
}
