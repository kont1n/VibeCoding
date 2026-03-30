package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kont1n/face-grouper/internal/model"
)

// SessionRepository implements repository pattern for processing sessions.
type SessionRepository struct {
	pool *pgxpool.Pool
}

// NewSessionRepository creates a new SessionRepository.
func NewSessionRepository(pool *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{pool: pool}
}

// Create creates a new processing session.
func (r *SessionRepository) Create(ctx context.Context, session *model.ProcessingSession) error {
	query := `
		INSERT INTO processing_sessions (id, status, stage, progress, total_items, processed_items, 
		                                  errors, error_details, config, started_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), $10)
	`

	errorDetailsJSON, err := json.Marshal(session.ErrorDetails)
	if err != nil {
		return fmt.Errorf("marshal error details: %w", err)
	}
	configJSON, err := json.Marshal(session.Config)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	_, err = r.pool.Exec(ctx, query,
		session.ID,
		session.Status,
		nullStringToSql(session.Stage),
		session.Progress,
		session.TotalItems,
		session.ProcessedItems,
		session.Errors,
		errorDetailsJSON,
		configJSON,
		uuidFromSql(session.CreatedBy),
	)

	return err
}

// GetByID retrieves a session by ID.
func (r *SessionRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.ProcessingSession, error) {
	query := `
		SELECT id, status, stage, progress, total_items, processed_items, errors, 
		       error_details, config, started_at, completed_at, created_by
		FROM processing_sessions WHERE id = $1
	`

	session := &model.ProcessingSession{}
	var errorDetailsJSON, configJSON []byte

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&session.ID,
		&session.Status,
		&session.Stage,
		&session.Progress,
		&session.TotalItems,
		&session.ProcessedItems,
		&session.Errors,
		&errorDetailsJSON,
		&configJSON,
		&session.StartedAt,
		&session.CompletedAt,
		&session.CreatedBy,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(errorDetailsJSON, &session.ErrorDetails); err != nil {
		return nil, err
	}

	if err := json.Unmarshal(configJSON, &session.Config); err != nil {
		return nil, err
	}

	return session, nil
}

// GetActive retrieves the currently active processing session.
func (r *SessionRepository) GetActive(ctx context.Context) (*model.ProcessingSession, error) {
	query := `
		SELECT id, status, stage, progress, total_items, processed_items, errors, 
		       error_details, config, started_at, completed_at, created_by
		FROM processing_sessions 
		WHERE status = 'processing'
		ORDER BY started_at DESC LIMIT 1
	`

	session := &model.ProcessingSession{}
	var errorDetailsJSON, configJSON []byte

	err := r.pool.QueryRow(ctx, query).Scan(
		&session.ID,
		&session.Status,
		&session.Stage,
		&session.Progress,
		&session.TotalItems,
		&session.ProcessedItems,
		&session.Errors,
		&errorDetailsJSON,
		&configJSON,
		&session.StartedAt,
		&session.CompletedAt,
		&session.CreatedBy,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(errorDetailsJSON, &session.ErrorDetails); err != nil {
		return nil, err
	}

	if err := json.Unmarshal(configJSON, &session.Config); err != nil {
		return nil, err
	}

	return session, nil
}

// Update updates a processing session.
func (r *SessionRepository) Update(ctx context.Context, session *model.ProcessingSession) error {
	query := `
		UPDATE processing_sessions
		SET status = $2, stage = $3, progress = $4, processed_items = $5, errors = $6,
		    error_details = $7, config = $8,
		    completed_at = CASE WHEN $2 = 'completed' OR $2 = 'failed' THEN NOW() ELSE completed_at END
		WHERE id = $1
	`

	errorDetailsJSON, err := json.Marshal(session.ErrorDetails)
	if err != nil {
		return fmt.Errorf("marshal error details: %w", err)
	}
	configJSON, err := json.Marshal(session.Config)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	_, err = r.pool.Exec(ctx, query,
		session.ID,
		session.Status,
		nullStringToSql(session.Stage),
		session.Progress,
		session.ProcessedItems,
		session.Errors,
		errorDetailsJSON,
		configJSON,
	)

	return err
}

// UpdateProgress updates only the progress fields.
func (r *SessionRepository) UpdateProgress(ctx context.Context, id uuid.UUID, progress float32, processedItems, errorCount int) error {
	query := `
		UPDATE processing_sessions
		SET progress = $2, processed_items = $3, errors = $4
		WHERE id = $1
	`

	_, err := r.pool.Exec(ctx, query, id, progress, processedItems, errorCount)
	return err
}

// List retrieves sessions with pagination.
func (r *SessionRepository) List(ctx context.Context, offset, limit int) ([]*model.ProcessingSession, error) {
	query := `
		SELECT id, status, stage, progress, total_items, processed_items, errors, 
		       error_details, config, started_at, completed_at, created_by
		FROM processing_sessions
		ORDER BY created_at DESC
		OFFSET $1 LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, offset, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*model.ProcessingSession
	for rows.Next() {
		session := &model.ProcessingSession{}
		var errorDetailsJSON, configJSON []byte

		err := rows.Scan(
			&session.ID,
			&session.Status,
			&session.Stage,
			&session.Progress,
			&session.TotalItems,
			&session.ProcessedItems,
			&session.Errors,
			&errorDetailsJSON,
			&configJSON,
			&session.StartedAt,
			&session.CompletedAt,
			&session.CreatedBy,
		)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(errorDetailsJSON, &session.ErrorDetails); err != nil {
			return nil, err
		}

		if err := json.Unmarshal(configJSON, &session.Config); err != nil {
			return nil, err
		}

		sessions = append(sessions, session)
	}

	return sessions, nil
}

// DeleteOld deletes sessions older than the specified duration.
func (r *SessionRepository) DeleteOld(ctx context.Context, olderThan time.Duration) error {
	query := `DELETE FROM processing_sessions WHERE created_at < NOW() - $1`
	_, err := r.pool.Exec(ctx, query, olderThan)
	return err
}

// GetStats retrieves processing statistics for the last 7 days.
func (r *SessionRepository) GetStats(ctx context.Context) (*model.ProcessingStats, error) {
	query := `
		SELECT 
		    COUNT(*) as total_sessions,
		    COUNT(*) FILTER (WHERE status = 'completed') as completed,
		    COUNT(*) FILTER (WHERE status = 'failed') as failed,
		    COUNT(*) FILTER (WHERE status = 'processing') as processing,
		    AVG(EXTRACT(EPOCH FROM (completed_at - started_at))) FILTER (WHERE status = 'completed') as avg_duration
		FROM processing_sessions
		WHERE created_at > NOW() - INTERVAL '7 days'
	`

	stats := &model.ProcessingStats{}
	var avgDuration sql.NullFloat64

	err := r.pool.QueryRow(ctx, query).Scan(
		&stats.TotalSessions,
		&stats.Completed,
		&stats.Failed,
		&stats.Processing,
		&avgDuration,
	)

	if err != nil {
		return nil, err
	}

	if avgDuration.Valid {
		stats.AvgDurationSeconds = float32(avgDuration.Float64)
	}

	return stats, nil
}
