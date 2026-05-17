$ErrorActionPreference = "Stop"

param(
  [Parameter(Mandatory = $true, Position = 0)]
  [string]$DashIP,

  [Parameter(Mandatory = $true, Position = 1)]
  [string]$DashPortOrSecret,

  [Parameter(Position = 2)]
  [string]$Secret,

  [Parameter(Position = 3)]
  [string]$IntervalSeconds = "",

  [Parameter(ValueFromRemainingArguments = $true)]
  [string[]]$ExtraArgs
)

$App = "ithiltir-node"
$ServiceName = "ithiltir-node"

$InstallDir = Join-Path $env:ProgramFiles "Ithiltir-node"
$DataDir = Join-Path $env:ProgramData "Ithiltir-node"
$NodeBinDir = Join-Path $DataDir "bin"
$BinPath = Join-Path $NodeBinDir "$App.exe"
$RunnerPath = Join-Path $InstallDir "ithiltir-runner.exe"

$DOWNLOAD_SCHEME = if ($env:DOWNLOAD_SCHEME) { $env:DOWNLOAD_SCHEME } else { "__DOWNLOAD_SCHEME__" }
$DOWNLOAD_HOST = if ($env:DOWNLOAD_HOST) { $env:DOWNLOAD_HOST } else { "__DOWNLOAD_HOST__" }
$DOWNLOAD_PATH = if ($env:DOWNLOAD_PATH) { $env:DOWNLOAD_PATH } else { "__DOWNLOAD_PATH__" }
$DOWNLOAD_PREFIX = if ($env:DOWNLOAD_PREFIX) { $env:DOWNLOAD_PREFIX } else { "node_windows_" }

function Get-DefaultPort([string]$Scheme) {
  if ([string]::IsNullOrWhiteSpace($Scheme)) { return "80" }
  switch ($Scheme.ToLowerInvariant()) {
    "https" { return "443" }
    "http" { return "80" }
    default { return "80" }
  }
}

function Require-Admin {
  $currentIdentity = [Security.Principal.WindowsIdentity]::GetCurrent()
  $principal = New-Object Security.Principal.WindowsPrincipal($currentIdentity)
  if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw "Administrator privileges are required. Please run PowerShell as Administrator."
  }
}

function Detect-Arch {
  $arch = $env:PROCESSOR_ARCHITECTURE
  switch ($arch) {
    "AMD64" { return "amd64" }
    "ARM64" { return "arm64" }
    default {
      if ([Environment]::Is64BitOperatingSystem) { return "amd64" }
      throw "Only amd64/arm64 are supported; current PROCESSOR_ARCHITECTURE=$arch"
    }
  }
}

function Download-File([string]$Url, [string]$OutFile) {
  $tmpDir = Split-Path -Parent $OutFile
  if ($tmpDir -and !(Test-Path $tmpDir)) {
    New-Item -ItemType Directory -Force -Path $tmpDir | Out-Null
  }

  try {
    Invoke-WebRequest -Uri $Url -OutFile $OutFile -UseBasicParsing -TimeoutSec 60
  } catch {
    throw "Download failed: $Url ($($_.Exception.Message))"
  }
}

function Url-Host([string]$HostName) {
  if ($HostName.StartsWith("[") -and $HostName.EndsWith("]")) { return $HostName }
  if ($HostName.Contains(":")) { return "[$HostName]" }
  return $HostName
}

function Report-Url([string]$DashIP, [string]$DashPort) {
  return "{0}://{1}:{2}/api/node/metrics" -f $DOWNLOAD_SCHEME, (Url-Host $DashIP), $DashPort
}

function Configure-Report([string]$Url, [string]$Secret, [string[]]$ExtraArgs = @()) {
  & $BinPath report install $Url $Secret @ExtraArgs
  if ($LASTEXITCODE -ne 0) {
    throw "Report configuration failed."
  }
}

function Enable-TimeSync {
  Write-Host "[+] enabling Windows time sync (non-fatal)"

  try {
    Set-Service -Name W32Time -StartupType Automatic -ErrorAction Stop
    Start-Service -Name W32Time -ErrorAction SilentlyContinue
    Write-Host "[+] Windows time service is enabled"

    w32tm.exe /resync /nowait | Out-Null
    if ($LASTEXITCODE -ne 0) {
      Write-Warning "Windows time service is enabled, but immediate resync was not accepted. It should sync on the normal Windows schedule."
    }
  } catch {
    Write-Warning "Could not enable Windows time sync automatically; please check Windows Time service manually. $($_.Exception.Message)"
  }
}

