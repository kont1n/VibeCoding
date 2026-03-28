[CmdletBinding()]
param(
    [string]$OutputDir = "./models"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$models = @(
    @{
        Name = "det_10g.onnx"
        Url  = "https://github.com/deepinsight/insightface/raw/master/model_zoo/buffalo_l/det_10g.onnx"
        Size = "~17 MB"
        Desc = "SCRFD face detector"
    },
    @{
        Name = "w600k_r50.onnx"
        Url  = "https://github.com/deepinsight/insightface/raw/master/model_zoo/buffalo_l/w600k_r50.onnx"
        Size = "~174 MB"
        Desc = "ArcFace face recognizer"
    }
)

if (-not (Test-Path -LiteralPath $OutputDir)) {
    New-Item -ItemType Directory -Path $OutputDir | Out-Null
}

Write-Host "Downloading InsightFace models to $OutputDir"
Write-Host "============================================="

foreach ($model in $models) {
    $outputPath = Join-Path $OutputDir $model.Name
    
    if (Test-Path -LiteralPath $outputPath) {
        Write-Host "[SKIP] $($model.Name) already exists"
        continue
    }
    
    Write-Host "Downloading $($model.Name) ($($model.Size)) - $($model.Desc)..."
    Write-Host "  URL: $($model.Url)"
    
    try {
        Invoke-WebRequest -Uri $model.Url -OutFile $outputPath -UseBasicParsing
        Write-Host "  [OK] Downloaded: $outputPath"
    } catch {
        Write-Error "Failed to download $($model.Name): $_"
        Write-Host "  Alternative: Download manually from $($model.Url)"
    }
}

Write-Host ""
Write-Host "============================================="
Write-Host "Models download complete!"
Write-Host "Verify with: dir $OutputDir"
