// Package imageutil provides pure Go image processing utilities
// as a replacement for OpenCV/gocv operations.
package imageutil

import (
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"math"
	"os"
	"path/filepath"
)

// Image represents a BGR image with underlying byte data.
// This is used for compatibility with ONNX models expecting BGR input.
type Image struct {
	Data   []uint8 // BGR interleaved data.
	Width  int
	Height int
}

// NewImage creates a new Image with the given dimensions.
func NewImage(width, height int) *Image {
	return &Image{
		Data:   make([]uint8, width*height*3),
		Width:  width,
		Height: height,
	}
}

// LoadImage loads an image from file and converts to BGR format.
func LoadImage(path string) (*Image, error) {
	f, err := os.Open(path) //nolint:gosec
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	return DecodeImage(f)
}

// DecodeImage decodes an image from reader and converts to BGR format.
func DecodeImage(r io.Reader) (*Image, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, err
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	result := NewImage(width, height)

	// Fast path for common formats.
	switch src := img.(type) {
	case *image.NRGBA:
		// NRGBA is already in 8-bit format.
		for y := 0; y < height; y++ {
			row := src.Pix[y*src.Stride : y*src.Stride+width*4]
			for x := 0; x < width; x++ {
				idx := (y*width + x) * 3
				srcIdx := x * 4
				// Convert RGBA to BGR.
				result.Data[idx] = row[srcIdx+2]   // B.
				result.Data[idx+1] = row[srcIdx+1] // G.
				result.Data[idx+2] = row[srcIdx]   // R.
			}
		}
		return result, nil

	case *image.RGBA:
		for y := 0; y < height; y++ {
			row := src.Pix[y*src.Stride : y*src.Stride+width*4]
			for x := 0; x < width; x++ {
				idx := (y*width + x) * 3
				srcIdx := x * 4
				result.Data[idx] = row[srcIdx+2]   // B.
				result.Data[idx+1] = row[srcIdx+1] // G.
				result.Data[idx+2] = row[srcIdx]   // R.
			}
		}
		return result, nil
	}

	// Fallback for other formats (slower but compatible).
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			c := img.At(x, y)
			r, g, b, _ := c.RGBA()
			idx := (y*width + x) * 3
			result.Data[idx] = uint8(b >> 8)
			result.Data[idx+1] = uint8(g >> 8)
			result.Data[idx+2] = uint8(r >> 8)
		}
	}

	return result, nil
}

// SaveImage saves an image to file as JPEG.
func SaveImage(img *Image, path string, quality int) error {
	// Convert BGR to RGB for encoding using direct buffer access.
	rgba := image.NewRGBA(image.Rect(0, 0, img.Width, img.Height))

	// Direct buffer manipulation for speed.
	dstPix := rgba.Pix
	srcData := img.Data

	for y := 0; y < img.Height; y++ {
		srcRow := y * img.Width * 3
		dstRow := y * rgba.Stride
		for x := 0; x < img.Width; x++ {
			srcIdx := srcRow + x*3
			dstIdx := dstRow + x*4
			// Convert BGR to RGBA.
			dstPix[dstIdx] = srcData[srcIdx+2]   // R.
			dstPix[dstIdx+1] = srcData[srcIdx+1] // G.
			dstPix[dstIdx+2] = srcData[srcIdx]   // B.
			dstPix[dstIdx+3] = 255               // A.
		}
	}

	f, err := os.Create(path) //nolint:gosec
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	opts := &jpeg.Options{Quality: quality}
	if quality <= 0 {
		opts.Quality = 90
	}

	return jpeg.Encode(f, rgba, opts)
}

// Resize resizes an image to the specified dimensions using bilinear interpolation.
func Resize(img *Image, width, height int) *Image {
	result := NewImage(width, height)

	scaleX := float64(img.Width) / float64(width)
	scaleY := float64(img.Height) / float64(height)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			srcX := float64(x) * scaleX
			srcY := float64(y) * scaleY

			x0 := int(math.Floor(srcX))
			y0 := int(math.Floor(srcY))
			x1 := x0 + 1
			y1 := y0 + 1

			if x1 >= img.Width {
				x1 = img.Width - 1
			}
			if y1 >= img.Height {
				y1 = img.Height - 1
			}

			dx := srcX - float64(x0)
			dy := srcY - float64(y0)

			for c := 0; c < 3; c++ {
				v00 := float64(img.Data[(y0*img.Width+x0)*3+c])
				v01 := float64(img.Data[(y0*img.Width+x1)*3+c])
				v10 := float64(img.Data[(y1*img.Width+x0)*3+c])
				v11 := float64(img.Data[(y1*img.Width+x1)*3+c])

				v0 := v00*(1-dx) + v01*dx
				v1 := v10*(1-dx) + v11*dx
				v := v0*(1-dy) + v1*dy

				result.Data[(y*width+x)*3+c] = uint8(math.Round(v))
			}
		}
	}

	return result
}

