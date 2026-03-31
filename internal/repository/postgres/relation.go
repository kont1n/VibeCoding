// Package postgres provides PostgreSQL repository implementations.
package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kont1n/face-grouper/internal/model"
)

// RelationRepositoryImpl implements RelationRepository interface.
type RelationRepositoryImpl struct {
	pool *pgxpool.Pool
}

// NewRelationRepository creates a new relation repository.
func NewRelationRepository(pool *pgxpool.Pool) *RelationRepositoryImpl {
	return &RelationRepositoryImpl{pool: pool}
}

// Create creates a new person relation.
func (r *RelationRepositoryImpl) Create(ctx context.Context, relation *model.PersonRelation) error {
	query := `
		INSERT INTO person_relations (person1_id, person2_id, similarity, created_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (person1_id, person2_id) DO UPDATE
		SET similarity = $3
	`

	_, err := r.pool.Exec(ctx, query,
		relation.Person1ID,
		relation.Person2ID,
		relation.Similarity,
		relation.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("create relation: %w", err)
	}

	return nil
}

// CreateBatch creates multiple relations in a single transaction.
func (r *RelationRepositoryImpl) CreateBatch(ctx context.Context, relations []*model.PersonRelation) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	query := `
		INSERT INTO person_relations (person1_id, person2_id, similarity, created_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (person1_id, person2_id) DO UPDATE
		SET similarity = $3
	`

	for _, relation := range relations {
		_, err := tx.Exec(ctx, query,
			relation.Person1ID,
			relation.Person2ID,
			relation.Similarity,
			relation.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("create relation: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// GetByPersonID returns all relations for a person.
func (r *RelationRepositoryImpl) GetByPersonID(ctx context.Context, personID uuid.UUID) ([]*model.PersonRelation, error) {
	query := `
		SELECT person1_id, person2_id, similarity, created_at
		FROM person_relations
		WHERE person1_id = $1 OR person2_id = $1
		ORDER BY similarity DESC
	`

	rows, err := r.pool.Query(ctx, query, personID)
	if err != nil {
		return nil, fmt.Errorf("get relations by person id: %w", err)
	}
	defer rows.Close()

	var relations []*model.PersonRelation
	for rows.Next() {
		relation := &model.PersonRelation{}
		err := rows.Scan(
			&relation.Person1ID,
			&relation.Person2ID,
			&relation.Similarity,
			&relation.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan relation: %w", err)
		}
		relations = append(relations, relation)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate relations: %w", err)
	}

	return relations, nil
}

// GetByPersonIDWithMinSimilarity returns relations for a person with minimum similarity filter.
func (r *RelationRepositoryImpl) GetByPersonIDWithMinSimilarity(ctx context.Context, personID uuid.UUID, minSimilarity float32) ([]*model.PersonRelation, error) {
	query := `
		SELECT person1_id, person2_id, similarity, created_at
		FROM person_relations
		WHERE (person1_id = $1 OR person2_id = $1)
		  AND similarity >= $2
		ORDER BY similarity DESC
	`

	rows, err := r.pool.Query(ctx, query, personID, minSimilarity)
	if err != nil {
		return nil, fmt.Errorf("get relations by person id: %w", err)
	}
	defer rows.Close()

	var relations []*model.PersonRelation
	for rows.Next() {
		relation := &model.PersonRelation{}
		err := rows.Scan(
			&relation.Person1ID,
			&relation.Person2ID,
			&relation.Similarity,
			&relation.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan relation: %w", err)
		}
		relations = append(relations, relation)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate relations: %w", err)
	}

	return relations, nil
}

// GetGraph returns person nodes for graph visualization.
func (r *RelationRepositoryImpl) GetGraph(ctx context.Context, personIDs []uuid.UUID, minSimilarity float32) ([]model.PersonNode, error) {
	if len(personIDs) == 0 {
		return nil, nil
	}

	// Get persons with their stats.
	query := `
		SELECT p.id, p.name, p.custom_name, p.avatar_path, p.face_count, p.photo_count
		FROM persons p
		WHERE p.id = ANY($1)
	`

	rows, err := r.pool.Query(ctx, query, personIDs)
	if err != nil {
		return nil, fmt.Errorf("get graph persons: %w", err)
	}
	defer rows.Close()

	nodes := make([]model.PersonNode, 0, len(personIDs))
	for rows.Next() {
		node := model.PersonNode{}
		err := rows.Scan(
			&node.ID,
			&node.Name,
			&node.CustomName,
			&node.AvatarPath,
			&node.FaceCount,
			&node.PhotoCount,
		)
		if err != nil {
			return nil, fmt.Errorf("scan graph node: %w", err)
		}
		nodes = append(nodes, node)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate graph nodes: %w", err)
	}

	return nodes, nil
}

// GetAll returns all relations.
func (r *RelationRepositoryImpl) GetAll(ctx context.Context) ([]*model.PersonRelation, error) {
	query := `
		SELECT person1_id, person2_id, similarity, created_at
		FROM person_relations
		ORDER BY similarity DESC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("get all relations: %w", err)
	}
	defer rows.Close()

	var relations []*model.PersonRelation
	for rows.Next() {
		relation := &model.PersonRelation{}
		err := rows.Scan(
			&relation.Person1ID,
			&relation.Person2ID,
			&relation.Similarity,
			&relation.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan relation: %w", err)
		}
		relations = append(relations, relation)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate relations: %w", err)
	}

	return relations, nil
}

// Delete deletes a relation.
func (r *RelationRepositoryImpl) Delete(ctx context.Context, person1ID, person2ID uuid.UUID) error {
	query := `DELETE FROM person_relations WHERE person1_id = $1 AND person2_id = $2`

	_, err := r.pool.Exec(ctx, query, person1ID, person2ID)
	if err != nil {
		return fmt.Errorf("delete relation: %w", err)
	}

	return nil
}

// DeleteByPersonID deletes all relations for a person.
func (r *RelationRepositoryImpl) DeleteByPersonID(ctx context.Context, personID uuid.UUID) error {
	query := `DELETE FROM person_relations WHERE person1_id = $1 OR person2_id = $1`

	_, err := r.pool.Exec(ctx, query, personID)
	if err != nil {
		return fmt.Errorf("delete relations by person id: %w", err)
	}

	return nil
}
