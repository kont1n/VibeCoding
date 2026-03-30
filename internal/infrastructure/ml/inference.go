package ml

import (
	"fmt"

	"github.com/kont1n/face-grouper/internal/service/imageutil"
)

// DetectorGateway определяет интерфейс для детекции лиц.
type DetectorGateway interface {
	Detect(img *imageutil.Image) ([]Detection, error)
	Close()
}

type detectorGateway struct {
	detector *Detector
}

// NewDetectorGateway создаёт новый экземпляр детектора.
func NewDetectorGateway(cfg DetectorConfig) (DetectorGateway, error) {
	det, err := NewDetector(cfg)
	if err != nil {
		return nil, fmt.Errorf("create detector: %w", err)
	}

	return &detectorGateway{detector: det}, nil
}

// Detect выполняет детекцию лиц на изображении.
func (g *detectorGateway) Detect(img *imageutil.Image) ([]Detection, error) {
	return g.detector.Detect(img)
}

// Close закрывает ресурсы детектора.
func (g *detectorGateway) Close() {
	g.detector.Close()
}

// RecognizerGateway определяет интерфейс для распознавания лиц.
type RecognizerGateway interface {
	GetEmbeddings(imgs []*imageutil.Image) ([][]float32, error)
	InputSize() int
	Close()
}

type recognizerGateway struct {
	recognizer *Recognizer
}

// NewRecognizerGateway создаёт новый экземпляр распознавателя.
func NewRecognizerGateway(cfg RecognizerConfig) (RecognizerGateway, error) {
	rec, err := NewRecognizer(cfg)
	if err != nil {
		return nil, fmt.Errorf("create recognizer: %w", err)
	}

	return &recognizerGateway{recognizer: rec}, nil
}

// GetEmbeddings извлекает эмбеддинги из изображений лиц.
func (g *recognizerGateway) GetEmbeddings(imgs []*imageutil.Image) ([][]float32, error) {
	return g.recognizer.GetEmbeddings(imgs)
}

// InputSize возвращает размер входного изображения для модели.
func (g *recognizerGateway) InputSize() int {
	return g.recognizer.InputSize()
}

// Close закрывает ресурсы распознавателя.
func (g *recognizerGateway) Close() {
	g.recognizer.Close()
}
