package ml

import (
	"fmt"
	"sync"

	ort "github.com/yalue/onnxruntime_go"

	"github.com/kont1n/face-grouper/internal/service/imageutil"
)

// Detector wraps an SCRFD ONNX model for face detection.
type Detector struct {
	session *ort.DynamicAdvancedSession

	inputName   string
	outputNames []string
	inputW      int64
	inputH      int64

	fmc         int
	featStrides []int
	numAnchors  int
	useKps      bool
	nmsThresh   float32
	detThresh   float32

	centerCache   map[[3]int][][2]float32
	centerCacheMu sync.Mutex
}

// DetectorConfig configures the SCRFD detector.
type DetectorConfig struct {
	ModelPath string
	Provider  ProviderConfig
	DetThresh float32
	NMSThresh float32
}

// NewDetector loads the SCRFD ONNX model and inspects its outputs to determine
// the FPN configuration (strides, anchors, keypoints support).
func NewDetector(cfg DetectorConfig) (*Detector, error) {
	opts, err := SessionOptions(cfg.Provider)
	if err != nil {
		return nil, fmt.Errorf("session options: %w", err)
	}
	defer opts.Destroy()

	inputs, outputs, err := ort.GetInputOutputInfo(cfg.ModelPath)
	if err != nil {
		return nil, fmt.Errorf("inspect model: %w", err)
	}

	inputNames := make([]string, len(inputs))
	outputNames := make([]string, len(outputs))
	for i, in := range inputs {
		inputNames[i] = in.Name
	}
	for i, out := range outputs {
		outputNames[i] = out.Name
	}

	session, err := ort.NewDynamicAdvancedSession(cfg.ModelPath, inputNames, outputNames, opts)
	if err != nil {
		return nil, fmt.Errorf("create detector session: %w", err)
	}

	d := &Detector{
		session:     session,
		inputName:   inputNames[0],
		outputNames: outputNames,
		inputW:      640,
		inputH:      640,
		nmsThresh:   0.4,
		detThresh:   0.5,
		centerCache: make(map[[3]int][][2]float32),
	}

	if cfg.DetThresh > 0 {
		d.detThresh = cfg.DetThresh
	}
	if cfg.NMSThresh > 0 {
		d.nmsThresh = cfg.NMSThresh
	}

	numOutputs := len(outputNames)
	switch numOutputs {
	case 6:
		d.fmc = 3
		d.featStrides = []int{8, 16, 32}
		d.numAnchors = 2
	case 9:
		d.fmc = 3
		d.featStrides = []int{8, 16, 32}
		d.numAnchors = 2
		d.useKps = true
	case 10:
		d.fmc = 5
		d.featStrides = []int{8, 16, 32, 64, 128}
		d.numAnchors = 1
	case 15:
		d.fmc = 5
		d.featStrides = []int{8, 16, 32, 64, 128}
		d.numAnchors = 1
		d.useKps = true
	default:
		session.Destroy()
		return nil, fmt.Errorf("unsupported SCRFD output count: %d", numOutputs)
	}

	return d, nil
}

