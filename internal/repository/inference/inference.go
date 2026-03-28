package inference

import (
	"fmt"

	ort "github.com/yalue/onnxruntime_go"

	"github.com/kont1n/face-grouper/internal/imageutil"
	"github.com/kont1n/face-grouper/internal/inference"
)

// DetectorRepository определяет интерфейс для детекции лиц.
type DetectorRepository interface {
	Detect(img *imageutil.Image) ([]inference.Detection, error)
	Close()
}

type detectorRepository struct {
	detector *inference.Detector
}

// NewDetectorRepository создаёт новый экземпляр репозитория детектора.
func NewDetectorRepository(cfg inference.DetectorConfig) (DetectorRepository, error) {
	det, err := inference.NewDetector(cfg)
	if err != nil {
		return nil, fmt.Errorf("create detector: %w", err)
	}

	return &detectorRepository{detector: det}, nil
}

// Detect выполняет детекцию лиц на изображении.
func (r *detectorRepository) Detect(img *imageutil.Image) ([]inference.Detection, error) {
	return r.detector.Detect(img)
}

// Close закрывает ресурсы детектора.
func (r *detectorRepository) Close() {
	r.detector.Close()
}

// RecognizerRepository определяет интерфейс для распознавания лиц.
type RecognizerRepository interface {
	GetEmbeddings(imgs []*imageutil.Image) ([][]float64, error)
	InputSize() int
	Close()
}

type recognizerRepository struct {
	recognizer *inference.Recognizer
}

// NewRecognizerRepository создаёт новый экземпляр репозитория распознавания.
func NewRecognizerRepository(cfg inference.RecognizerConfig) (RecognizerRepository, error) {
	rec, err := inference.NewRecognizer(cfg)
	if err != nil {
		return nil, fmt.Errorf("create recognizer: %w", err)
	}

	return &recognizerRepository{recognizer: rec}, nil
}

// GetEmbeddings извлекает эмбеддинги из изображений лиц.
func (r *recognizerRepository) GetEmbeddings(imgs []*imageutil.Image) ([][]float64, error) {
	return r.recognizer.GetEmbeddings(imgs)
}

// InputSize возвращает размер входного изображения для модели.
func (r *recognizerRepository) InputSize() int {
	return r.recognizer.InputSize()
}

// Close закрывает ресурсы распознавателя.
func (r *recognizerRepository) Close() {
	r.recognizer.Close()
}

// DestroyORT освобождаем ресурсы ONNX Runtime.
func DestroyORT() {
	inference.DestroyORT()
}

// InitORT инициализируем ONNX Runtime.
func InitORT(libPath string) error {
	return inference.InitORT(libPath)
}

// SessionOptions создаем опции сессии.
func SessionOptions(gpu bool) (*ort.SessionOptions, error) {
	return inference.SessionOptions(gpu)
}
