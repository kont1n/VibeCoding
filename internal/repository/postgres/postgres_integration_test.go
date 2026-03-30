//go:build integration
// +build integration

package postgres_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kont1n/face-grouper/internal/config/env"
	"github.com/kont1n/face-grouper/internal/database"
	"github.com/kont1n/face-grouper/internal/model"
	"github.com/kont1n/face-grouper/internal/repository/postgres"
)

var (
	testDB   *database.DB
	testPool *pgxpool.Pool
	ctx      context.Context
	cancel   context.CancelFunc
)

// TestMain runs setup before all tests and teardown after.
func TestMain(m *testing.M) {
	// Setup
	var err error
	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	testPool, err = setupTestDatabase()
	if err != nil {
		fmt.Printf("Failed to setup test database: %v\n", err)
		os.Exit(1)
	}

	testDB = &database.DB{
		Pool:      testPool,
		Persons:   postgres.NewPersonRepository(testPool),
		Faces:     postgres.NewFaceRepository(testPool),
		Photos:    postgres.NewPhotoRepository(testPool),
		Relations: postgres.NewRelationRepository(testPool),
		Sessions:  postgres.NewSessionRepository(testPool),
	}

	// Run tests
	code := m.Run()

	// Teardown
	testPool.Close()
	os.Exit(code)
}

// setupTestDatabase creates a test database connection.
func setupTestDatabase() (*pgxpool.Pool, error) {
	cfg := env.DatabaseConfig{
		Host:          getEnv("TEST_DB_HOST", "localhost"),
		Port:          getIntEnv("TEST_DB_PORT", 5432),
		Database:      getEnv("TEST_DB_NAME", "face-grouper-test"),
		User:          getEnv("TEST_DB_USER", "face-grouper"),
		Password:      getEnv("TEST_DB_PASSWORD", "secret"),
		SSLMode:       "disable",
		MaxConns:      5,
		MinConns:      1,
		RunMigrations: true,
	}

	db, err := database.New(ctx, cfg)
	if err != nil {
		return nil, err
	}

	return db.Pool, nil
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intValue int
		fmt.Sscanf(value, "%d", &intValue)
		return intValue
	}
	return defaultValue
}

// ============= Person Repository Tests =============

func TestPersonRepository_CreateAndGet(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	// Create person
	person := &model.Person{
		Name:         "Test Person",
		FaceCount:    5,
		PhotoCount:   3,
		QualityScore: 0.85,
	}

	err := testDB.Persons.Create(ctx, person)
	require.NoError(err)
	require.NotEmpty(person.ID)

	// Get by ID
	retrieved, err := testDB.Persons.GetByID(ctx, person.ID)
	require.NoError(err)
	require.NotNil(retrieved)

	assert.Equal(person.Name, retrieved.Name)
	assert.Equal(person.FaceCount, retrieved.FaceCount)
}

func TestPersonRepository_List(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	// Create multiple persons
	for i := 0; i < 3; i++ {
		person := &model.Person{
			Name:       fmt.Sprintf("Person %d", i),
			FaceCount:  i + 1,
			PhotoCount: i + 2,
		}
		err := testDB.Persons.Create(ctx, person)
		require.NoError(err)
	}

	// List persons
	persons, err := testDB.Persons.List(ctx, 0, 10)
	require.NoError(err)
	assert.GreaterOrEqual(len(persons), 3)
}

func TestPersonRepository_Search(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	// Create person with specific name
	person := &model.Person{
		Name: "Иван Иванов",
	}
	err := testDB.Persons.Create(ctx, person)
	require.NoError(err)

	// Search
	results, err := testDB.Persons.Search(ctx, "Иван", 10)
	require.NoError(err)
	assert.Greater(len(results), 0)
}

// ============= Face Repository Tests =============

