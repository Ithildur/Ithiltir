param(
	[string]$OutDir = "release",
	[string]$Version = "",
	[string]$NodeVersion = "",
	[switch]$NodeLocal,
	[string]$NodeLocalDir = "",
	[switch]$UseGitTag,
	[switch]$Release,
	# [string[]]$Targets = @("linux/amd64", "windows/amd64"),
	[string[]]$Targets = @("linux/amd64"),
	[Alias("z")]
	[switch]$Zip
)

$ErrorActionPreference = "Stop"
$FrontendDistDir = "build/frontend/dist"
$NodeLocalDefaultDir = "deploy/node"
$NodeRemoteUrl = "https://github.com/Ithildur/Ithiltir-node.git"
$NodeRepoSlug = "Ithildur/Ithiltir-node"
$BuildChannel = "release"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path

function Get-GitTag {
	if ($env:GITHUB_REF_TYPE -eq "tag" -and $env:GITHUB_REF_NAME) {
		return $env:GITHUB_REF_NAME.Trim()
	}

	$tags = @(& git -C $repoRoot tag --points-at HEAD 2>$null | Where-Object { $_ })
	if ($tags.Count -ne 1) {
		if ($tags.Count -gt 0) {
			[Console]::Error.WriteLine(($tags -join [Environment]::NewLine))
		}
		throw "current commit must have exactly one git tag"
	}

	return $tags[0].Trim()
}

function Invoke-VersionTool {
	param(
		[Parameter(Mandatory = $true)]
		[string[]]$Arguments,
		[string[]]$InputLines = @()
	)

	$go = Resolve-Go
	if ($InputLines.Count -gt 0 -or ($Arguments.Count -gt 0 -and $Arguments[0] -eq "latest")) {
		$inputText = ""
		if ($InputLines.Count -gt 0) {
			$inputText = ($InputLines -join [Environment]::NewLine) + [Environment]::NewLine
		}
		$output = @($inputText | & $go -C $repoRoot run ./cmd/releasever @Arguments)
	} else {
		$output = @(& $go -C $repoRoot run ./cmd/releasever @Arguments)
	}
	if ($LASTEXITCODE -ne 0) {
		throw ("version tool failed: go run ./cmd/releasever {0}" -f ($Arguments -join " "))
	}
	return $output
}

function Get-VersionChannel {
	param(
		[Parameter(Mandatory = $true)]
		[string]$Value
	)

	$output = Invoke-VersionTool -Arguments @("channel", $Value)
	if ($output.Count -eq 0) { return "" }
	return $output[0].Trim()
}

function Test-SemVer {
	param(
		[Parameter(Mandatory = $true)]
		[string]$Value
	)

	try {
		Invoke-VersionTool -Arguments @("validate", $Value) | Out-Null
		return $true
	} catch {
		return $false
	}
}

function Resolve-BuildVersion {
	param(
		[string]$RawVersion
	)

	if ($UseGitTag) {
		$RawVersion = Get-GitTag
	}

	if ($Release -and -not $UseGitTag) {
		throw "release mode requires -UseGitTag"
	}

	if ([string]::IsNullOrWhiteSpace($RawVersion)) {
		$RawVersion = "0.0.0-dev"
	}

	$RawVersion = $RawVersion.Trim()
	try {
		$script:BuildChannel = Get-VersionChannel -Value $RawVersion
	} catch {
		throw "version must be strict SemVer without a v prefix: $RawVersion"
	}

	return $RawVersion
}

function Get-LatestRemoteTag {
	param(
		[Parameter(Mandatory = $true)]
		[string]$RemoteUrl,
		[Parameter(Mandatory = $true)]
		[string]$Channel
	)

	$refs = @(& git ls-remote --tags --refs --sort="v:refname" $RemoteUrl 2>$null)
	if ($LASTEXITCODE -ne 0) {
		$refs = @(& git ls-remote --tags --refs $RemoteUrl)
		if ($LASTEXITCODE -ne 0) {
			throw "failed to fetch tags from $RemoteUrl"
		}
	}

	$tags = @()
	foreach ($line in $refs) {
		$parts = $line -split "\s+"
		if ($parts.Count -lt 2) { continue }
		$tags += ($parts[1] -replace "^refs/tags/", "")
	}

	try {
		$latest = Invoke-VersionTool -Arguments @("latest", $Channel) -InputLines $tags
	} catch {
		throw "no valid Ithiltir-node $Channel tags found on $RemoteUrl"
	}
	if ($latest.Count -eq 0 -or [string]::IsNullOrWhiteSpace($latest[0])) {
		throw "no valid Ithiltir-node $Channel tags found on $RemoteUrl"
	}
	return $latest[0].Trim()
}

