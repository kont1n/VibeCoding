package clustering

import (
	"context"

	"go.uber.org/zap"

	clustering "github.com/kont1n/face-grouper/internal/clustering"
	"github.com/kont1n/face-grouper/internal/model"
	"github.com/kont1n/face-grouper/platform/pkg/logger"
)

// ClusterService определяет интерфейс сервиса кластеризации.
type ClusterService interface {
	Cluster(ctx context.Context, faces []model.Face, threshold float64) ([]model.Cluster, error)
}

type clusterService struct{}

// NewClusterService создаёт новый экземпляр сервиса кластеризации.
func NewClusterService() ClusterService {
	return &clusterService{}
}

// Cluster группирует лица по сходству эмбеддингов.
func (s *clusterService) Cluster(ctx context.Context, faces []model.Face, threshold float64) ([]model.Cluster, error) {
	logger.Info(ctx, "starting clustering",
		zap.Int("faces", len(faces)),
		zap.Float64("threshold", threshold),
	)

	clusters := Cluster(faces, threshold)

	logger.Info(ctx, "clustering completed",
		zap.Int("persons", len(clusters)),
	)

	return clusters, nil
}

// Cluster — обёртка над функцией кластеризации.
func Cluster(faces []model.Face, threshold float64) []model.Cluster {
	return clustering.Cluster(faces, threshold)
}