func TestFaceRepository_CreateAndGet(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	// Create person first
	person := &model.Person{Name: "Face Test Person"}
	err := testDB.Persons.Create(ctx, person)
	require.NoError(err)

	// Create face
	face := &model.Face{
		PersonID:  person.ID,
		Embedding: make([]float64, 512),
		BBox: model.BBox{
			X1: 100, Y1: 100, X2: 200, Y2: 200,
		},
		DetScore: 0.95,
	}

	// Initialize embedding with some values
	for i := range face.Embedding {
		face.Embedding[i] = float64(i) * 0.001
	}

	err = testDB.Faces.Create(ctx, face)
	require.NoError(err)

	// Get by ID
	retrieved, err := testDB.Faces.GetByID(ctx, face.ID)
	require.NoError(err)
	require.NotNil(retrieved)

	assert.Equal(face.PersonID, retrieved.PersonID)
	assert.Equal(face.DetScore, retrieved.DetScore)
	assert.Len(retrieved.Embedding, 512)
}

func TestFaceRepository_FindSimilar(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	// Create person and face
	person := &model.Person{Name: "Similar Face Test"}
	err := testDB.Persons.Create(ctx, person)
	require.NoError(err)

	embedding := make([]float64, 512)
	for i := range embedding {
		embedding[i] = float64(i) * 0.001
	}

	face := &model.Face{
		PersonID:  person.ID,
		Embedding: embedding,
		BBox:      model.BBox{X1: 100, Y1: 100, X2: 200, Y2: 200},
		DetScore:  0.95,
	}

	err = testDB.Faces.Create(ctx, face)
	require.NoError(err)

	// Find similar faces
	similar, err := testDB.Persons.FindSimilarFaces(ctx, embedding, 10)
	require.NoError(err)
	assert.Greater(len(similar), 0)
	assert.Greater(similar[0].Similarity, 0.9) // Should be very similar
}

// ============= Photo Repository Tests =============

func TestPhotoRepository_CreateAndGet(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	photo := &model.Photo{
		Path:         "/test/photo.jpg",
		OriginalPath: "/original/photo.jpg",
		Width:        1920,
		Height:       1080,
		FileSize:     1024000,
		MimeType:     "image/jpeg",
	}

	err := testDB.Photos.Create(ctx, photo)
	require.NoError(err)

	// Get by path
	retrieved, err := testDB.Photos.GetByPath(ctx, photo.Path)
	require.NoError(err)
	require.NotNil(retrieved)

	assert.Equal(photo.Width, retrieved.Width)
	assert.Equal(photo.Height, retrieved.Height)
}

// ============= Session Repository Tests =============

func TestSessionRepository_CreateAndUpdate(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	session := &model.ProcessingSession{
		Status:         "processing",
		Stage:          "extraction",
		Progress:       0.0,
		TotalItems:     100,
		ProcessedItems: 0,
		Errors:         0,
	}

	err := testDB.Sessions.Create(ctx, session)
	require.NoError(err)

	// Update progress
	err = testDB.Sessions.UpdateProgress(ctx, session.ID, 0.5, 50, 2)
	require.NoError(err)

	// Get and verify
	retrieved, err := testDB.Sessions.GetByID(ctx, session.ID)
	require.NoError(err)
	require.NotNil(retrieved)

	assert.Equal(float32(0.5), retrieved.Progress)
	assert.Equal(50, retrieved.ProcessedItems)
	assert.Equal(2, retrieved.Errors)
}

// ============= Relation Repository Tests =============

func TestRelationRepository_Create(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	// Create two persons
	person1 := &model.Person{Name: "Person 1"}
	person2 := &model.Person{Name: "Person 2"}

	err := testDB.Persons.Create(ctx, person1)
	require.NoError(err)

	err = testDB.Persons.Create(ctx, person2)
	require.NoError(err)

	// Create relation
	relation := &model.PersonRelation{
		Person1ID:  person1.ID,
		Person2ID:  person2.ID,
		Similarity: 0.85,
	}

	err = testDB.Relations.Create(ctx, relation)
	require.NoError(err)

	// Get relations
	relations, err := testDB.Relations.GetByPersonID(ctx, person1.ID, 0.5)
	require.NoError(err)
	assert.Greater(len(relations), 0)
	assert.Equal(float32(0.85), relations[0].Similarity)
}