// BlobFromImage converts an image to NCHW blob format with normalization.
// mean and std are applied as: (pixel - mean) / std
// If swapRGB is true, converts BGR to RGB.
func BlobFromImage(img *Image, mean, std float32, swapRGB bool) ([]float32, error) {
	size := img.Width * img.Height
	blob := make([]float32, size*3) // 3 channels.

	invStd := 1.0 / std

	for y := 0; y < img.Height; y++ {
		for x := 0; x < img.Width; x++ {
			idx := (y*img.Width + x) * 3
			b := float32(img.Data[idx])
			g := float32(img.Data[idx+1])
			r := float32(img.Data[idx+2])

			dstIdx := y*img.Width + x

			if swapRGB {
				// RGB order (NCHW).
				blob[dstIdx] = (r - mean) * invStd        // R channel.
				blob[dstIdx+size] = (g - mean) * invStd   // G channel.
				blob[dstIdx+size*2] = (b - mean) * invStd // B channel.
			} else {
				// BGR order (NCHW).
				blob[dstIdx] = (b - mean) * invStd        // B channel.
				blob[dstIdx+size] = (g - mean) * invStd   // G channel.
				blob[dstIdx+size*2] = (r - mean) * invStd // R channel.
			}
		}
	}

	return blob, nil
}

// Crop extracts a region from the image.
func Crop(img *Image, x, y, width, height int) *Image {
	// Clamp to image bounds.
	if x < 0 {
		width += x
		x = 0
	}
	if y < 0 {
		height += y
		y = 0
	}
	if x+width > img.Width {
		width = img.Width - x
	}
	if y+height > img.Height {
		height = img.Height - y
	}

	if width <= 0 || height <= 0 {
		return nil
	}

	result := NewImage(width, height)
	for cy := 0; cy < height; cy++ {
		for cx := 0; cx < width; cx++ {
			srcIdx := ((y+cy)*img.Width + (x + cx)) * 3
			dstIdx := (cy*width + cx) * 3
			result.Data[dstIdx] = img.Data[srcIdx]
			result.Data[dstIdx+1] = img.Data[srcIdx+1]
			result.Data[dstIdx+2] = img.Data[srcIdx+2]
		}
	}

	return result
}

// WarpAffine applies an affine transformation to the image.
// m is a 2x3 transformation matrix (src -> dst). We compute inverse (dst -> src).
func WarpAffine(img *Image, m [2][3]float64, width, height int) *Image {
	result := NewImage(width, height)

	// Compute inverse of the 2x3 affine matrix.
	// m = [a b tx; c d ty] represents: dst.x = a*src.x + b*src.y + tx.
	// We need inverse: src.x = a'*dst.x + b'*dst.y + tx'.
	a, b, tx := m[0][0], m[0][1], m[0][2]
	c, d, ty := m[1][0], m[1][1], m[1][2]

	// Compute determinant of 2x2 linear part.
	det := a*d - b*c
	if det == 0 {
		det = 1e-10 // Avoid division by zero.
	}

	// Inverse of 2x2 matrix: [a b; c d]^-1 = 1/det * [d -b; -c a].
	// For affine transform with translation.
	invDet := 1.0 / det
	invA := d * invDet
	invB := -b * invDet
	invTx := -(d*tx - b*ty) * invDet
	invC := -c * invDet
	invD := a * invDet
	invTy := -(-c*tx + a*ty) * invDet

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Apply inverse transform to find source coordinates.
			srcX := invA*float64(x) + invB*float64(y) + invTx
			srcY := invC*float64(x) + invD*float64(y) + invTy

			// Bilinear interpolation.
			x0 := int(math.Floor(srcX))
			y0 := int(math.Floor(srcY))
			x1 := x0 + 1
			y1 := y0 + 1

			dx := srcX - float64(x0)
			dy := srcY - float64(y0)

			for c := 0; c < 3; c++ {
				var v float64

				// Check bounds and interpolate.
				if x0 >= 0 && x0 < img.Width && y0 >= 0 && y0 < img.Height {
					v00 := float64(img.Data[(y0*img.Width+x0)*3+c])
					var v01, v10, v11 float64

					if x1 < img.Width {
						v01 = float64(img.Data[(y0*img.Width+x1)*3+c])
					} else {
						v01 = v00
					}

					if y1 < img.Height {
						v10 = float64(img.Data[(y1*img.Width+x0)*3+c])
						if x1 < img.Width {
							v11 = float64(img.Data[(y1*img.Width+x1)*3+c])
						} else {
							v11 = v10
						}
					} else {
						v10 = v00
						v11 = v01
					}

					v0 := v00*(1-dx) + v01*dx
					v1 := v10*(1-dx) + v11*dx
					v = v0*(1-dy) + v1*dy
				} else {
					// Out of bounds - use border constant (black).
					v = 0
				}

				result.Data[(y*width+x)*3+c] = uint8(math.Round(v))
			}
		}
	}

	return result
}

