package inference

import (
	"math"

	"github.com/kont1n/face-grouper/internal/imageutil"
)

// Standard ArcFace destination landmarks for 112x112 image.
var arcfaceDst = [5][2]float64{
	{38.2946, 51.6963},
	{73.5318, 51.5014},
	{56.0252, 71.7366},
	{41.5493, 92.3655},
	{70.7299, 92.2041},
}

// NormCrop aligns a face using 5 keypoints via similarity transform,
// producing an imageSize x imageSize BGR image suitable for ArcFace.
func NormCrop(img *imageutil.Image, kps [5][2]float32, imageSize int) *imageutil.Image {
	ratio := float64(imageSize) / 112.0
	dst := make([][2]float64, 5)
	for i := range arcfaceDst {
		dst[i][0] = arcfaceDst[i][0] * ratio
		dst[i][1] = arcfaceDst[i][1] * ratio
	}

	src := make([][2]float64, 5)
	for i := range kps {
		src[i][0] = float64(kps[i][0])
		src[i][1] = float64(kps[i][1])
	}

	M := estimateSimilarityTransform(src, dst)

	warped := imageutil.WarpAffine(img, M, imageSize, imageSize)

	return warped
}

// estimateSimilarityTransform computes a 2x3 similarity transform matrix
// mapping src points to dst points using the Umeyama algorithm.
func estimateSimilarityTransform(src, dst [][2]float64) [2][3]float64 {
	n := len(src)
	if n < 2 {
		return [2][3]float64{}
	}

	var srcMean, dstMean [2]float64
	for i := 0; i < n; i++ {
		srcMean[0] += src[i][0]
		srcMean[1] += src[i][1]
		dstMean[0] += dst[i][0]
		dstMean[1] += dst[i][1]
	}
	fn := float64(n)
	srcMean[0] /= fn
	srcMean[1] /= fn
	dstMean[0] /= fn
	dstMean[1] /= fn

	srcC := make([][2]float64, n)
	dstC := make([][2]float64, n)
	var srcVar float64
	for i := 0; i < n; i++ {
		srcC[i][0] = src[i][0] - srcMean[0]
		srcC[i][1] = src[i][1] - srcMean[1]
		dstC[i][0] = dst[i][0] - dstMean[0]
		dstC[i][1] = dst[i][1] - dstMean[1]
		srcVar += srcC[i][0]*srcC[i][0] + srcC[i][1]*srcC[i][1]
	}
	srcVar /= fn
	if srcVar < 1e-10 {
		// Source points are nearly identical; return identity transform
		return [2][3]float64{
			{1, 0, dstMean[0] - srcMean[0]},
			{0, 1, dstMean[1] - srcMean[1]},
		}
	}

	// Cross-covariance matrix (2x2): C = (1/n) * dst^T * src
	var a, b, c, d float64
	for i := 0; i < n; i++ {
		a += dstC[i][0] * srcC[i][0]
		b += dstC[i][0] * srcC[i][1]
		c += dstC[i][1] * srcC[i][0]
		d += dstC[i][1] * srcC[i][0]
	}
	a /= fn
	b /= fn
	c /= fn
	d /= fn

	u, s, vt := svd2x2(a, b, c, d)

	detU := u[0][0]*u[1][1] - u[0][1]*u[1][0]
	detVt := vt[0][0]*vt[1][1] - vt[0][1]*vt[1][0]
	if detU*detVt < 0 {
		s[1] = -s[1]
		u[0][1] = -u[0][1]
		u[1][1] = -u[1][1]
	}

	// R = U * Vt
	r00 := u[0][0]*vt[0][0] + u[0][1]*vt[1][0]
	r01 := u[0][0]*vt[0][1] + u[0][1]*vt[1][1]
	r10 := u[1][0]*vt[0][0] + u[1][1]*vt[1][0]
	r11 := u[1][0]*vt[0][1] + u[1][1]*vt[1][1]

	scale := (s[0] + s[1]) / srcVar

	tx := dstMean[0] - scale*(r00*srcMean[0]+r01*srcMean[1])
	ty := dstMean[1] - scale*(r10*srcMean[0]+r11*srcMean[1])

	return [2][3]float64{
		{scale * r00, scale * r01, tx},
		{scale * r10, scale * r11, ty},
	}
}

// svd2x2 computes the SVD of a 2x2 matrix [[a,b],[c,d]].
func svd2x2(a, b, c, d float64) ([2][2]float64, [2]float64, [2][2]float64) {
	s1 := a*a + b*b + c*c + d*d
	s2 := math.Sqrt((a*a+b*b-c*c-d*d)*(a*a+b*b-c*c-d*d) + 4*(a*c+b*d)*(a*c+b*d))

	sigma1 := math.Sqrt((s1 + s2) / 2)
	sigma2 := math.Sqrt(math.Max((s1-s2)/2, 0))

	theta := 0.5 * math.Atan2(2*(a*c+b*d), a*a+b*b-c*c-d*d)
	phi := 0.5 * math.Atan2(2*(a*b+c*d), a*a-b*b+c*c-d*d)

	cosTheta := math.Cos(theta)
	sinTheta := math.Sin(theta)
	cosPhi := math.Cos(phi)
	sinPhi := math.Sin(phi)

	s11 := cosTheta*(a*cosPhi+b*sinPhi) + sinTheta*(c*cosPhi+d*sinPhi)
	s22 := cosTheta*(-a*sinPhi+b*cosPhi) + sinTheta*(-c*sinPhi+d*cosPhi)

	if s11 < 0 {
		sigma1 = -sigma1
	}
	if s22 < 0 {
		sigma2 = -sigma2
	}

	U := [2][2]float64{
		{cosTheta, -sinTheta},
		{sinTheta, cosTheta},
	}
	Vt := [2][2]float64{
		{cosPhi, sinPhi},
		{-sinPhi, cosPhi},
	}

	if sigma1 < 0 {
		sigma1 = -sigma1
		U[0][0] = -U[0][0]
		U[1][0] = -U[1][0]
	}
	if sigma2 < 0 {
		sigma2 = -sigma2
		U[0][1] = -U[0][1]
		U[1][1] = -U[1][1]
	}

	return U, [2]float64{sigma1, sigma2}, Vt
}
