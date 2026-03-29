package avatar

import (
	"image"
	"image/color"
	"testing"
)

func TestCalculateFaceScoreScalesWithArea(t *testing.T) {
	t.Parallel()

	img := checkerboard(32, 32)
	small := CalculateFaceScore(img, Box{X1: 0, Y1: 0, X2: 10, Y2: 10})
	large := CalculateFaceScore(img, Box{X1: 0, Y1: 0, X2: 20, Y2: 20})

	if small <= 0 {
		t.Fatalf("expected positive score for textured image, got %f", small)
	}
	if large <= small {
		t.Fatalf("expected larger bbox to have higher score, small=%f large=%f", small, large)
	}
}

func TestEstimateFrontalPoseFactorFromAngles(t *testing.T) {
	t.Parallel()

	front := EstimateFrontalPoseFactorFromAngles(0, 0, 0)
	turned := EstimateFrontalPoseFactorFromAngles(0, 35, 10)

	if front <= turned {
		t.Fatalf("expected frontal factor to decrease with larger angles, front=%f turned=%f", front, turned)
	}
}

func TestEstimateFrontalPoseFactorFromKeypoints(t *testing.T) {
	t.Parallel()

	// Each landmark has [x, y] coordinates.
	// 5 landmarks: left_eye, right_eye, nose, left_mouth, right_mouth.
	frontKps := [5][2]float64{
		{30, 30}, // left eye
		{70, 30}, // right eye
		{50, 45}, // nose
		{35, 65}, // left mouth
		{65, 65}, // right mouth
	}
	sideKps := [5][2]float64{
		{30, 30}, // left eye
		{70, 35}, // right eye
		{63, 46}, // nose (shifted)
		{35, 65}, // left mouth
		{65, 72}, // right mouth
	}

	front := EstimateFrontalPoseFactorFromKeypoints(frontKps)
	side := EstimateFrontalPoseFactorFromKeypoints(sideKps)
	if front <= side {
		t.Fatalf("expected frontal keypoints to have higher factor, front=%f side=%f", front, side)
	}
}

func checkerboard(w, h int) image.Image {
	img := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if (x+y)%2 == 0 {
				img.SetGray(x, y, color.Gray{Y: 255})
			} else {
				img.SetGray(x, y, color.Gray{Y: 0})
			}
		}
	}
	return img
}
