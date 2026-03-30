package ml

import (
	"fmt"

	"github.com/kont1n/face-grouper/internal/service/imageutil"
)

// DetectorRepository определяет интерфейс для детекции лиц.
type DetectorRepository interface {
	Detect(img *imageutil.Image) ([]Detection, error)
	Close()
}

type detectorRepository struct {
	detector *Detector
}

// NewDetectorRepository создаёт новый экземпляр репозитория детектора.
func NewDetectorRepository(cfg DetectorConfig) (DetectorRepository, error) {
	det, err := NewDetector(cfg)
	if err != nil {
		return nil, fmt.Errorf("create detector: %w", err)
	}

	return &detectorRepository{detector: det}, nil
}

// Detect выполняет детекцию лиц на изображении.
func (r *detectorRepository) Detect(img *imageutil.Image) ([]Detection, error) {
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
	recognizer *Recognizer
}

// NewRecognizerRepository создаёт новый экземпляр репозитория распознавания.
func NewRecognizerRepository(cfg RecognizerConfig) (RecognizerRepository, error) {
	rec, err := NewRecognizer(cfg)
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
