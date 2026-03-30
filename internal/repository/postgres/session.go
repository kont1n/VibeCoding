// Package postgres provides PostgreSQL repository implementations.
package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kont1n/face-grouper/internal/model"
)

// SessionRepository provides database operations for processing sessions.
type SessionRepository struct {
	pool *pgxpool.Pool
}

// NewSessionRepository creates a new session repository.
func NewSessionRepository(pool *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{pool: pool}
}

// Create creates a new processing session.
func (r *SessionRepository) Create(ctx context.Context, session *model.ProcessingSession) error {
	query := `
		INSERT INTO processing_sessions (id, status, stage, progress, total_items, processed_items,
		                                  errors, error_details, config, started_at, completed_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	var errorDetails json.RawMessage
	if len(session.ErrorDetails) > 0 {
		var err error
		errorDetails, err = json.Marshal(session.ErrorDetails)
		if err != nil {
			return fmt.Errorf("marshal error details: %w", err)
		}
	}

	_, err := r.pool.Exec(ctx, query,
		session.ID,
		session.Status,
		session.Stage,
		session.Progress,
		session.TotalItems,
		session.ProcessedItems,
		session.Errors,
		errorDetails,
		session.Config,
		session.StartedAt,
		session.CompletedAt,
		session.CreatedBy,
	)

	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	return nil
}

// GetByID returns a session by ID.
func (r *SessionRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.ProcessingSession, error) {
	query := `
		SELECT id, status, stage, progress, total_items, processed_items, errors,
		       error_details, config, started_at, completed_at, created_by
		FROM processing_sessions
		WHERE id = $1
	`

	session := &model.ProcessingSession{}
	var errorDetails json.RawMessage
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&session.ID,
		&session.Status,
		&session.Stage,
		&session.Progress,
		&session.TotalItems,
		&session.ProcessedItems,
		&session.Errors,
		&errorDetails,
		&session.Config,
		&session.StartedAt,
		&session.CompletedAt,
		&session.CreatedBy,
	)

	if err != nil {
		return nil, fmt.Errorf("get session by id: %w", err)
	}

	if len(errorDetails) > 0 {
		var details []model.ErrorDetail
		if err := json.Unmarshal(errorDetails, &details); err == nil {
			session.ErrorDetails = details
		}
	}

	return session, nil
}

// GetAll returns all sessions.
func (r *SessionRepository) GetAll(ctx context.Context) ([]*model.ProcessingSession, error) {
	query := `
		SELECT id, status, stage, progress, total_items, processed_items, errors,
		       error_details, config, started_at, completed_at, created_by
		FROM processing_sessions
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("get all sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*model.ProcessingSession
	for rows.Next() {
		session := &model.ProcessingSession{}
		var errorDetails json.RawMessage
		err := rows.Scan(
			&session.ID,
			&session.Status,
			&session.Stage,
			&session.Progress,
			&session.TotalItems,
			&session.ProcessedItems,
			&session.Errors,
			&errorDetails,
			&session.Config,
			&session.StartedAt,
			&session.CompletedAt,
			&session.CreatedBy,
		)
		if err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		if len(errorDetails) > 0 {
			var details []model.ErrorDetail
			if err := json.Unmarshal(errorDetails, &details); err == nil {
				session.ErrorDetails = details
			}
		}
		sessions = append(sessions, session)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sessions: %w", err)
	}

	return sessions, nil
}

// UpdateStatus updates session status and progress.
func (r *SessionRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status, stage string, progress float32, processedItems int) error {
	query := `
		UPDATE processing_sessions
		SET status = $2, stage = $3, progress = $4, processed_items = $5
		WHERE id = $1
	`

	_, err := r.pool.Exec(ctx, query, id, status, stage, progress, processedItems)
	if err != nil {
		return fmt.Errorf("update session status: %w", err)
	}

	return nil
}

// Complete marks a session as completed.
func (r *SessionRepository) Complete(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE processing_sessions
		SET status = 'completed', completed_at = $2
		WHERE id = $1
	`

	_, err := r.pool.Exec(ctx, query, id, time.Now())
	if err != nil {
		return fmt.Errorf("complete session: %w", err)
	}

	return nil
}

// Fail marks a session as failed with errors.
func (r *SessionRepository) Fail(ctx context.Context, id uuid.UUID, errors int, errorDetails []model.ErrorDetail) error {
	query := `
		UPDATE processing_sessions
		SET status = 'failed', errors = $2, error_details = $3, completed_at = $4
		WHERE id = $1
	`

	errorDetailsJSON, err := json.Marshal(errorDetails)
	if err != nil {
		return fmt.Errorf("marshal error details: %w", err)
	}

	_, err = r.pool.Exec(ctx, query, id, errors, errorDetailsJSON, time.Now())
	if err != nil {
		return fmt.Errorf("fail session: %w", err)
	}

	return nil
}

// Delete deletes a session.
func (r *SessionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM processing_sessions WHERE id = $1`

	_, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	return nil
}

// GetStats returns processing statistics.
func (r *SessionRepository) GetStats(ctx context.Context) (*model.ProcessingStats, error) {
	query := `
		SELECT 
			COUNT(*) as total_sessions,
			COUNT(*) FILTER (WHERE status = 'completed') as completed,
			COUNT(*) FILTER (WHERE status = 'failed') as failed,
			COUNT(*) FILTER (WHERE status = 'processing') as processing,
			COALESCE(AVG(EXTRACT(EPOCH FROM (completed_at - started_at))), 0) as avg_duration
		FROM processing_sessions
		WHERE completed_at IS NOT NULL
	`

	stats := &model.ProcessingStats{}
	err := r.pool.QueryRow(ctx, query).Scan(
		&stats.TotalSessions,
		&stats.Completed,
		&stats.Failed,
		&stats.Processing,
		&stats.AvgDurationSeconds,
	)

	if err != nil {
		return nil, fmt.Errorf("get stats: %w", err)
	}

	return stats, nil
}

// GetActive returns the currently active (processing) session.
func (r *SessionRepository) GetActive(ctx context.Context) (*model.ProcessingSession, error) {
	query := `
		SELECT id, status, stage, progress, total_items, processed_items, errors,
		       error_details, config, started_at, completed_at, created_by
		FROM processing_sessions
		WHERE status = 'processing'
		ORDER BY started_at DESC
		LIMIT 1
	`

	session := &model.ProcessingSession{}
	var errorDetails json.RawMessage
	err := r.pool.QueryRow(ctx, query).Scan(
		&session.ID,
		&session.Status,
		&session.Stage,
		&session.Progress,
		&session.TotalItems,
		&session.ProcessedItems,
		&session.Errors,
		&errorDetails,
		&session.Config,
		&session.StartedAt,
		&session.CompletedAt,
		&session.CreatedBy,
	)

	if err != nil {
		return nil, fmt.Errorf("get active session: %w", err)
	}

	if len(errorDetails) > 0 {
		var details []model.ErrorDetail
		if err := json.Unmarshal(errorDetails, &details); err == nil {
			session.ErrorDetails = details
		}
	}

	return session, nil
}
