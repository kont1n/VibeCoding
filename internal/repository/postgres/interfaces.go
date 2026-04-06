// Package postgres provides database repository interfaces.
package postgres

import (
	"context"

	"github.com/google/uuid"

	"github.com/kont1n/face-grouper/internal/model"
)

// PersonRepository defines the interface for person data access.
type PersonRepository interface {
	// Create creates a new person.
	Create(ctx context.Context, person *model.Person) error
	// GetByID returns a person by ID.
	GetByID(ctx context.Context, id uuid.UUID) (*model.Person, error)
	// GetAll returns all persons.
	GetAll(ctx context.Context) ([]*model.Person, error)
	// Update updates a person.
	Update(ctx context.Context, person *model.Person) error
	// UpdateName updates person's custom name.
	UpdateName(ctx context.Context, id uuid.UUID, customName string) error
	// IncrementFaceCount increments face count for a person.
	IncrementFaceCount(ctx context.Context, id uuid.UUID) error
	// IncrementPhotoCount increments photo count for a person.
	IncrementPhotoCount(ctx context.Context, id uuid.UUID) error
	// Delete deletes a person.
	Delete(ctx context.Context, id uuid.UUID) error
	// Search searches for persons by name.
	Search(ctx context.Context, query string) ([]*model.Person, error)
	// List returns a paginated list of persons.
	List(ctx context.Context, offset, limit int) ([]*model.Person, error)
	// Count returns the total number of persons.
	Count(ctx context.Context) (int, error)
}

// FaceRepository defines the interface for face data access.
type FaceRepository interface {
	// Create creates a new face.
	Create(ctx context.Context, face *model.Face) error
	// GetByID returns a face by ID.
	GetByID(ctx context.Context, id uuid.UUID) (*model.Face, error)
	// GetByPersonID returns all faces for a person.
	GetByPersonID(ctx context.Context, personID uuid.UUID) ([]*model.Face, error)
	// CreateBatch creates multiple faces in a single transaction.
	CreateBatch(ctx context.Context, faces []*model.Face) error
	// FindSimilar finds similar faces using vector similarity.
	FindSimilar(ctx context.Context, embedding []float32, threshold float64, limit int) ([]*model.Face, error)
}

// PhotoRepository defines the interface for photo data access.
type PhotoRepository interface {
	// Create creates a new photo.
	Create(ctx context.Context, photo *model.Photo) error
	// GetByID returns a photo by ID.
	GetByID(ctx context.Context, id uuid.UUID) (*model.Photo, error)
	// GetByPersonID returns all photos for a person.
	GetByPersonID(ctx context.Context, personID uuid.UUID) ([]*model.Photo, error)
	// GetByPath returns a photo by file path.
	GetByPath(ctx context.Context, path string) (*model.Photo, error)
	// CreateBatch creates multiple photos in a single transaction.
	CreateBatch(ctx context.Context, photos []*model.Photo) error
	// ListByPerson returns paginated photos for a person.
	ListByPerson(ctx context.Context, personID uuid.UUID, offset, limit int) ([]*model.Photo, error)
	// CountByPerson returns the number of photos for a person.
	CountByPerson(ctx context.Context, personID uuid.UUID) (int, error)
	// ListByPersonWithFaces returns photos for a person with all faces and bounding boxes.
	ListByPersonWithFaces(ctx context.Context, personID uuid.UUID, offset, limit int) ([]*model.PhotoWithFaces, error)
}

// RelationRepository defines the interface for relation data access.
type RelationRepository interface {
	// Create creates a new relation.
	Create(ctx context.Context, relation *model.PersonRelation) error
	// GetByPersonID returns all relations for a person.
	GetByPersonID(ctx context.Context, personID uuid.UUID) ([]*model.PersonRelation, error)
	// CreateBatch creates multiple relations in a single transaction.
	CreateBatch(ctx context.Context, relations []*model.PersonRelation) error
	// GetByPersonIDWithMinSimilarity returns relations with minimum similarity.
	GetByPersonIDWithMinSimilarity(ctx context.Context, personID uuid.UUID, minSimilarity float32) ([]*model.PersonRelation, error)
	// GetGraph returns the relation graph for given person IDs.
	GetGraph(ctx context.Context, personIDs []uuid.UUID, minSimilarity float32) ([]model.PersonNode, error)
}

// SessionRepository defines the interface for session data access.
type SessionRepository interface {
	// Create creates a new processing session.
	Create(ctx context.Context, session *model.ProcessingSession) error
	// GetByID returns a session by ID.
	GetByID(ctx context.Context, id string) (*model.ProcessingSession, error)
	// Update updates a session.
	Update(ctx context.Context, session *model.ProcessingSession) error
	// Delete deletes a session.
	Delete(ctx context.Context, id string) error
	// GetActive returns the currently active (processing) session.
	GetActive(ctx context.Context) (*model.ProcessingSession, error)
}