function Resolve-NodeVersion {
	param(
		[string]$RawVersion,
		[string]$LocalDir
	)

	if ([string]::IsNullOrWhiteSpace($RawVersion)) {
		if (-not [string]::IsNullOrWhiteSpace($env:ITHILTIR_NODE_VERSION)) {
			$RawVersion = $env:ITHILTIR_NODE_VERSION
		}
	}

	if ([string]::IsNullOrWhiteSpace($RawVersion) -and -not [string]::IsNullOrWhiteSpace($LocalDir)) {
		throw "local node packaging requires -NodeVersion or ITHILTIR_NODE_VERSION"
	}

	if ([string]::IsNullOrWhiteSpace($RawVersion)) {
		if (-not (Get-Command git -ErrorAction SilentlyContinue)) {
			throw "git is required to resolve latest Ithiltir-node release tag"
		}
		$RawVersion = Get-LatestRemoteTag -RemoteUrl $NodeRemoteUrl -Channel $BuildChannel
	}

	$RawVersion = $RawVersion.Trim()
	if (-not (Test-SemVer -Value $RawVersion)) {
		throw "node version must be strict SemVer without a v prefix: $RawVersion"
	}

	if ($Release) {
		$nodeChannel = Get-VersionChannel -Value $RawVersion
		if ($BuildChannel -eq "release" -and $nodeChannel -ne "release") {
			throw "node version channel ($nodeChannel) must match dash version channel ($BuildChannel)"
		}
	}

	return $RawVersion
}

function Download-File {
	param(
		[Parameter(Mandatory = $true)]
		[string]$Url,
		[Parameter(Mandatory = $true)]
		[string]$OutFile
	)

	$outDir = Split-Path -Parent $OutFile
	if ($outDir -and -not (Test-Path $outDir)) {
		New-Item -ItemType Directory -Path $outDir -Force | Out-Null
	}

	Invoke-WebRequest -Uri $Url -OutFile $OutFile -UseBasicParsing -TimeoutSec 600
}

function Download-NodeAsset {
	param(
		[Parameter(Mandatory = $true)]
		[string]$Version,
		[Parameter(Mandatory = $true)]
		[string]$Asset,
		[Parameter(Mandatory = $true)]
		[string]$OutFile
	)

	$url = "https://github.com/{0}/releases/download/{1}/{2}" -f $NodeRepoSlug, $Version, $Asset
	Write-Host ("downloading node asset: {0}" -f $url)
	Download-File -Url $url -OutFile $OutFile
}

function Set-UnixExecutable {
	param(
		[Parameter(Mandatory = $true)]
		[string[]]$Paths
	)

	$chmod = Get-Command chmod -ErrorAction SilentlyContinue
	if (-not $chmod) { return }
	foreach ($path in $Paths) {
		& $chmod.Source 755 $path
		if ($LASTEXITCODE -ne 0) {
			throw ("chmod failed: {0}" -f $path)
		}
	}
}

function Set-NodeDeployPermissions {
	param(
		[Parameter(Mandatory = $true)]
		[string]$DeployDir
	)

	Set-UnixExecutable -Paths @(
		(Join-Path $DeployDir "linux/node_linux_amd64"),
		(Join-Path $DeployDir "linux/node_linux_arm64"),
		(Join-Path $DeployDir "macos/node_macos_arm64")
	)
}

