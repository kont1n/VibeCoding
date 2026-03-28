package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kont1n/face-grouper/internal/model"
)

// RelationRepository implements repository pattern for person relations.
type RelationRepository struct {
	pool *pgxpool.Pool
}

// NewRelationRepository creates a new RelationRepository.
func NewRelationRepository(pool *pgxpool.Pool) *RelationRepository {
	return &RelationRepository{pool: pool}
}

// Create creates a new person relation.
func (r *RelationRepository) Create(ctx context.Context, relation *model.PersonRelation) error {
	query := `
		INSERT INTO person_relations (person1_id, person2_id, similarity, created_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (person1_id, person2_id) DO NOTHING
	`

	_, err := r.pool.Exec(ctx, query, relation.Person1ID, relation.Person2ID, relation.Similarity)
	return err
}

// GetByPersonID retrieves all relations for a person.
func (r *RelationRepository) GetByPersonID(ctx context.Context, personID uuid.UUID, minSimilarity float32) ([]*model.PersonRelation, error) {
	query := `
		SELECT person1_id, person2_id, similarity, created_at
		FROM person_relations
		WHERE (person1_id = $1 OR person2_id = $1) AND similarity >= $2
		ORDER BY similarity DESC
	`

	rows, err := r.pool.Query(ctx, query, personID, minSimilarity)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var relations []*model.PersonRelation
	for rows.Next() {
		rel := &model.PersonRelation{}
		err := rows.Scan(&rel.Person1ID, &rel.Person2ID, &rel.Similarity, &rel.CreatedAt)
		if err != nil {
			return nil, err
		}
		relations = append(relations, rel)
	}

	return relations, nil
}

// GetGraph retrieves graph data for multiple persons.
func (r *RelationRepository) GetGraph(ctx context.Context, personIDs []uuid.UUID, minSimilarity float32) ([]*model.PersonNode, error) {
	query := `
		SELECT p.id, p.name, p.custom_name, p.avatar_path, p.face_count, p.photo_count,
		       ARRAY_AGG(DISTINCT pr.person2_id) FILTER (WHERE pr.person2_id IS NOT NULL) as connected_ids,
		       ARRAY_AGG(DISTINCT pr.similarity) FILTER (WHERE pr.similarity IS NOT NULL) as similarities
		FROM persons p
		LEFT JOIN person_relations pr ON p.id = pr.person1_id AND pr.similarity >= $2
		WHERE p.id = ANY($1::uuid[])
		GROUP BY p.id
	`

	rows, err := r.pool.Query(ctx, query, personIDs, minSimilarity)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []*model.PersonNode
	for rows.Next() {
		node := &model.PersonNode{}
		var connectedIDs []uuid.UUID
		var similarities []float32

		err := rows.Scan(
			&node.ID,
			&node.Name,
			&node.CustomName,
			&node.AvatarPath,
			&node.FaceCount,
			&node.PhotoCount,
			&connectedIDs,
			&similarities,
		)
		if err != nil {
			return nil, err
		}

		node.ConnectedPersonIDs = connectedIDs
		node.ConnectionSimilarities = similarities
		nodes = append(nodes, node)
	}

	return nodes, nil
}

// DeleteByPersonID deletes all relations for a person.
func (r *RelationRepository) DeleteByPersonID(ctx context.Context, personID uuid.UUID) error {
	query := `DELETE FROM person_relations WHERE person1_id = $1 OR person2_id = $1`
	_, err := r.pool.Exec(ctx, query, personID)
	return err
}

// GetStrength returns the similarity strength between two persons.
func (r *RelationRepository) GetStrength(ctx context.Context, person1ID, person2ID uuid.UUID) (float32, error) {
	query := `
		SELECT similarity FROM person_relations
		WHERE (person1_id = $1 AND person2_id = $2) OR (person1_id = $2 AND person2_id = $1)
		LIMIT 1
	`

	var similarity float32
	err := r.pool.QueryRow(ctx, query, person1ID, person2ID).Scan(&similarity)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}

	return similarity, nil
}

// Count returns total number of relations.
func (r *RelationRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM person_relations`).Scan(&count)
	return count, err
}
