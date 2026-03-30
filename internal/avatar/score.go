// Package avatar provides face scoring and avatar selection utilities.
package avatar

import (
	"image"
	"math"
)

// Box represents a face bounding box in source image coordinates.
type Box struct {
	X1 float64
	Y1 float64
	X2 float64
	Y2 float64
}

// CalculateFaceScore calculates the quality score of a face crop:
// Score = (Width * Height) * Sharpness * FrontalPoseFactor.
// When frontal pose is not available, factor is treated as 1.0.
func CalculateFaceScore(crop image.Image, bbox Box) float64 {
	return CalculateFaceScoreWithFrontal(crop, bbox, 1.0)
}

// CalculateFaceScoreWithFrontal calculates score using an explicit frontal factor.
func CalculateFaceScoreWithFrontal(crop image.Image, bbox Box, frontalPoseFactor float64) float64 {
	if crop == nil {
		return 0
	}
	area := (bbox.X2 - bbox.X1) * (bbox.Y2 - bbox.Y1)
	if area <= 0 {
		return 0
	}

	sharpness := laplacianVariance(crop)
	if sharpness <= 0 {
		return 0
	}

	return area * sharpness * clamp(frontalPoseFactor, 0.15)
}

// EstimateFrontalPoseFactorFromAngles converts pitch/yaw/roll degrees to
// a frontal factor in [0.15..1.0], where values near 1 are more frontal.
func EstimateFrontalPoseFactorFromAngles(pitch, yaw, roll float64) float64 {
	// Around 15-20 deg should already get a visible penalty.
	const sigmaDeg = 18.0
	norm := (pitch*pitch + yaw*yaw + roll*roll) / (sigmaDeg * sigmaDeg)
	return clamp(math.Exp(-0.5*norm), 0.15)
}

// EstimateFrontalPoseFactorFromKeypoints estimates frontal factor from 5 landmarks:
// left_eye, right_eye, nose, left_mouth, right_mouth.
func EstimateFrontalPoseFactorFromKeypoints(kps [5][2]float64) float64 {
	leftEye := kps[0]
	rightEye := kps[1]
	nose := kps[2]
	leftMouth := kps[3]
	rightMouth := kps[4]

	eyeDist := distance(leftEye, rightEye)
	if eyeDist < 1 {
		return 0.15
	}

	eyeMidX := (leftEye[0] + rightEye[0]) * 0.5
	eyeMidY := (leftEye[1] + rightEye[1]) * 0.5
	mouthMidX := (leftMouth[0] + rightMouth[0]) * 0.5
	mouthMidY := (leftMouth[1] + rightMouth[1]) * 0.5

	// Yaw proxy: horizontal asymmetry of nose around eye/mouth centers.
	yawRaw := (math.Abs(nose[0]-eyeMidX) + math.Abs(nose[0]-mouthMidX)) * 0.5 / eyeDist
	yawNorm := clamp(yawRaw/0.35, 0)

	// Roll proxy: eye line tilt.
	rollRad := math.Abs(math.Atan2(rightEye[1]-leftEye[1], rightEye[0]-leftEye[0]))
	rollNorm := clamp(rollRad/(25.0*math.Pi/180.0), 0)

	// Pitch proxy: vertical nose position between eyes and mouth.
	den := mouthMidY - eyeMidY
	pitchNorm := 1.0
	if math.Abs(den) > 1e-6 {
		ratio := (nose[1] - eyeMidY) / den
		pitchNorm = clamp(math.Abs(ratio-0.5)/0.35, 0)
	}

	penalty := 1.7*yawNorm + 1.2*pitchNorm + 1.0*rollNorm
	return clamp(math.Exp(-penalty), 0.15)
}

func laplacianVariance(img image.Image) float64 {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	if w < 3 || h < 3 {
		return 0
	}

	gray := make([]float64, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, bv, _ := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			// Convert to 8-bit luminance in pure Go.
			r8 := float64((r >> 8) & 0xFF)
			g8 := float64((g >> 8) & 0xFF)
			b8 := float64((bv >> 8) & 0xFF)
			gray[y*w+x] = 0.299*r8 + 0.587*g8 + 0.114*b8
		}
	}

	var sum float64
	var sumSq float64
	count := 0

	// 3x3 Laplacian kernel:
	// [-1 -1 -1
	//  -1  8 -1
	//  -1 -1 -1]
	for y := 1; y < h-1; y++ {
		row := y * w
		for x := 1; x < w-1; x++ {
			i := row + x
			lap := -gray[i-w-1] - gray[i-w] - gray[i-w+1] -
				gray[i-1] + 8*gray[i] - gray[i+1] -
				gray[i+w-1] - gray[i+w] - gray[i+w+1]
			sum += lap
			sumSq += lap * lap
			count++
		}
	}

	if count == 0 {
		return 0
	}

	mean := sum / float64(count)
	variance := sumSq/float64(count) - mean*mean
	if variance < 0 {
		return 0
	}
	return variance
}

func distance(a, b [2]float64) float64 {
	dx := a[0] - b[0]
	dy := a[1] - b[1]
	return math.Hypot(dx, dy)
}

func clamp(v, minV float64) float64 {
	const maxV = 1.0
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}
