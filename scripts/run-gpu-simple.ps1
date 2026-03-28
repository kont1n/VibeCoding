[CmdletBinding()]
param(
    [Alias("Input")]
    [string]$InputDir = "./dataset",
    [Alias("Output")]
    [string]$OutputDir = "./output",
    [int]$Workers = 4,
    [int]$GpuDetSessions = 2,
    [int]$GpuRecSessions = 2,
    [int]$EmbedBatchSize = 64,
    [int]$EmbedFlushMs = 10,
    [double]$Threshold = 0.5,
    [double]$DetThresh = 0.5,
    [double]$AvatarUpdateThreshold = 0.10,
    [int]$IntraThreads = 0,
    [int]$InterThreads = 0,
    [int]$MaxDim = 1920,
    [switch]$Serve,
    [int]$Port = 8080,
    [switch]$View,
    [string]$ModelsDir = "",
    [string]$OrtVersion = "",
    [switch]$SkipNvidiaRuntimeInstall,
    [string[]]$ExtraArgs = @()
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Get-OrtReleaseAsset {
    param([string]$RequestedVersion)

    # По умолчанию используем версию 1.24.4 или новее для совместимости с API v24
    if ([string]::IsNullOrWhiteSpace($RequestedVersion)) {
        $RequestedVersion = "1.24.4"
    }
    
    $version = $RequestedVersion.Trim().TrimStart("v")
    return [pscustomobject]@{
        Version   = $version
        AssetName = "onnxruntime-win-x64-gpu-$version.zip"
        Url       = "https://github.com/microsoft/onnxruntime/releases/download/v$version/onnxruntime-win-x64-gpu-$version.zip"
    }
}

function Get-CachedOrtLib {
    param([string]$RuntimeRoot)

    if (-not (Test-Path -LiteralPath $RuntimeRoot)) {
        return $null
    }

    $cached = Get-ChildItem -LiteralPath $RuntimeRoot -Recurse -Filter onnxruntime.dll -ErrorAction SilentlyContinue |
        Where-Object { $_.FullName -match "onnxruntime-win-x64-gpu-" } |
        Sort-Object LastWriteTime -Descending |
        Select-Object -First 1

    if ($null -eq $cached) {
        return $null
    }

    return $cached.FullName
}

function Ensure-OrtGpuRuntime {
    param(
        [string]$RuntimeRoot,
        [string]$RequestedVersion
    )

    if (-not (Test-Path -LiteralPath $RuntimeRoot)) {
        New-Item -ItemType Directory -Path $RuntimeRoot | Out-Null
    }

    $asset = $null
    try {
        $asset = Get-OrtReleaseAsset -RequestedVersion $RequestedVersion
    } catch {
        $cached = Get-CachedOrtLib -RuntimeRoot $RuntimeRoot
        if ($null -eq $cached) {
            throw
        }
        Write-Warning "Failed to fetch latest ONNX Runtime, using cached runtime: $cached"
        return (Resolve-Path -LiteralPath $cached).Path
    }

    $extractRoot = Join-Path $RuntimeRoot "onnxruntime-win-x64-gpu-$($asset.Version)"
    $innerRoot = Join-Path $extractRoot "onnxruntime-win-x64-gpu-$($asset.Version)"
    $ortLib = Join-Path $innerRoot "lib\onnxruntime.dll"
    if (Test-Path -LiteralPath $ortLib) {
        return (Resolve-Path -LiteralPath $ortLib).Path
    }

    $zipPath = Join-Path $RuntimeRoot $asset.AssetName
    Write-Host "Downloading ONNX Runtime GPU $($asset.Version)..."
    Invoke-WebRequest -Uri $asset.Url -OutFile $zipPath
    Expand-Archive -LiteralPath $zipPath -DestinationPath $extractRoot -Force
    Remove-Item -LiteralPath $zipPath -Force -ErrorAction SilentlyContinue

    if (-not (Test-Path -LiteralPath $ortLib)) {
        throw "ONNX Runtime archive was extracted, but onnxruntime.dll was not found: $ortLib"
    }

    return (Resolve-Path -LiteralPath $ortLib).Path
}

function Get-NvidiaBinDirs {
    $py = Get-Command py -ErrorAction SilentlyContinue
    if ($null -eq $py) {
        return @()
    }

    $sitePathsRaw = & py -c "import site; paths = list(dict.fromkeys(site.getsitepackages() + [site.getusersitepackages()])); print('\n'.join(paths))"
    $sitePaths = @($sitePathsRaw) | ForEach-Object { $_.Trim() } | Where-Object { $_ -ne "" } | Select-Object -Unique

    $bins = @()
    foreach ($sitePath in $sitePaths) {
        $nvidiaRoot = Join-Path $sitePath "nvidia"
        if (-not (Test-Path -LiteralPath $nvidiaRoot)) {
            continue
        }

        $subdirs = Get-ChildItem -LiteralPath $nvidiaRoot -Directory -ErrorAction SilentlyContinue
        foreach ($subdir in $subdirs) {
            $binDir = Join-Path $subdir.FullName "bin"
            if (Test-Path -LiteralPath $binDir) {
                $bins += $binDir
            }
        }
    }

    return $bins | Select-Object -Unique
}

function Test-DllAvailable {
    param(
        [string]$DllName,
        [string[]]$ExtraDirs
    )

    $pathDirs = @($env:Path -split ";")
    $allDirs = @($ExtraDirs + $pathDirs) | Select-Object -Unique

    foreach ($dir in $allDirs) {
        if (-not (Test-Path -LiteralPath $dir)) {
            continue
        }
        $candidate = Join-Path $dir $DllName
        if (Test-Path -LiteralPath $candidate) {
            return $true
        }
    }

    return $false
}

function Get-MissingNvidiaDlls {
    param([string[]]$SearchDirs)

    $requiredDlls = @(
        "cublasLt64_12.dll",
        "cudart64_12.dll",
        "cudnn64_9.dll",
        "cufft64_11.dll"
    )

    $missing = @()
    foreach ($dll in $requiredDlls) {
        if (-not (Test-DllAvailable -DllName $dll -ExtraDirs $SearchDirs)) {
            $missing += $dll
        }
    }

    return $missing
}

function Ensure-NvidiaRuntime {
    param([switch]$SkipInstall)

    $nvidiaBins = Get-NvidiaBinDirs
    $missing = Get-MissingNvidiaDlls -SearchDirs $nvidiaBins
    if (@($missing).Count -eq 0) {
        return $nvidiaBins
    }

    if ($SkipInstall) {
        throw "Missing CUDA runtime DLLs: $($missing -join ", "). Remove -SkipNvidiaRuntimeInstall or add required DLLs to PATH."
    }

    $py = Get-Command py -ErrorAction SilentlyContinue
    if ($null -eq $py) {
        throw "Python launcher 'py' is required to auto-install NVIDIA runtime DLLs via pip."
    }

    $packages = @(
        "nvidia-cublas-cu12",
        "nvidia-cuda-runtime-cu12",
        "nvidia-cudnn-cu12",
        "nvidia-cufft-cu12",
        "nvidia-curand-cu12",
        "nvidia-cusolver-cu12",
        "nvidia-cusparse-cu12",
        "nvidia-nvjitlink-cu12"
    )

    Write-Host "Installing/upgrading NVIDIA runtime packages..."
    & py -m pip install --upgrade @packages
    if ($LASTEXITCODE -ne 0) {
        throw "pip install for NVIDIA runtime packages failed."
    }

    $nvidiaBins = Get-NvidiaBinDirs
    $missingAfterInstall = Get-MissingNvidiaDlls -SearchDirs $nvidiaBins
    if (@($missingAfterInstall).Count -gt 0) {
        throw "CUDA runtime DLLs are still missing after install: $($missingAfterInstall -join ", ")."
    }

    return $nvidiaBins
}

function Add-PathEntries {
    param([string[]]$Entries)

    $current = @($env:Path -split ";")
    $ordered = New-Object System.Collections.Generic.List[string]

    foreach ($entry in $Entries) {
        if ([string]::IsNullOrWhiteSpace($entry)) {
            continue
        }
        if (-not (Test-Path -LiteralPath $entry)) {
            continue
        }
        if (-not $ordered.Contains($entry)) {
            $ordered.Add($entry)
        }
    }

    foreach ($entry in $current) {
        if ([string]::IsNullOrWhiteSpace($entry)) {
            continue
        }
        if (-not $ordered.Contains($entry)) {
            $ordered.Add($entry)
        }
    }

    $env:Path = ($ordered -join ";")
}

function Resolve-ModelsDirectory {
    param(
        [string]$Preferred,
        [string]$RepoRootPath
    )

    $candidates = @()
    if (-not [string]::IsNullOrWhiteSpace($Preferred)) {
        $candidates += $Preferred
    }

    $candidates += (Join-Path $RepoRootPath "models")

    $userProfile = [Environment]::GetFolderPath("UserProfile")
    if (-not [string]::IsNullOrWhiteSpace($userProfile)) {
        $candidates += (Join-Path $userProfile ".insightface\models\buffalo_l")
    }

    foreach ($candidate in ($candidates | Select-Object -Unique)) {
        $det = Join-Path $candidate "det_10g.onnx"
        $rec = Join-Path $candidate "w600k_r50.onnx"
        if ((Test-Path -LiteralPath $det) -and (Test-Path -LiteralPath $rec)) {
            return (Resolve-Path -LiteralPath $candidate).Path
        }
    }

    throw "InsightFace models not found. Pass -ModelsDir or place det_10g.onnx and w600k_r50.onnx in ./models."
}

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
Push-Location $repoRoot

try {
    $exePath = Join-Path $repoRoot "face-grouper.exe"
    if (-not (Test-Path -LiteralPath $exePath)) {
        throw "Executable not found at $exePath. Run 'go build -o face-grouper.exe .' first."
    }

    Write-Host "Resolving ONNX Runtime GPU..."
    $runtimeRoot = Join-Path $repoRoot "runtime"
    $ortLibPath = Ensure-OrtGpuRuntime -RuntimeRoot $runtimeRoot -RequestedVersion $OrtVersion
    $ortDir = Split-Path -Parent $ortLibPath

    $resolvedModelsDir = ""
    if (-not $View) {
        Write-Host "Resolving InsightFace models..."
        $resolvedModelsDir = Resolve-ModelsDirectory -Preferred $ModelsDir -RepoRootPath $repoRoot
    }

    Write-Host "Checking NVIDIA CUDA runtime DLLs..."
    $nvidiaBins = Ensure-NvidiaRuntime -SkipInstall:$SkipNvidiaRuntimeInstall
    Add-PathEntries -Entries (@($ortDir) + $nvidiaBins)

    $inv = [System.Globalization.CultureInfo]::InvariantCulture
    $launchArgs = @(
        "--input", $InputDir,
        "--output", $OutputDir,
        "--workers", $Workers.ToString($inv),
        "--gpu-det-sessions", $GpuDetSessions.ToString($inv),
        "--gpu-rec-sessions", $GpuRecSessions.ToString($inv),
        "--embed-batch-size", $EmbedBatchSize.ToString($inv),
        "--embed-flush-ms", $EmbedFlushMs.ToString($inv),
        "--threshold", $Threshold.ToString($inv),
        "--det-thresh", $DetThresh.ToString($inv),
        "--avatar-update-threshold", $AvatarUpdateThreshold.ToString($inv),
        "--intra-threads", $IntraThreads.ToString($inv),
        "--inter-threads", $InterThreads.ToString($inv),
        "--max-dim", $MaxDim.ToString($inv),
        "--port", $Port.ToString($inv),
        "--gpu",
        "--ort-lib", $ortLibPath
    )

    if (-not $View) {
        $launchArgs += @("--models-dir", $resolvedModelsDir)
    }
    if ($Serve) {
        $launchArgs += "--serve"
    }
    if ($View) {
        $launchArgs += "--view"
    }
    if (@($ExtraArgs).Count -gt 0) {
        $launchArgs += $ExtraArgs
    }

    Write-Host "ONNX Runtime: $ortLibPath"
    if (-not $View) {
        Write-Host "Models: $resolvedModelsDir"
    }
    Write-Host "Starting face-grouper on GPU..."

    & $exePath @launchArgs
    exit $LASTEXITCODE
} finally {
    Pop-Location
}
