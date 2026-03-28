module github.com/kont1n/face-grouper

go 1.25.0

require (
	github.com/joho/godotenv v1.5.1
	github.com/kont1n/face-grouper/platform v0.0.0-00010101000000-000000000000
	github.com/yalue/onnxruntime_go v1.23.0
	go.uber.org/zap v1.27.1
	gonum.org/v1/gonum v0.17.0
)

require go.uber.org/multierr v1.10.0 // indirect

// Local module replacements
replace github.com/kont1n/face-grouper/platform => ./platform

replace github.com/kont1n/face-grouper/internal => ./internal

replace github.com/kont1n/face-grouper/cmd => ./cmd