function Prepare-RemoteNodeDeploy {
	param(
		[Parameter(Mandatory = $true)]
		[string]$Version,
		[Parameter(Mandatory = $true)]
		[string]$DeployDir
	)

	if (Test-Path $DeployDir) {
		Remove-Item -Recurse -Force $DeployDir
	}
	New-Item -ItemType Directory -Path (Join-Path $DeployDir "linux") -Force | Out-Null
	New-Item -ItemType Directory -Path (Join-Path $DeployDir "macos") -Force | Out-Null
	New-Item -ItemType Directory -Path (Join-Path $DeployDir "windows") -Force | Out-Null

	$linuxAmd64 = Join-Path $DeployDir "linux/node_linux_amd64"
	$linuxArm64 = Join-Path $DeployDir "linux/node_linux_arm64"
	$macArm64 = Join-Path $DeployDir "macos/node_macos_arm64"

	Download-NodeAsset -Version $Version -Asset "Ithiltir-node-linux-amd64" -OutFile $linuxAmd64
	Download-NodeAsset -Version $Version -Asset "Ithiltir-node-linux-arm64" -OutFile $linuxArm64
	Download-NodeAsset -Version $Version -Asset "Ithiltir-node-macos-arm64" -OutFile $macArm64
	Download-NodeAsset -Version $Version -Asset "Ithiltir-node-windows-amd64.exe" -OutFile (Join-Path $DeployDir "windows/node_windows_amd64.exe")
	Download-NodeAsset -Version $Version -Asset "Ithiltir-node-windows-arm64.exe" -OutFile (Join-Path $DeployDir "windows/node_windows_arm64.exe")
	Download-NodeAsset -Version $Version -Asset "Ithiltir-runner-windows-amd64.exe" -OutFile (Join-Path $DeployDir "windows/runner_windows_amd64.exe")
	Download-NodeAsset -Version $Version -Asset "Ithiltir-runner-windows-arm64.exe" -OutFile (Join-Path $DeployDir "windows/runner_windows_arm64.exe")

	Set-NodeDeployPermissions -DeployDir $DeployDir
}

function Copy-LocalNodeAsset {
	param(
		[Parameter(Mandatory = $true)]
		[string[]]$Sources,
		[Parameter(Mandatory = $true)]
		[string]$OutFile
	)

	foreach ($source in $Sources) {
		if (Test-Path $source -PathType Leaf) {
			$outDir = Split-Path -Parent $OutFile
			if ($outDir -and -not (Test-Path $outDir)) {
				New-Item -ItemType Directory -Path $outDir -Force | Out-Null
			}
			Copy-Item -Force $source $OutFile
			return
		}
	}

	throw ("local node asset not found. tried: {0}" -f ($Sources -join ", "))
}

function Prepare-LocalNodeDeploy {
	param(
		[Parameter(Mandatory = $true)]
		[string]$SourceDir,
		[Parameter(Mandatory = $true)]
		[string]$DeployDir
	)

	if (-not (Test-Path $SourceDir -PathType Container)) {
		throw ("local node deploy directory not found: {0}" -f $SourceDir)
	}

	if (Test-Path $DeployDir) {
		Remove-Item -Recurse -Force $DeployDir
	}
	New-Item -ItemType Directory -Path (Join-Path $DeployDir "linux") -Force | Out-Null
	New-Item -ItemType Directory -Path (Join-Path $DeployDir "macos") -Force | Out-Null
	New-Item -ItemType Directory -Path (Join-Path $DeployDir "windows") -Force | Out-Null

	Copy-LocalNodeAsset -Sources @((Join-Path $SourceDir "linux/node_linux_amd64"), (Join-Path $SourceDir "linux/Ithiltir-node-linux-amd64"), (Join-Path $SourceDir "Ithiltir-node-linux-amd64")) -OutFile (Join-Path $DeployDir "linux/node_linux_amd64")
	Copy-LocalNodeAsset -Sources @((Join-Path $SourceDir "linux/node_linux_arm64"), (Join-Path $SourceDir "linux/Ithiltir-node-linux-arm64"), (Join-Path $SourceDir "Ithiltir-node-linux-arm64")) -OutFile (Join-Path $DeployDir "linux/node_linux_arm64")
	Copy-LocalNodeAsset -Sources @((Join-Path $SourceDir "macos/node_macos_arm64"), (Join-Path $SourceDir "macos/Ithiltir-node-macos-arm64"), (Join-Path $SourceDir "Ithiltir-node-macos-arm64")) -OutFile (Join-Path $DeployDir "macos/node_macos_arm64")
	Copy-LocalNodeAsset -Sources @((Join-Path $SourceDir "windows/node_windows_amd64.exe"), (Join-Path $SourceDir "windows/Ithiltir-node-windows-amd64.exe"), (Join-Path $SourceDir "Ithiltir-node-windows-amd64.exe")) -OutFile (Join-Path $DeployDir "windows/node_windows_amd64.exe")
	Copy-LocalNodeAsset -Sources @((Join-Path $SourceDir "windows/node_windows_arm64.exe"), (Join-Path $SourceDir "windows/Ithiltir-node-windows-arm64.exe"), (Join-Path $SourceDir "Ithiltir-node-windows-arm64.exe")) -OutFile (Join-Path $DeployDir "windows/node_windows_arm64.exe")
	Copy-LocalNodeAsset -Sources @((Join-Path $SourceDir "windows/runner_windows_amd64.exe"), (Join-Path $SourceDir "windows/Ithiltir-runner-windows-amd64.exe"), (Join-Path $SourceDir "Ithiltir-runner-windows-amd64.exe")) -OutFile (Join-Path $DeployDir "windows/runner_windows_amd64.exe")
	Copy-LocalNodeAsset -Sources @((Join-Path $SourceDir "windows/runner_windows_arm64.exe"), (Join-Path $SourceDir "windows/Ithiltir-runner-windows-arm64.exe"), (Join-Path $SourceDir "Ithiltir-runner-windows-arm64.exe")) -OutFile (Join-Path $DeployDir "windows/runner_windows_arm64.exe")

	Set-NodeDeployPermissions -DeployDir $DeployDir
}

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

