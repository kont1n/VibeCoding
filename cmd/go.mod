module github.com/kont1n/face-grouper/cmd

go 1.25.0

require (
	github.com/kont1n/face-grouper/internal/app v0.0.0
	github.com/kont1n/face-grouper/internal/config v0.0.0
	github.com/kont1n/face-grouper/platform v0.0.0
)

// Replace with local modules
replace github.com/kont1n/face-grouper/internal/app => ../internal/app
replace github.com/kont1n/face-grouper/internal/config => ../internal/config
replace github.com/kont1n/face-grouper/platform => ../platform
