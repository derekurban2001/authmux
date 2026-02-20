Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$Repo = "derekurban2001/authmux"
$ModulePath = "github.com/derekurban2001/authmux"
$BinaryBase = "authmux"
$BinaryName = "authmux.exe"

$InstallDir = if ($env:AUTHMUX_INSTALL_DIR) { $env:AUTHMUX_INSTALL_DIR } else { Join-Path $HOME ".local\bin" }
$Version = if ($env:AUTHMUX_VERSION) { $env:AUTHMUX_VERSION } else { "latest" }
$AutoPathRaw = if ($env:AUTHMUX_AUTO_PATH) { $env:AUTHMUX_AUTO_PATH } else { "1" }

function Write-Log {
  param([string]$Message)
  Write-Host "[authmux-install] $Message"
}

function Write-WarnMessage {
  param([string]$Message)
  Write-Warning "[authmux-install] $Message"
}

function Fail {
  param([string]$Message)
  throw "[authmux-install] ERROR: $Message"
}

function Test-Truthy {
  param([string]$Value)
  if ([string]::IsNullOrWhiteSpace($Value)) {
    return $false
  }
  switch ($Value.ToLowerInvariant()) {
    "1" { return $true }
    "true" { return $true }
    "yes" { return $true }
    "on" { return $true }
    default { return $false }
  }
}

function Get-Arch {
  $arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()
  switch ($arch) {
    "x64" { return "amd64" }
    "arm64" { return "arm64" }
    default { Fail "Unsupported architecture: $arch" }
  }
}

function Invoke-Download {
  param(
    [Parameter(Mandatory = $true)][string]$Url,
    [Parameter(Mandatory = $true)][string]$OutFile
  )
  Invoke-WebRequest -Uri $Url -OutFile $OutFile
}

function Get-LatestTag {
  $api = "https://api.github.com/repos/$Repo/releases/latest"
  try {
    $release = Invoke-RestMethod -Uri $api
    if ($null -ne $release -and $release.tag_name) {
      return [string]$release.tag_name
    }
  } catch {
    return $null
  }
  return $null
}

function New-TempDir {
  $tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("authmux-install-" + [System.Guid]::NewGuid().ToString("N"))
  New-Item -Path $tempDir -ItemType Directory -Force | Out-Null
  return $tempDir
}

function Install-FromRelease {
  param(
    [Parameter(Mandatory = $true)][string]$ResolvedVersion,
    [Parameter(Mandatory = $true)][string]$Arch
  )

  $verNoV = $ResolvedVersion.TrimStart("v")
  $base = "https://github.com/$Repo/releases/download/$ResolvedVersion"
  $assets = @(
    @{ Name = "${BinaryBase}_${verNoV}_windows_${Arch}.zip"; Type = "zip" },
    @{ Name = "${BinaryBase}_${verNoV}_windows_${Arch}.tar.gz"; Type = "tar.gz" }
  )

  $tempDir = New-TempDir
  try {
    Write-Log "Trying GitHub release install: $ResolvedVersion (windows/$Arch)"
    foreach ($asset in $assets) {
      $assetPath = Join-Path $tempDir $asset.Name
      $assetUrl = "$base/$($asset.Name)"
      try {
        Invoke-Download -Url $assetUrl -OutFile $assetPath
      } catch {
        continue
      }

      $extractDir = Join-Path $tempDir ("extract-" + $asset.Type.Replace(".", "-"))
      New-Item -Path $extractDir -ItemType Directory -Force | Out-Null

      if ($asset.Type -eq "zip") {
        Expand-Archive -Path $assetPath -DestinationPath $extractDir -Force
      } else {
        if (-not (Get-Command tar -ErrorAction SilentlyContinue)) {
          Write-WarnMessage "tar not found; cannot extract $($asset.Name)"
          continue
        }
        & tar -xzf $assetPath -C $extractDir | Out-Null
      }

      $candidate = Get-ChildItem -Path $extractDir -Recurse -File |
        Where-Object { $_.Name -ieq $BinaryName -or $_.Name -ieq $BinaryBase } |
        Select-Object -First 1
      if ($null -eq $candidate) {
        continue
      }

      New-Item -Path $InstallDir -ItemType Directory -Force | Out-Null
      $dest = Join-Path $InstallDir $BinaryName
      Copy-Item -Path $candidate.FullName -Destination $dest -Force
      Write-Log "Installed $BinaryName to $dest"
      return $true
    }

    Write-WarnMessage "Release asset not found for windows/$Arch"
    return $false
  } finally {
    Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
  }
}

