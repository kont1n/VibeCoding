module github.com/kont1n/face-grouper

go 1.25.0

require (
	github.com/golang-migrate/migrate/v4 v4.19.1
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.9.1
	github.com/joho/godotenv v1.5.1
	github.com/kont1n/face-grouper/platform v0.0.0-00010101000000-000000000000
	github.com/pgvector/pgvector-go v0.3.0
	github.com/yalue/onnxruntime_go v1.23.0
	go.uber.org/zap v1.27.1
	gonum.org/v1/gonum v0.17.0
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/lib/pq v1.10.9 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	golang.org/x/sync v0.18.0 // indirect
	golang.org/x/text v0.31.0 // indirect
)

// Local module replacements
replace github.com/kont1n/face-grouper/platform => ./platform

replace github.com/kont1n/face-grouper/internal => ./internal

replace github.com/kont1n/face-grouper/cmd => ./cmd
