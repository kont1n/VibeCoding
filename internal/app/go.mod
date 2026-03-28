module github.com/kont1n/face-grouper/internal/app

go 1.25.0

require (
	github.com/kont1n/face-grouper/internal/api/cli v0.0.0
	github.com/kont1n/face-grouper/internal/config v0.0.0
	github.com/kont1n/face-grouper/internal/inference v0.0.0
	github.com/kont1n/face-grouper/internal/report v0.0.0
	github.com/kont1n/face-grouper/internal/repository/inference v0.0.0
	github.com/kont1n/face-grouper/internal/service/clustering v0.0.0
	github.com/kont1n/face-grouper/internal/service/extraction v0.0.0
	github.com/kont1n/face-grouper/internal/service/organization v0.0.0
	github.com/kont1n/face-grouper/internal/service/scan v0.0.0
	github.com/kont1n/face-grouper/platform v0.0.0
	go.uber.org/zap v1.27.1
)

// Replace with local modules
replace github.com/kont1n/face-grouper/internal/api/cli => ../api/cli
replace github.com/kont1n/face-grouper/internal/config => ../config
replace github.com/kont1n/face-grouper/internal/inference => ../inference
replace github.com/kont1n/face-grouper/internal/report => ../report
replace github.com/kont1n/face-grouper/internal/repository/inference => ../repository/inference
replace github.com/kont1n/face-grouper/internal/service/clustering => ../service/clustering
replace github.com/kont1n/face-grouper/internal/service/extraction => ../service/extraction
replace github.com/kont1n/face-grouper/internal/service/organization => ../service/organization
replace github.com/kont1n/face-grouper/internal/service/scan => ../service/scan
replace github.com/kont1n/face-grouper/platform => ../../platform
