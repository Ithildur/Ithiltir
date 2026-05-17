param(
	[string]$OutDir = "build/frontend/dist"
)

$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$webRoot = Join-Path $repoRoot "web"

function Resolve-RepoPath {
	param(
		[Parameter(Mandatory = $true)]
		[string]$Path
	)

	if ([System.IO.Path]::IsPathRooted($Path)) {
		return [System.IO.Path]::GetFullPath($Path)
	}

	return [System.IO.Path]::GetFullPath((Join-Path $repoRoot $Path))
}

if (-not (Test-Path (Join-Path $webRoot "package.json"))) {
	throw ("frontend package.json not found: {0}" -f (Join-Path $webRoot "package.json"))
}

if (-not (Test-Path (Join-Path $webRoot "bun.lock"))) {
	throw ("frontend lockfile not found: {0}" -f (Join-Path $webRoot "bun.lock"))
}

$bun = Get-Command bun -ErrorAction SilentlyContinue
if (-not $bun) {
	throw "bun is required to build the frontend"
}

$outPath = Resolve-RepoPath $OutDir
$outParent = Split-Path -Parent $outPath
if (-not (Test-Path $outParent)) {
	New-Item -ItemType Directory -Path $outParent -Force | Out-Null
}

Push-Location $webRoot
try {
	& $bun.Source install --frozen-lockfile
	if ($LASTEXITCODE -ne 0) {
		throw ("bun install failed (exit {0})" -f $LASTEXITCODE)
	}

	& $bun.Source run build -- --outDir $outPath --emptyOutDir
	if ($LASTEXITCODE -ne 0) {
		throw ("vite build failed (exit {0})" -f $LASTEXITCODE)
	}
} finally {
	Pop-Location
}

Write-Host ("frontend built: {0}" -f $outPath)
