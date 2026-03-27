package inference

import (
	"fmt"
	"image"
	"math"

	ort "github.com/yalue/onnxruntime_go"
	"gocv.io/x/gocv"
)

// Recognizer wraps an ArcFace ONNX model for face embedding extraction.
type Recognizer struct {
	session     *ort.DynamicAdvancedSession
	inputName   string
	outputNames []string
	inputSize   int
	inputMean   float32
	inputStd    float32
}

// RecognizerConfig configures the ArcFace recognizer.
type RecognizerConfig struct {
	ModelPath string
	GPU       bool
}

// NewRecognizer loads the ArcFace ONNX model.
func NewRecognizer(cfg RecognizerConfig) (*Recognizer, error) {
	opts, err := SessionOptions(cfg.GPU)
	if err != nil {
		return nil, fmt.Errorf("session options: %w", err)
	}
	defer opts.Destroy()

	inputs, outputs, err := ort.GetInputOutputInfo(cfg.ModelPath)
	if err != nil {
		return nil, fmt.Errorf("inspect recognition model: %w", err)
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
		return nil, fmt.Errorf("create recognizer session: %w", err)
	}

	inputSize := 112
	if len(inputs) > 0 && len(inputs[0].Dimensions) >= 4 {
		h := inputs[0].Dimensions[2]
		if h > 0 {
			inputSize = int(h)
		}
	}

	return &Recognizer{
		session:     session,
		inputName:   inputNames[0],
		outputNames: outputNames,
		inputSize:   inputSize,
		inputMean:   127.5,
		inputStd:    127.5,
	}, nil
}

// GetEmbeddings extracts L2-normalized 512-d embeddings from aligned face images.
// Each face must be an aligned BGR image of size inputSize x inputSize.
func (r *Recognizer) GetEmbeddings(faces []gocv.Mat) ([][]float64, error) {
	if len(faces) == 0 {
		return nil, nil
	}

	batchSize := int64(len(faces))
	h := int64(r.inputSize)
	w := int64(r.inputSize)
	blobMat := gocv.NewMat()
	defer blobMat.Close()

	gocv.BlobFromImages(
		faces,
		&blobMat,
		1.0/float64(r.inputStd),
		image.Point{X: r.inputSize, Y: r.inputSize},
		gocv.NewScalar(float64(r.inputMean), float64(r.inputMean), float64(r.inputMean), 0),
		true,
		false,
		gocv.MatTypeCV32F,
	)
	if blobMat.Empty() {
		return nil, fmt.Errorf("recognizer blob prep: empty blob")
	}

	blobPtr, err := blobMat.DataPtrFloat32()
	if err != nil {
		return nil, fmt.Errorf("recognizer blob prep: %w", err)
	}
	blob := make([]float32, len(blobPtr))
	copy(blob, blobPtr)

	inputShape := ort.NewShape(batchSize, 3, h, w)
	inputTensor, err := ort.NewTensor(inputShape, blob)
	if err != nil {
		return nil, fmt.Errorf("create recognizer input tensor: %w", err)
	}
	defer inputTensor.Destroy()

	outputs := make([]ort.Value, len(r.outputNames))
	if err := r.session.Run([]ort.Value{inputTensor}, outputs); err != nil {
		return nil, fmt.Errorf("recognizer inference: %w", err)
	}
	defer func() {
		for _, o := range outputs {
			if o != nil {
				o.Destroy()
			}
		}
	}()

	outTensor := outputs[0].(*ort.Tensor[float32])
	outData := outTensor.GetData()
	outShape := outTensor.GetShape()

	embDim := int(outShape[1])
	embeddings := make([][]float64, batchSize)

	for b := 0; b < int(batchSize); b++ {
		emb := make([]float64, embDim)
		var norm float64
		for j := 0; j < embDim; j++ {
			v := float64(outData[b*embDim+j])
			emb[j] = v
			norm += v * v
		}
		norm = math.Sqrt(norm)
		if norm < 1e-10 {
			norm = 1e-10
		}
		for j := range emb {
			emb[j] /= norm
		}
		embeddings[b] = emb
	}

	return embeddings, nil
}

// InputSize returns the expected aligned face image size.
func (r *Recognizer) InputSize() int {
	return r.inputSize
}

// Close releases the ONNX session.
func (r *Recognizer) Close() {
	if r.session != nil {
		r.session.Destroy()
	}
}