// Detect runs face detection on an imageutil.Image.
// Returns detections in original image coordinates.
func (d *Detector) Detect(img *imageutil.Image) ([]Detection, error) {
	imgH := img.Height
	imgW := img.Width
	detW := int(d.inputW)
	detH := int(d.inputH)

	// Compute letterbox resize keeping aspect ratio.
	imRatio := float32(imgH) / float32(imgW)
	modelRatio := float32(detH) / float32(detW)

	var newW, newH int
	if imRatio > modelRatio {
		newH = detH
		newW = int(float32(newH) / imRatio)
	} else {
		newW = detW
		newH = int(float32(newW) * imRatio)
	}
	detScale := float32(newH) / float32(imgH)

	// Resize image.
	resized := imageutil.Resize(img, newW, newH)
	defer resized.Close()

	// Create canvas and paste resized image.
	detImg := imageutil.NewImage(detW, detH)
	defer detImg.Close()

	// Copy resized image to canvas (BGR format).
	for y := 0; y < newH; y++ {
		for x := 0; x < newW; x++ {
			srcIdx := (y*newW + x) * 3
			dstIdx := (y*detW + x) * 3
			detImg.Data[dstIdx] = resized.Data[srcIdx]
			detImg.Data[dstIdx+1] = resized.Data[srcIdx+1]
			detImg.Data[dstIdx+2] = resized.Data[srcIdx+2]
		}
	}

	blob, err := blobFromImageNCHW(detImg, 127.5, 128.0)
	if err != nil {
		return nil, fmt.Errorf("detector blob prep: %w", err)
	}

	inputShape := ort.NewShape(1, 3, d.inputH, d.inputW)
	inputTensor, err := ort.NewTensor(inputShape, blob)
	if err != nil {
		return nil, fmt.Errorf("create input tensor: %w", err)
	}
	defer func() {
		inputTensor.Destroy()
	}()

	outputs := make([]ort.Value, len(d.outputNames))
	if err := d.session.Run([]ort.Value{inputTensor}, outputs); err != nil {
		for _, o := range outputs {
			if o != nil {
				o.Destroy()
			}
		}
		return nil, fmt.Errorf("detector inference: %w", err)
	}
	defer func() {
		for _, o := range outputs {
			if o != nil {
				o.Destroy()
			}
		}
	}()

	var allDets []Detection
	for idx, stride := range d.featStrides {
		scoresTensor := outputs[idx].(*ort.Tensor[float32])
		bboxTensor := outputs[idx+d.fmc].(*ort.Tensor[float32])

		scores := scoresTensor.GetData()
		bboxPreds := bboxTensor.GetData()

		height := int(d.inputH) / stride
		width := int(d.inputW) / stride
		anchorCenters := d.getAnchorCenters(height, width, stride)

		var kpsData []float32
		if d.useKps {
			kpsTensor := outputs[idx+d.fmc*2].(*ort.Tensor[float32])
			kpsData = kpsTensor.GetData()
		}

		for i, ac := range anchorCenters {
			if scores[i] < d.detThresh {
				continue
			}

			cx, cy := ac[0], ac[1]
			st := float32(stride)

			det := Detection{
				X1:    (cx - bboxPreds[i*4+0]*st) / detScale,
				Y1:    (cy - bboxPreds[i*4+1]*st) / detScale,
				X2:    (cx + bboxPreds[i*4+2]*st) / detScale,
				Y2:    (cy + bboxPreds[i*4+3]*st) / detScale,
				Score: scores[i],
			}

			if d.useKps && kpsData != nil {
				for k := 0; k < 5; k++ {
					det.Kps[k][0] = (cx + kpsData[i*10+k*2]*st) / detScale
					det.Kps[k][1] = (cy + kpsData[i*10+k*2+1]*st) / detScale
				}
			}

			allDets = append(allDets, det)
		}
	}

	return NMS(allDets, d.nmsThresh), nil
}

// Close releases the ONNX session.
func (d *Detector) Close() {
	if d.session != nil {
		d.session.Destroy()
	}
}

func (d *Detector) getAnchorCenters(height, width, stride int) [][2]float32 {
	key := [3]int{height, width, stride}
	d.centerCacheMu.Lock()
	defer d.centerCacheMu.Unlock()

	if cached, ok := d.centerCache[key]; ok {
		return cached
	}

	centers := make([][2]float32, 0, height*width*d.numAnchors)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			for a := 0; a < d.numAnchors; a++ {
				centers = append(centers, [2]float32{
					float32(x * stride),
					float32(y * stride),
				})
			}
		}
	}

	d.centerCache[key] = centers
	return centers
}

// blobFromImageNCHW converts a BGR image to float32 NCHW blob:
// (pixel - mean) / std and BGR->RGB channel swap.
func blobFromImageNCHW(img *imageutil.Image, mean, std float32) ([]float32, error) {
	if img.Empty() {
		return nil, fmt.Errorf("empty input image")
	}
	if std == 0 {
		return nil, fmt.Errorf("std must be non-zero")
	}

	return imageutil.BlobFromImage(img, mean, std, true)
}
