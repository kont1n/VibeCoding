package inference

import (
	"fmt"
	"image"
	"sync"

	"gocv.io/x/gocv"
	ort "github.com/yalue/onnxruntime_go"
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
	GPU       bool
	DetThresh float32
	NMSThresh float32
}

// NewDetector loads the SCRFD ONNX model and inspects its outputs to determine
// the FPN configuration (strides, anchors, keypoints support).
func NewDetector(cfg DetectorConfig) (*Detector, error) {
	opts := SessionOptions(cfg.GPU)

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

// Detect runs face detection on a BGR gocv.Mat.
// Returns detections in original image coordinates.
func (d *Detector) Detect(img gocv.Mat) ([]Detection, error) {
	imgH := img.Rows()
	imgW := img.Cols()
	detW := int(d.inputW)
	detH := int(d.inputH)

	// Compute letterbox resize keeping aspect ratio
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

	resized := gocv.NewMat()
	defer resized.Close()
	gocv.Resize(img, &resized, image.Point{X: newW, Y: newH}, 0, 0, gocv.InterpolationLinear)

	// Paste resized image onto zero-padded canvas
	detImg := gocv.NewMatWithSize(detH, detW, img.Type())
	defer detImg.Close()
	roi := detImg.Region(image.Rect(0, 0, newW, newH))
	resized.CopyTo(&roi)
	roi.Close()

	blob := imgToNCHW(detImg, 127.5, 128.0)

	inputShape := ort.NewShape(1, 3, d.inputH, d.inputW)
	inputTensor, err := ort.NewTensor(inputShape, blob)
	if err != nil {
		return nil, fmt.Errorf("create input tensor: %w", err)
	}
	defer inputTensor.Destroy()

	outputs := make([]ort.Value, len(d.outputNames))
	if err := d.session.Run([]ort.Value{inputTensor}, outputs); err != nil {
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
	if cached, ok := d.centerCache[key]; ok {
		d.centerCacheMu.Unlock()
		return cached
	}
	d.centerCacheMu.Unlock()

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

	d.centerCacheMu.Lock()
	d.centerCache[key] = centers
	d.centerCacheMu.Unlock()
	return centers
}

// imgToNCHW converts a BGR gocv.Mat to a float32 NCHW blob with
// normalization: (pixel - mean) / std, and BGR->RGB channel swap.
func imgToNCHW(img gocv.Mat, mean, std float32) []float32 {
	rows := img.Rows()
	cols := img.Cols()
	ch := img.Channels()
	planeSize := rows * cols
	blob := make([]float32, 3*planeSize)

	for y := 0; y < rows; y++ {
		for x := 0; x < cols; x++ {
			off := y*cols + x
			b := float32(img.GetUCharAt(y, x*ch+0))
			g := float32(img.GetUCharAt(y, x*ch+1))
			r := float32(img.GetUCharAt(y, x*ch+2))

			blob[0*planeSize+off] = (r - mean) / std
			blob[1*planeSize+off] = (g - mean) / std
			blob[2*planeSize+off] = (b - mean) / std
		}
	}
	return blob
}
