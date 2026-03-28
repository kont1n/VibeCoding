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
	Data   []uint8 // BGR interleaved data
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
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

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

	// Convert to BGR
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			c := img.At(x, y)
			r, g, b, _ := c.RGBA()
			// RGBA() returns 16-bit values, convert to 8-bit
			// and store as BGR
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
	// Convert BGR to RGB for encoding
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

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

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
	blob := make([]float32, size*3) // 3 channels

	invStd := 1.0 / std

	for y := 0; y < img.Height; y++ {
		for x := 0; x < img.Width; x++ {
			idx := (y*img.Width + x) * 3
			b := float32(img.Data[idx])
			g := float32(img.Data[idx+1])
			r := float32(img.Data[idx+2])

			dstIdx := y*img.Width + x

			if swapRGB {
				// RGB order (NCHW)
				blob[dstIdx] = (r - mean) * invStd     // R channel
				blob[dstIdx+size] = (g - mean) * invStd // G channel
				blob[dstIdx+size*2] = (b - mean) * invStd // B channel
			} else {
				// BGR order (NCHW)
				blob[dstIdx] = (b - mean) * invStd     // B channel
				blob[dstIdx+size] = (g - mean) * invStd // G channel
				blob[dstIdx+size*2] = (r - mean) * invStd // R channel
			}
		}
	}

	return blob, nil
}

// Crop extracts a region from the image.
func Crop(img *Image, x, y, width, height int) *Image {
	// Clamp to image bounds
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
// M is a 2x3 transformation matrix.
func WarpAffine(img *Image, M [2][3]float64, width, height int) *Image {
	result := NewImage(width, height)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Apply inverse transform to find source coordinates
			srcX := M[0][0]*float64(x) + M[0][1]*float64(y) + M[0][2]
			srcY := M[1][0]*float64(x) + M[1][1]*float64(y) + M[1][2]

			// Bilinear interpolation
			x0 := int(math.Floor(srcX))
			y0 := int(math.Floor(srcY))
			x1 := x0 + 1
			y1 := y0 + 1

			dx := srcX - float64(x0)
			dy := srcY - float64(y0)

			for c := 0; c < 3; c++ {
				var v float64

				// Check bounds and interpolate
				if x0 >= 0 && x0 < img.Width && y0 >= 0 && y0 < img.Height {
					v00 := float64(img.Data[(y0*img.Width+x0)*3+c])
					v01 := 0.0
					v10 := 0.0
					v11 := 0.0

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
					// Out of bounds - use border constant (black)
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
	// Draw horizontal lines
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

	// Draw vertical lines
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
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return SaveImage(img, path, quality)
}
