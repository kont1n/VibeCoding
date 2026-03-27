package organizer

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/kont1n/face-grouper/internal/models"
)

// Organize creates Person_N directories under outputDir and symlinks photos into them.
// A photo may appear in multiple Person_N folders if it contains multiple people.
func Organize(clusters []models.Cluster, outputDir string) error {
	if err := os.RemoveAll(outputDir); err != nil {
		return fmt.Errorf("failed to clean output dir: %w", err)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}

	sort.Slice(clusters, func(i, j int) bool {
		return len(clusters[i].Faces) > len(clusters[j].Faces)
	})

	for i, cluster := range clusters {
		personDir := filepath.Join(outputDir, fmt.Sprintf("Person_%d", i+1))
		if err := os.MkdirAll(personDir, 0o755); err != nil {
			return fmt.Errorf("failed to create %s: %w", personDir, err)
		}

		seen := make(map[string]bool)
		for _, face := range cluster.Faces {
			if seen[face.FilePath] {
				continue
			}
			seen[face.FilePath] = true

			fileName := filepath.Base(face.FilePath)
			linkPath := filepath.Join(personDir, fileName)

			if err := os.Symlink(face.FilePath, linkPath); err != nil {
				fmt.Printf("WARNING: cannot create symlink %s -> %s: %v\n", linkPath, face.FilePath, err)
			}
		}

		fmt.Printf("Person_%d: %d unique photo(s)\n", i+1, len(seen))
	}

	return nil
}