$outRoot = Resolve-RepoPath $OutDir
New-Item -ItemType Directory -Path $outRoot -Force | Out-Null
if ($NodeLocal -and [string]::IsNullOrWhiteSpace($NodeLocalDir)) {
	$NodeLocalDir = $NodeLocalDefaultDir
}

function Resolve-Go {
	$go = Get-Command go -ErrorAction SilentlyContinue
	if (-not $go) {
		throw "go is required to build dash"
	}

	return $go.Source
}

function Get-DashBinaryPath {
	param(
		[string]$Os,
		[string]$Arch,
		[string]$DistRoot
	)

	$osDir = $Os
	if ($Os -eq "darwin") {
		$osDir = "macos"
	}

	$binName = "dash_{0}" -f $Arch
	if ($Os -eq "windows") {
		$binName = "{0}.exe" -f $binName
	}

	return Join-Path $DistRoot (Join-Path $osDir $binName)
}

function Build-DashBinary {
	param(
		[Parameter(Mandatory = $true)]
		[string]$Os,
		[Parameter(Mandatory = $true)]
		[string]$Arch,
		[Parameter(Mandatory = $true)]
		[string]$DistRoot,
		[Parameter(Mandatory = $true)]
		[string]$GoCmd,
		[Parameter(Mandatory = $true)]
		[string]$Ldflags
	)

	$output = Get-DashBinaryPath -Os $Os -Arch $Arch -DistRoot $DistRoot
	$outputDir = Split-Path -Parent $output
	New-Item -ItemType Directory -Path $outputDir -Force | Out-Null
	Write-Host ("building dash for {0}/{1}: {2}" -f $Os, $Arch, $output)

	$oldGoos = $env:GOOS
	$oldGoarch = $env:GOARCH
	$oldCgo = $env:CGO_ENABLED
	try {
		$env:GOOS = $Os
		$env:GOARCH = $Arch
		$env:CGO_ENABLED = "0"
		& $GoCmd -C $repoRoot build -trimpath -ldflags $Ldflags -o $output ./cmd/dash
		if ($LASTEXITCODE -ne 0) {
			throw ("go build failed for {0}/{1} (exit {2})" -f $Os, $Arch, $LASTEXITCODE)
		}
	} finally {
		$env:GOOS = $oldGoos
		$env:GOARCH = $oldGoarch
		$env:CGO_ENABLED = $oldCgo
	}
}

$buildVersion = Resolve-BuildVersion -RawVersion $Version
$nodeBuildVersion = Resolve-NodeVersion -RawVersion $NodeVersion -LocalDir $NodeLocalDir
$ldflags = "-s -w -X dash/internal/version.Current=$buildVersion -X dash/internal/version.BundledNode=$nodeBuildVersion"
Write-Host ("dash version: {0}" -f $buildVersion)
Write-Host ("dash channel: {0}" -f $BuildChannel)
Write-Host ("node version: {0}" -f $nodeBuildVersion)
if (-not [string]::IsNullOrWhiteSpace($NodeLocalDir)) {
	Write-Host ("node source: {0}" -f (Resolve-RepoPath $NodeLocalDir))
}