function Stop-And-RemoveService([string]$Name) {
  $svc = Get-Service -Name $Name -ErrorAction SilentlyContinue
  if ($null -eq $svc) { return }

  try {
    if ($svc.Status -ne "Stopped") { Stop-Service -Name $Name -Force -ErrorAction SilentlyContinue }
  } catch { }

  sc.exe delete $Name | Out-Null
}

function Create-Or-UpdateService([string]$Name, [string]$BinaryPathName) {
  Stop-And-RemoveService $Name
  New-Service -Name $Name -BinaryPathName $BinaryPathName -DisplayName "Ithiltir Node" -StartupType Automatic
  Start-Service -Name $Name
}

Require-Admin
Enable-TimeSync

$arch = Detect-Arch
$url = "{0}://{1}{2}/{3}{4}.exe" -f $DOWNLOAD_SCHEME, $DOWNLOAD_HOST, $DOWNLOAD_PATH, $DOWNLOAD_PREFIX, $arch
$runnerUrl = "{0}://{1}{2}/runner_windows_{3}.exe" -f $DOWNLOAD_SCHEME, $DOWNLOAD_HOST, $DOWNLOAD_PATH, $arch

$resolvedSecret = $Secret
$resolvedPort = $DashPortOrSecret
if ([string]::IsNullOrWhiteSpace($Secret)) {
  $resolvedSecret = $DashPortOrSecret
  $resolvedPort = Get-DefaultPort $DOWNLOAD_SCHEME
}
if ([string]::IsNullOrWhiteSpace($resolvedSecret)) {
  throw "Secret is required."
}

$intervalValue = 0
$extra = @()
if ($IntervalSeconds) {
  if ($IntervalSeconds -match '^[0-9]+$') {
    $intervalValue = [int]$IntervalSeconds
  } else {
    $extra += $IntervalSeconds
  }
}
if ($ExtraArgs) { $extra += $ExtraArgs }
$reportArgs = @($extra | Where-Object { $_ -eq "--require-https" })

$intervalLabel = if ($intervalValue -gt 0) { $intervalValue.ToString() } else { "default" }

Write-Host "[+] arch=$arch"
Write-Host "[+] url=$url"
Write-Host "[+] runner=$runnerUrl"
Write-Host "[+] install=$InstallDir"
Write-Host "[+] mode=push dash_ip=$DashIP dash_port=$resolvedPort interval=$intervalLabel"

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
New-Item -ItemType Directory -Force -Path $DataDir | Out-Null
New-Item -ItemType Directory -Force -Path $NodeBinDir | Out-Null

$tmpFile = Join-Path $env:TEMP ("{0}-{1}.tmp" -f $App, [Guid]::NewGuid().ToString("n"))
$runnerTmpFile = Join-Path $env:TEMP ("ithiltir-runner-{0}.tmp" -f [Guid]::NewGuid().ToString("n"))
try {
  Download-File -Url $url -OutFile $tmpFile
  Copy-Item -Force -Path $tmpFile -Destination $BinPath
  Download-File -Url $runnerUrl -OutFile $runnerTmpFile
  Copy-Item -Force -Path $runnerTmpFile -Destination $RunnerPath
} finally {
  Remove-Item -Force -ErrorAction SilentlyContinue $tmpFile
  Remove-Item -Force -ErrorAction SilentlyContinue $runnerTmpFile
}
Configure-Report -Url (Report-Url $DashIP $resolvedPort) -Secret $resolvedSecret -ExtraArgs $reportArgs

$args = @("push")
if ($intervalValue -gt 0) { $args += @($intervalValue.ToString()) }
if ($extra.Count -gt 0) { $args += $extra }

$quotedArgs = $args | ForEach-Object {
  $s = $_
  if ($s -match '[\s"]') { '"' + ($s -replace '"', '\"') + '"' } else { $s }
}
$binaryQuoted = '"' + ($RunnerPath -replace '"', '\"') + '"'
$binaryPathName = $binaryQuoted + " " + ($quotedArgs -join " ")

Create-Or-UpdateService -Name $ServiceName -BinaryPathName $binaryPathName

Write-Host "[OK] Done: Windows service $ServiceName is running and set to start automatically"
Write-Host "     Status: Get-Service $ServiceName"
Write-Host "     Logs:   Event Viewer -> Windows Logs -> Application/System"