// Region creates a copy of a rectangular region from the image.
func (img *Image) Region(rect image.Rectangle) *Image {
	return Crop(img, rect.Min.X, rect.Min.Y, rect.Dx(), rect.Dy())
}

// Empty returns true if the image has no data.
func (img *Image) Empty() bool {
	return img == nil || img.Data == nil || img.Width <= 0 || img.Height <= 0
}

// Close releases resources (no-op for pure Go, but kept for API compatibility).
func (img *Image) Close() {
	img.Data = nil
}

// ToRGBA converts the image to RGBA format.
func (img *Image) ToRGBA() *image.RGBA {
	rgba := image.NewRGBA(image.Rect(0, 0, img.Width, img.Height))
	for y := 0; y < img.Height; y++ {
		for x := 0; x < img.Width; x++ {
			idx := (y*img.Width + x) * 3
			rgba.Set(x, y, color.RGBA{
				R: img.Data[idx+2],
				G: img.Data[idx+1],
				B: img.Data[idx],
				A: 255,
			})
		}
	}
	return rgba
}

// DrawRectangle draws a rectangle on the image (in-place).
func (img *Image) DrawRectangle(x1, y1, x2, y2 int, r, g, b uint8) {
	// Draw horizontal lines.
	for x := x1; x <= x2 && x < img.Width; x++ {
		if y1 >= 0 && y1 < img.Height {
			idx := (y1*img.Width + x) * 3
			img.Data[idx] = b
			img.Data[idx+1] = g
			img.Data[idx+2] = r
		}
		if y2 >= 0 && y2 < img.Height {
			idx := (y2*img.Width + x) * 3
			img.Data[idx] = b
			img.Data[idx+1] = g
			img.Data[idx+2] = r
		}
	}

	// Draw vertical lines.
	for y := y1; y <= y2 && y < img.Height; y++ {
		if x1 >= 0 && x1 < img.Width {
			idx := (y*img.Width + x1) * 3
			img.Data[idx] = b
			img.Data[idx+1] = g
			img.Data[idx+2] = r
		}
		if x2 >= 0 && x2 < img.Width {
			idx := (y*img.Width + x2) * 3
			img.Data[idx] = b
			img.Data[idx+1] = g
			img.Data[idx+2] = r
		}
	}
}

// SaveJPEG saves an image as JPEG to the specified path.
func SaveJPEG(img *Image, path string, quality int) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	return SaveImage(img, path, quality)
}

// ValidateImageHeader validates that the given bytes represent a valid image.
// Checks magic bytes for JPEG, PNG, and WebP formats.
func ValidateImageHeader(data []byte) bool {
	if len(data) < 12 {
		return false
	}

	// JPEG: FF D8 FF.
	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return true
	}

	// PNG: 89 50 4E 47 0D 0A 1A 0A.
	if len(data) >= 8 && data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E &&
		data[3] == 0x47 && data[4] == 0x0D && data[5] == 0x0A && data[6] == 0x1A && data[7] == 0x0A {
		return true
	}

	// WebP: RIFF .... WEBP.
	if len(data) >= 12 && data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 &&
		data[3] == 0x46 && data[8] == 0x57 && data[9] == 0x45 && data[10] == 0x42 && data[11] == 0x50 {
		return true
	}

	// GIF: 47 49 46 38.
	if len(data) >= 4 && data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x38 {
		return true
	}

	// BMP: 42 4D.
	if len(data) >= 2 && data[0] == 0x42 && data[1] == 0x4D {
		return true
	}

	return false
}