function Install-WithGo {
  param([Parameter(Mandatory = $true)][string]$RequestedVersion)

  if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Fail "go is required for fallback install because no matching release binary was found"
  }

  Write-Log "Falling back to go install"
  $pkgVersion = if ($RequestedVersion -eq "latest" -or [string]::IsNullOrWhiteSpace($RequestedVersion)) { "latest" } else { $RequestedVersion }

  $env:GO111MODULE = "on"
  & go install "$ModulePath@$pkgVersion"
  if ($LASTEXITCODE -ne 0) {
    Fail "go install failed"
  }

  $gobin = (& go env GOBIN).Trim()
  if ([string]::IsNullOrWhiteSpace($gobin)) {
    $gopath = (& go env GOPATH).Trim()
    if ([string]::IsNullOrWhiteSpace($gopath)) {
      Fail "Unable to resolve GOPATH from go env"
    }
    $gobin = Join-Path $gopath "bin"
  }

  $source = @(
    (Join-Path $gobin $BinaryName),
    (Join-Path $gobin $BinaryBase)
  ) | Where-Object { Test-Path $_ } | Select-Object -First 1

  if ($null -eq $source) {
    Fail "go install completed but binary not found in $gobin"
  }

  New-Item -Path $InstallDir -ItemType Directory -Force | Out-Null
  $dest = Join-Path $InstallDir $BinaryName
  Copy-Item -Path $source -Destination $dest -Force
  Write-Log "Installed $BinaryName to $dest"
}

function Normalize-PathEntry {
  param([string]$Entry)
  if ([string]::IsNullOrWhiteSpace($Entry)) {
    return ""
  }
  return $Entry.Trim().TrimEnd("\")
}

function Ensure-PathContainsInstallDir {
  param([Parameter(Mandatory = $true)][string]$Dir)

  if (-not (Test-Truthy $AutoPathRaw)) {
    return
  }

  $normalizedDir = Normalize-PathEntry -Entry $Dir
  $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
  $userEntries = @()
  if (-not [string]::IsNullOrWhiteSpace($userPath)) {
    $userEntries = $userPath -split ";"
  }

  $userHasDir = $false
  foreach ($entry in $userEntries) {
    if ((Normalize-PathEntry -Entry $entry) -ieq $normalizedDir) {
      $userHasDir = $true
      break
    }
  }

  if (-not $userHasDir) {
    $newUserPath = if ([string]::IsNullOrWhiteSpace($userPath)) { $Dir } else { "$userPath;$Dir" }
    [Environment]::SetEnvironmentVariable("Path", $newUserPath, "User")
    Write-Log "Added $Dir to User PATH"
  } else {
    Write-Log "User PATH already contains $Dir"
  }

  $sessionHasDir = $false
  foreach ($entry in ($env:Path -split ";")) {
    if ((Normalize-PathEntry -Entry $entry) -ieq $normalizedDir) {
      $sessionHasDir = $true
      break
    }
  }

  if (-not $sessionHasDir) {
    $env:Path = "$Dir;$env:Path"
    Write-Log "Added $Dir to PATH for current PowerShell session"
  }
}

function Main {
  $arch = Get-Arch

  $resolvedVersion = $Version
  if ($Version -eq "latest") {
    $latest = Get-LatestTag
    if ([string]::IsNullOrWhiteSpace($latest)) {
      Write-WarnMessage "Could not resolve latest release tag; will use go install"
      $resolvedVersion = $null
    } else {
      $resolvedVersion = $latest
    }
  }

  $installedFromRelease = $false
  if (-not [string]::IsNullOrWhiteSpace($resolvedVersion) -and $resolvedVersion -ne "latest") {
    $installedFromRelease = Install-FromRelease -ResolvedVersion $resolvedVersion -Arch $arch
  }
  if (-not $installedFromRelease) {
    Install-WithGo -RequestedVersion $Version
  }

  Ensure-PathContainsInstallDir -Dir $InstallDir

  if (-not (Get-Command $BinaryBase -ErrorAction SilentlyContinue)) {
    Write-WarnMessage "$BinaryBase is installed but not currently available in this shell. Open a new terminal and try again."
  }

  Write-Log "Done"
  Write-Log "Run: $BinaryBase --help"
}

Main