$goCmd = Resolve-Go
$frontendScript = Join-Path $repoRoot "scripts/build_frontend.ps1"
$frontendDistPath = Resolve-RepoPath $FrontendDistDir

$targetList = @()
foreach ($rawTarget in $Targets) {
	$targetList += $rawTarget.Split(",", [System.StringSplitOptions]::RemoveEmptyEntries) | ForEach-Object { $_.Trim() } | Where-Object { $_ }
}
if ($targetList.Count -eq 0) {
	throw "no valid targets provided"
}
foreach ($t in $targetList) {
	$parts = $t.Split("/", 2)
	if ($parts.Count -ne 2 -or [string]::IsNullOrWhiteSpace($parts[0]) -or [string]::IsNullOrWhiteSpace($parts[1])) {
		throw "invalid target: $t (expected os/arch)"
	}
}

Push-Location $repoRoot
try {
	if (-not (Test-Path $frontendScript)) {
		throw ("frontend build script not found: {0}" -f $frontendScript)
	}

	& $frontendScript -OutDir $FrontendDistDir
	if ($LASTEXITCODE -ne 0) {
		throw ("frontend build failed (exit {0})" -f $LASTEXITCODE)
	}

	if (-not (Test-Path $frontendDistPath)) {
		throw ("frontend build output not found: {0}" -f $frontendDistPath)
	}

	$distRoot = Join-Path $repoRoot "build"
	$nodeDeployPath = Join-Path $distRoot "node-deploy"
	if ([string]::IsNullOrWhiteSpace($NodeLocalDir)) {
		Prepare-RemoteNodeDeploy -Version $nodeBuildVersion -DeployDir $nodeDeployPath
	} else {
		Prepare-LocalNodeDeploy -SourceDir (Resolve-RepoPath $NodeLocalDir) -DeployDir $nodeDeployPath
	}

	foreach ($t in $targetList) {
		$parts = $t.Split("/", 2)
		if ($parts.Count -ne 2) {
			throw "invalid target: $t (expected os/arch)"
		}
		$os = $parts[0]
		$arch = $parts[1]
		Build-DashBinary -Os $os -Arch $arch -DistRoot $distRoot -GoCmd $goCmd -Ldflags $ldflags

		$buildRoot = Join-Path $outRoot "build"
		New-Item -ItemType Directory -Path $buildRoot -Force | Out-Null
		$pkgRoot = Join-Path $buildRoot "Ithiltir-dash"
		if (Test-Path $pkgRoot) {
			Remove-Item -Recurse -Force $pkgRoot
		}
		New-Item -ItemType Directory -Path $pkgRoot | Out-Null
		New-Item -ItemType Directory -Path (Join-Path $pkgRoot "bin") | Out-Null
		New-Item -ItemType Directory -Path (Join-Path $pkgRoot "logs") | Out-Null

		$exeName = "dash"
		if ($os -eq "windows") {
			$exeName = "dash.exe"
		}
		$exePath = Join-Path $pkgRoot ("bin/{0}" -f $exeName)

		$sourceExe = Get-DashBinaryPath -Os $os -Arch $arch -DistRoot $distRoot
		if (-not (Test-Path $sourceExe)) {
			throw ("dash build output not found: {0}" -f $sourceExe)
		}

		Copy-Item -Force $sourceExe $exePath

		Copy-Item -Recurse -Force (Join-Path $repoRoot "configs") (Join-Path $pkgRoot "configs")
		Copy-Item -Recurse -Force $frontendDistPath (Join-Path $pkgRoot "dist")
		Copy-Item -Recurse -Force $nodeDeployPath (Join-Path $pkgRoot "deploy")
		Copy-Item -Force (Join-Path $repoRoot "install_dash_linux.sh") $pkgRoot
		Copy-Item -Force (Join-Path $repoRoot "update_dash_linux.sh") $pkgRoot

		if ($Zip) {
			$zipPath = Join-Path $outRoot ("Ithiltir_dash_{0}_{1}.zip" -f $os, $arch)
			if (Test-Path $zipPath) {
				Remove-Item -Force $zipPath
			}
			Compress-Archive -Path $pkgRoot -DestinationPath $zipPath -Force
		}
	}
} finally {
	Pop-Location
}

Write-Host ("done. output: {0}" -f $outRoot)
