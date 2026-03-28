package organization

import (
	"context"
	"io"

	"go.uber.org/zap"

	"github.com/kont1n/face-grouper/internal/model"
	"github.com/kont1n/face-grouper/internal/organizer"
	"github.com/kont1n/face-grouper/platform/pkg/logger"
)

// PersonInfo содержит метаданные организованного кластера.
type PersonInfo struct {
	ID           int
	PhotoCount   int
	FaceCount    int
	Thumbnail    string
	AvatarPath   string
	QualityScore float64
	Photos       []string
}

// OrganizeService определяет интерфейс сервиса организации.
type OrganizeService interface {
	Organize(ctx context.Context, clusters []model.Cluster, outputDir string, avatarUpdateThreshold float64, w io.Writer) ([]PersonInfo, error)
}

type organizeService struct{}

// NewOrganizeService создаёт новый экземпляр сервиса организации.
func NewOrganizeService() OrganizeService {
	return &organizeService{}
}

// Organize создаёт директории Person_N и организует результаты.
func (s *organizeService) Organize(ctx context.Context, clusters []model.Cluster, outputDir string, avatarUpdateThreshold float64, w io.Writer) ([]PersonInfo, error) {
	logger.Info(ctx, "starting organization",
		zap.Int("clusters", len(clusters)),
		zap.String("output_dir", outputDir),
	)

	personInfos, err := organizer.Organize(clusters, outputDir, avatarUpdateThreshold, w)
	if err != nil {
		logger.Error(ctx, "organization failed", zap.Error(err))
		return nil, err
	}

	// Конвертация в наш тип.
	persons := make([]PersonInfo, len(personInfos))
	for i, p := range personInfos {
		persons[i] = PersonInfo(p)
	}

	logger.Info(ctx, "organization completed",
		zap.Int("persons", len(persons)),
	)

	return persons, nil
}
