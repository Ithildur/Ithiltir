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

$ErrorActionPreference = "Stop"

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
$APP_LANGUAGE = if ($env:APP_LANGUAGE) { $env:APP_LANGUAGE } else { "__APP_LANGUAGE__" }

function Use-Zh {
  $lang = $APP_LANGUAGE.Trim().ToLowerInvariant()
  return !($lang -eq "en" -or $lang -eq "english")
}

function Msg([string]$Key, [object[]]$Args = @()) {
  if (Use-Zh) {
    switch ($Key) {
      "AdminRequired" { return "需要管理员权限。请以管理员身份运行 PowerShell。" }
      "UnsupportedArch" { return "仅支持 amd64/arm64，当前 PROCESSOR_ARCHITECTURE=$($Args[0])" }
      "DownloadFailed" { return "下载失败：$($Args[0]) ($($Args[1]))" }
      "ReportConfigFailed" { return "上报配置失败。" }
      "EnableTimeSync" { return "[+] 正在启用 Windows 时间同步（非致命）" }
      "TimeServiceEnabled" { return "[+] Windows 时间服务已启用" }
      "TimeResyncWarn" { return "Windows 时间服务已启用，但系统未接受立即同步请求。它会按 Windows 正常计划同步。" }
      "TimeSyncFailed" { return "无法自动启用 Windows 时间同步；请手动检查 Windows Time 服务。$($Args[0])" }
      "SecretRequired" { return "Secret 不能为空。" }
      "Done" { return "[OK] 完成：Windows 服务 $ServiceName 已运行并设置为自动启动" }
      "Status" { return "     状态：Get-Service $ServiceName" }
      "Logs" { return "     日志：事件查看器 -> Windows 日志 -> 应用程序/系统" }
      default { return $Key }
    }
  }

  switch ($Key) {
    "AdminRequired" { return "Administrator privileges are required. Please run PowerShell as Administrator." }
    "UnsupportedArch" { return "Only amd64/arm64 are supported; current PROCESSOR_ARCHITECTURE=$($Args[0])" }
    "DownloadFailed" { return "Download failed: $($Args[0]) ($($Args[1]))" }
    "ReportConfigFailed" { return "Report configuration failed." }
    "EnableTimeSync" { return "[+] enabling Windows time sync (non-fatal)" }
    "TimeServiceEnabled" { return "[+] Windows time service is enabled" }
    "TimeResyncWarn" { return "Windows time service is enabled, but immediate resync was not accepted. It should sync on the normal Windows schedule." }
    "TimeSyncFailed" { return "Could not enable Windows time sync automatically; please check Windows Time service manually. $($Args[0])" }
    "SecretRequired" { return "Secret is required." }
    "Done" { return "[OK] Done: Windows service $ServiceName is running and set to start automatically" }
    "Status" { return "     Status: Get-Service $ServiceName" }
    "Logs" { return "     Logs:   Event Viewer -> Windows Logs -> Application/System" }
    default { return $Key }
  }
}

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
    throw (Msg "AdminRequired")
  }
}

function Detect-Arch {
  $arch = $env:PROCESSOR_ARCHITECTURE
  switch ($arch) {
    "AMD64" { return "amd64" }
    "ARM64" { return "arm64" }
    default {
      if ([Environment]::Is64BitOperatingSystem) { return "amd64" }
      throw (Msg "UnsupportedArch" @($arch))
    }
  }
}

function Test-DownloadRedirect([Uri]$Original, [Uri]$Current, [Uri]$Next) {
  if (!$Next.IsAbsoluteUri) { return $false }
  if (![string]::IsNullOrEmpty($Next.UserInfo)) { return $false }
  if (![string]::Equals($Original.Host, $Next.Host, [StringComparison]::OrdinalIgnoreCase)) {
    return $false
  }

  $currentScheme = $Current.Scheme.ToLowerInvariant()
  $nextScheme = $Next.Scheme.ToLowerInvariant()
  if ($currentScheme -eq $nextScheme) {
    return ($nextScheme -eq "http" -or $nextScheme -eq "https") -and $Current.Port -eq $Next.Port
  }
  return $currentScheme -eq "http" -and $nextScheme -eq "https"
}

function Download-File([string]$Url, [string]$OutFile, [string]$Secret) {
  $tmpDir = Split-Path -Parent $OutFile
  if ($tmpDir -and !(Test-Path $tmpDir)) {
    New-Item -ItemType Directory -Force -Path $tmpDir | Out-Null
  }

  $handler = $null
  $client = $null
  try {
    Add-Type -AssemblyName System.Net.Http
    $handler = [System.Net.Http.HttpClientHandler]::new()
    $handler.AllowAutoRedirect = $false
    $client = [System.Net.Http.HttpClient]::new($handler)
    $client.Timeout = [TimeSpan]::FromSeconds(60)

    $original = [Uri]$Url
    $current = $original
    for ($redirects = 0; ; $redirects++) {
      $request = [System.Net.Http.HttpRequestMessage]::new([System.Net.Http.HttpMethod]::Get, $current)
      $response = $null
      try {
        [void]$request.Headers.TryAddWithoutValidation("X-Node-Secret", $Secret)
        $response = $client.SendAsync($request).GetAwaiter().GetResult()
        $status = [int]$response.StatusCode
        if ($status -ge 200 -and $status -lt 300) {
          $input = $response.Content.ReadAsStreamAsync().GetAwaiter().GetResult()
          $output = [IO.File]::Open($OutFile, [IO.FileMode]::Create, [IO.FileAccess]::Write, [IO.FileShare]::None)
          try {
            $input.CopyTo($output)
          } finally {
            $output.Dispose()
            $input.Dispose()
          }
          return
        }

        if ($status -notin @(301, 302, 303, 307, 308) -or $null -eq $response.Headers.Location) {
          throw "HTTP status $status"
        }
        if ($redirects -ge 5) {
          throw "redirect limit exceeded"
        }
        $next = if ($response.Headers.Location.IsAbsoluteUri) {
          $response.Headers.Location
        } else {
          [Uri]::new($current, $response.Headers.Location)
        }
        if (!(Test-DownloadRedirect $original $current $next)) {
          throw "unsafe redirect to $next"
        }
        $current = $next
      } finally {
        if ($null -ne $response) { $response.Dispose() }
        $request.Dispose()
      }
    }
  } catch {
    throw (Msg "DownloadFailed" @($Url, $_.Exception.Message))
  } finally {
    if ($null -ne $client) { $client.Dispose() }
    if ($null -ne $handler) { $handler.Dispose() }
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
    throw (Msg "ReportConfigFailed")
  }
}

function Enable-TimeSync {
  Write-Host (Msg "EnableTimeSync")

  try {
    Set-Service -Name W32Time -StartupType Automatic -ErrorAction Stop
    Start-Service -Name W32Time -ErrorAction SilentlyContinue
    Write-Host (Msg "TimeServiceEnabled")

    w32tm.exe /resync /nowait | Out-Null
    if ($LASTEXITCODE -ne 0) {
      Write-Warning (Msg "TimeResyncWarn")
    }
  } catch {
    Write-Warning (Msg "TimeSyncFailed" @($_.Exception.Message))
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
  throw (Msg "SecretRequired")
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
  Download-File -Url $url -OutFile $tmpFile -Secret $resolvedSecret
  Copy-Item -Force -Path $tmpFile -Destination $BinPath
  Download-File -Url $runnerUrl -OutFile $runnerTmpFile -Secret $resolvedSecret
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

Write-Host (Msg "Done")
Write-Host (Msg "Status")
Write-Host (Msg "Logs")
