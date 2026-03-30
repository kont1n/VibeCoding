// Package organizer creates per-person directories and manages face thumbnails.
package organizer

import (
	"fmt"
	"hash/fnv"
	"image"
	_ "image/jpeg" // register JPEG decoder for image.DecodeConfig.
	_ "image/png"  // register PNG decoder for image.DecodeConfig.
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kont1n/face-grouper/internal/avatar"
	"github.com/kont1n/face-grouper/internal/model"
	"github.com/kont1n/face-grouper/internal/report"
)

// PersonInfo holds metadata about an organized person cluster for the report.
type PersonInfo struct {
	ID           int
	PhotoCount   int
	FaceCount    int
	Thumbnail    string
	AvatarPath   string
	QualityScore float64
	Photos       []string
}

type previousAvatar struct {
	avatarPath string
	quality    float64
}

// Organize creates Person_N directories under outputDir, symlinks photos, and picks
// the best face thumbnail per person. Returns metadata for each person cluster.
func Organize(clusters []model.Cluster, outputDir string, avatarUpdateThreshold float64, w io.Writer) ([]PersonInfo, error) {
	if avatarUpdateThreshold < 0 {
		avatarUpdateThreshold = 0
	}

	prev := make(map[int]previousAvatar)
	if oldReport, err := report.Load(outputDir); err == nil {
		for _, p := range oldReport.Persons {
			prev[p.ID] = previousAvatar{
				avatarPath: p.AvatarPath,
				quality:    p.QualityScore,
			}
		}
	}

	// Clean only Person_* dirs and old report — preserve .thumbnails, avatars and logs.
	if entries, err := os.ReadDir(outputDir); err == nil {
		for _, e := range entries {
			name := e.Name()
			if (e.IsDir() && strings.HasPrefix(name, "Person_")) || name == "report.json" {
				_ = os.RemoveAll(filepath.Join(outputDir, name))
			}
		}
	}

	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create output dir: %w", err)
	}
	avatarsDir := filepath.Join(outputDir, "avatars")
	if err := os.MkdirAll(avatarsDir, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create avatars dir: %w", err)
	}

	sort.Slice(clusters, func(i, j int) bool {
		return len(clusters[i].Faces) > len(clusters[j].Faces)
	})

	var persons []PersonInfo

	for i, cluster := range clusters {
		personName := fmt.Sprintf("Person_%d", i+1)
		personDir := filepath.Join(outputDir, personName)
		if err := os.MkdirAll(personDir, 0o750); err != nil {
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
					_, _ = fmt.Fprintf(w, "WARNING: %s: %v\n", dstPath, err)
				}
				photos = append(photos, personName+"/"+fileName)
			}

			if face.Thumbnail != "" {
				faceScore := scoreFace(face)
				if faceScore > bestScore {
					bestScore = faceScore
					bestThumb = face.Thumbnail
				}
			}
		}

		thumbRel := ""
		if bestThumb != "" {
			thumbDst := filepath.Join(personDir, "thumb.jpg")
			if err := copyFile(bestThumb, thumbDst); err != nil {
				_, _ = fmt.Fprintf(w, "WARNING: thumbnail copy for %s: %v\n", personName, err)
			} else {
				thumbRel = personName + "/thumb.jpg"
			}
		}

		personID := i + 1
		prevAvatar := prev[personID]
		avatarRel := prevAvatar.avatarPath
		avatarScore := prevAvatar.quality

		if avatarRel != "" {
			avatarAbs := filepath.Join(outputDir, filepath.FromSlash(avatarRel))
			if _, err := os.Stat(avatarAbs); err != nil {
				avatarRel = ""
				avatarScore = 0
			}
		}

		if bestThumb != "" {
			shouldUpdate := false
			if avatarRel == "" || avatarScore <= 0 {
				shouldUpdate = true
			} else if bestScore >= avatarScore*(1.0+avatarUpdateThreshold) {
				shouldUpdate = true
			}

			if shouldUpdate {
				avatarRel = filepath.ToSlash(filepath.Join("avatars", fmt.Sprintf("Person_%d.jpg", personID)))
				avatarAbs := filepath.Join(outputDir, filepath.FromSlash(avatarRel))
				if err := copyFile(bestThumb, avatarAbs); err != nil {
					_, _ = fmt.Fprintf(w, "WARNING: avatar update for Person_%d: %v\n", personID, err)
				} else {
					avatarScore = bestScore
				}
			}
		}

		if avatarRel == "" && thumbRel != "" {
			avatarRel = thumbRel
			if avatarScore <= 0 {
				avatarScore = bestScore
			}
		}

		persons = append(persons, PersonInfo{
			ID:           personID,
			PhotoCount:   len(seen),
			FaceCount:    len(cluster.Faces),
			Thumbnail:    thumbRel,
			AvatarPath:   avatarRel,
			QualityScore: avatarScore,
			Photos:       photos,
		})

		_, _ = fmt.Fprintf(w, "Person_%d: %d unique photo(s)\n", personID, len(seen))
	}

	return persons, nil
}

func scoreFace(face model.Face) float64 {
	if face.Thumbnail == "" {
		return 0
	}
	f, err := os.Open(face.Thumbnail) //nolint:gosec
	if err != nil {
		return 0
	}
	defer func() { _ = f.Close() }()

	img, _, err := image.Decode(f)
	if err != nil {
		return 0
	}

	box := avatar.Box{
		X1: float64(face.BBox.X1),
		Y1: float64(face.BBox.Y1),
		X2: float64(face.BBox.X2),
		Y2: float64(face.BBox.Y2),
	}

	frontalFactor := 1.0
	if hasKeypoints(face.Keypoints) {
		frontalFactor = avatar.EstimateFrontalPoseFactorFromKeypoints(face.Keypoints)
	}
	return avatar.CalculateFaceScoreWithFrontal(img, box, frontalFactor)
}

func hasKeypoints(kps [5][2]float64) bool {
	for i := 0; i < 5; i++ {
		if kps[i][0] != 0 || kps[i][1] != 0 {
			return true
		}
	}
	return false
}

func linkOrCopy(src, dst string) error {
	if err := os.Symlink(src, dst); err == nil {
		return nil
	}
	return copyFile(src, dst)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src) //nolint:gosec
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600) //nolint:gosec // dst is under controlled output tree
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
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
