[CmdletBinding()]
param(
    [string]$Msys2Bin = "",
    [string]$Output = "face-grouper.exe",
    [switch]$SkipClean
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
Push-Location $repoRoot

try {
    $candidateBins = @()
    if ([string]::IsNullOrWhiteSpace($Msys2Bin)) {
        $candidateBins = @(
            "C:\msys64\ucrt64\bin",
            "C:\msys64\mingw64\bin",
            "C:\msys64\clang64\bin"
        )
    } else {
        $candidateBins = @($Msys2Bin)
    }

    $resolvedBin = ""
    foreach ($bin in $candidateBins) {
        if (-not (Test-Path -LiteralPath $bin)) {
            continue
        }
        $gccPath = Join-Path $bin "gcc.exe"
        $gppPath = Join-Path $bin "g++.exe"
        $pkgPath = Join-Path $bin "pkg-config.exe"
        if ((Test-Path -LiteralPath $gccPath) -and (Test-Path -LiteralPath $gppPath) -and (Test-Path -LiteralPath $pkgPath)) {
            $resolvedBin = $bin
            break
        }
    }

    if ([string]::IsNullOrWhiteSpace($resolvedBin)) {
        throw "MSYS2 toolchain not found. Install MSYS2 packages and/or pass -Msys2Bin explicitly."
    }

    $env:Path = "$resolvedBin;$env:Path"
    $env:CGO_ENABLED = "1"
    $env:CC = "gcc"
    $env:CXX = "g++"

    if (-not (Get-Command gcc -ErrorAction SilentlyContinue)) {
        throw "gcc not found in PATH. Check MSYS2 installation and -Msys2Bin value."
    }
    if (-not (Get-Command g++ -ErrorAction SilentlyContinue)) {
        throw "g++ not found in PATH. Check MSYS2 installation and -Msys2Bin value."
    }
    if (-not (Get-Command pkg-config -ErrorAction SilentlyContinue)) {
        throw "pkg-config not found in PATH. Install pkgconf for your MSYS2 toolchain."
    }

    $cflags = (& pkg-config --cflags opencv4).Trim()
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($cflags)) {
        throw "pkg-config failed to resolve opencv4 cflags. Ensure OpenCV is installed for selected MSYS2 toolchain."
    }
    $ldflags = (& pkg-config --libs opencv4).Trim()
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($ldflags)) {
        throw "pkg-config failed to resolve opencv4 libs. Ensure OpenCV is installed for selected MSYS2 toolchain."
    }

    $env:CGO_CFLAGS = $cflags
    $env:CGO_CXXFLAGS = $cflags
    $env:CGO_LDFLAGS = $ldflags

    Write-Host "Building in $repoRoot"
    Write-Host "MSYS2 bin: $resolvedBin"
    Write-Host "CGO_ENABLED=$env:CGO_ENABLED CC=$env:CC CXX=$env:CXX"
    Write-Host "Using OpenCV flags from pkg-config (opencv4)"

    if (-not $SkipClean) {
        & go clean -cache
        if ($LASTEXITCODE -ne 0) {
            throw "go clean -cache failed."
        }
    }

    & go build -tags customenv -o $Output .
    if ($LASTEXITCODE -ne 0) {
        throw "go build failed."
    }

    Write-Host "Build completed: $Output"
}
finally {
    Pop-Location
}
