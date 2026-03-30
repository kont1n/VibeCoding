// Package cli provides CLI API for face grouping operations.
package cli

import (
	"context"
	"io"

	"github.com/kont1n/face-grouper/internal/model"
	"github.com/kont1n/face-grouper/internal/service/clustering"
	"github.com/kont1n/face-grouper/internal/service/extraction"
	"github.com/kont1n/face-grouper/internal/service/organizer"
	"github.com/kont1n/face-grouper/internal/service/scan"
)

// API предоставляет высокоуровневый интерфейс для операций группировки лиц.
type API struct {
	scanService       scan.ScanService
	extractionService extraction.ExtractionService
	clusterService    clustering.ClusterService
	organizeService   *organizer.Organizer
}

// NewAPI создаёт новый CLI API.
func NewAPI(
	scanService scan.ScanService,
	extractionService extraction.ExtractionService,
	clusterService clustering.ClusterService,
	organizeService *organizer.Organizer,
) *API {
	return &API{
		scanService:       scanService,
		extractionService: extractionService,
		clusterService:    clusterService,
		organizeService:   organizeService,
	}
}

// Scan сканирует директорию и возвращает список изображений.
func (a *API) Scan(ctx context.Context, dir string) ([]string, error) {
	return a.scanService.Scan(ctx, dir)
}

// Extract извлекает эмбеддинги лиц из изображений.
func (a *API) Extract(ctx context.Context, files []string, thumbDir string, w io.Writer) (*extraction.ExtractionResult, error) {
	return a.extractionService.Extract(ctx, files, thumbDir, w)
}

// Cluster группирует лица по эмбеддингам.
func (a *API) Cluster(ctx context.Context, faces []model.Face, threshold float64) ([]model.Cluster, error) {
	return a.clusterService.Cluster(ctx, faces, threshold)
}

// Organize организует результаты в папки perсон.
func (a *API) Organize(ctx context.Context, clusters []model.Cluster, outputDir string, avatarUpdateThreshold float64, w io.Writer) ([]organizer.PersonInfo, error) {
	return a.organizeService.Organize(clusters, outputDir, avatarUpdateThreshold, w)
}
