package inference

import "sort"

// Detection holds a single face detection before NMS.
type Detection struct {
	X1, Y1, X2, Y2 float32
	Score           float32
	Kps             [5][2]float32
}

// NMS performs greedy Non-Maximum Suppression on detections sorted by score.
func NMS(dets []Detection, thresh float32) []Detection {
	if len(dets) == 0 {
		return nil
	}

	sort.Slice(dets, func(i, j int) bool {
		return dets[i].Score > dets[j].Score
	})

	suppressed := make([]bool, len(dets))
	var keep []Detection

	for i := range dets {
		if suppressed[i] {
			continue
		}
		keep = append(keep, dets[i])
		for j := i + 1; j < len(dets); j++ {
			if suppressed[j] {
				continue
			}
			if iou(dets[i], dets[j]) > thresh {
				suppressed[j] = true
			}
		}
	}
	return keep
}

func iou(a, b Detection) float32 {
	xx1 := max32(a.X1, b.X1)
	yy1 := max32(a.Y1, b.Y1)
	xx2 := min32(a.X2, b.X2)
	yy2 := min32(a.Y2, b.Y2)

	w := max32(0, xx2-xx1)
	h := max32(0, yy2-yy1)
	inter := w * h

	areaA := (a.X2 - a.X1) * (a.Y2 - a.Y1)
	areaB := (b.X2 - b.X1) * (b.Y2 - b.Y1)
	union := areaA + areaB - inter
	if union <= 0 {
		return 0
	}
	return inter / union
}

func max32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}
