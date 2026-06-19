package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(database *sql.DB) *Repository {
	return &Repository{db: database}
}

func (r *Repository) Insert(ctx context.Context, event Event) error {
	metadata := event.Metadata
	if metadata == nil {
		metadata = map[string]string{}
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	var actorUserID any
	if event.ActorUserID != nil {
		actorUserID = *event.ActorUserID
	}

	_, err = r.db.ExecContext(
		ctx,
		`INSERT INTO audit_events (
			event_type, outcome, actor_user_id, actor_username, actor_role,
			target_type, target_id, reason_kind, request_id, remote_ip, metadata_json, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11::jsonb, $12)`,
		event.EventType,
		event.Outcome,
		actorUserID,
		event.ActorUsername,
		event.ActorRole,
		event.TargetType,
		event.TargetID,
		event.ReasonKind,
		event.RequestID,
		event.RemoteIP,
		string(encoded),
		event.CreatedAt,
	)
	return err
}

func (r *Repository) List(ctx context.Context, limit int) ([]Event, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT
			id, event_type, outcome, actor_user_id, actor_username, actor_role,
			target_type, target_id, reason_kind, request_id, remote_ip, metadata_json::text, created_at
		 FROM audit_events
		 ORDER BY created_at DESC, id DESC
		 LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []Event{}
	for rows.Next() {
		event := Event{}
		actorUserID := sql.NullInt64{}
		metadata := "{}"
		if err := rows.Scan(
			&event.ID,
			&event.EventType,
			&event.Outcome,
			&actorUserID,
			&event.ActorUsername,
			&event.ActorRole,
			&event.TargetType,
			&event.TargetID,
			&event.ReasonKind,
			&event.RequestID,
			&event.RemoteIP,
			&metadata,
			&event.CreatedAt,
		); err != nil {
			return nil, err
		}
		if actorUserID.Valid {
			id := actorUserID.Int64
			event.ActorUserID = &id
		}
		if err := json.Unmarshal([]byte(metadata), &event.Metadata); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (r *Repository) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	result, err := r.db.ExecContext(ctx, `DELETE FROM audit_events WHERE created_at < $1`, cutoff.UTC().Format(time.RFC3339))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
