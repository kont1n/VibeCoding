package organizer

import (
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kont1n/face-grouper/internal/models"
)

// PersonInfo holds metadata about an organized person cluster for the report.
type PersonInfo struct {
	ID         int
	PhotoCount int
	FaceCount  int
	Thumbnail  string
	Photos     []string
}

// Organize creates Person_N directories under outputDir, symlinks photos, and picks
// the best face thumbnail per person. Returns metadata for each person cluster.
func Organize(clusters []models.Cluster, outputDir string, w io.Writer) ([]PersonInfo, error) {
	// Clean only Person_* dirs and old report — preserve .thumbnails and logs
	if entries, err := os.ReadDir(outputDir); err == nil {
		for _, e := range entries {
			name := e.Name()
			if (e.IsDir() && strings.HasPrefix(name, "Person_")) || name == "report.json" {
				os.RemoveAll(filepath.Join(outputDir, name))
			}
		}
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create output dir: %w", err)
	}

	sort.Slice(clusters, func(i, j int) bool {
		return len(clusters[i].Faces) > len(clusters[j].Faces)
	})

	var persons []PersonInfo

	for i, cluster := range clusters {
		personName := fmt.Sprintf("Person_%d", i+1)
		personDir := filepath.Join(outputDir, personName)
		if err := os.MkdirAll(personDir, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create %s: %w", personDir, err)
		}

		seen := make(map[string]bool)
		usedFileNames := make(map[string]bool)
		var bestScore float64
		var bestThumb string
		var photos []string

		for _, face := range cluster.Faces {
			if !seen[face.FilePath] {
				seen[face.FilePath] = true
				fileName := uniquePhotoName(face.FilePath, usedFileNames)
				dstPath := filepath.Join(personDir, fileName)
				if err := linkOrCopy(face.FilePath, dstPath); err != nil {
					fmt.Fprintf(w, "WARNING: %s: %v\n", dstPath, err)
				}
				photos = append(photos, personName+"/"+fileName)
			}

			if face.DetScore > bestScore && face.Thumbnail != "" {
				bestScore = face.DetScore
				bestThumb = face.Thumbnail
			}
		}

		thumbRel := ""
		if bestThumb != "" {
			thumbDst := filepath.Join(personDir, "thumb.jpg")
			if err := copyFile(bestThumb, thumbDst); err != nil {
				fmt.Fprintf(w, "WARNING: thumbnail copy for %s: %v\n", personName, err)
			} else {
				thumbRel = personName + "/thumb.jpg"
			}
		}

		persons = append(persons, PersonInfo{
			ID:         i + 1,
			PhotoCount: len(seen),
			FaceCount:  len(cluster.Faces),
			Thumbnail:  thumbRel,
			Photos:     photos,
		})

		fmt.Fprintf(w, "Person_%d: %d unique photo(s)\n", i+1, len(seen))
	}

	return persons, nil
}

func linkOrCopy(src, dst string) error {
	if err := os.Symlink(src, dst); err == nil {
		return nil
	}
	return copyFile(src, dst)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return nil
}

func uniquePhotoName(srcPath string, used map[string]bool) string {
	base := filepath.Base(srcPath)
	if !used[base] {
		used[base] = true
		return base
	}

	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	hash := shortPathHash(srcPath)
	candidate := fmt.Sprintf("%s_%s%s", name, hash, ext)
	if !used[candidate] {
		used[candidate] = true
		return candidate
	}

	for i := 1; ; i++ {
		candidate = fmt.Sprintf("%s_%s_%d%s", name, hash, i, ext)
		if !used[candidate] {
			used[candidate] = true
			return candidate
		}
	}
}

func shortPathHash(path string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(path))
	return fmt.Sprintf("%016x", h.Sum64())[:10]
}
