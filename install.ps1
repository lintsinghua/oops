# Install oops without administrator rights. Windows (PowerShell 5.1 and pwsh).
# Override with OOPS_VERSION, OOPS_INSTALL_DIR, and OOPS_DOWNLOAD_BASE (tests).
$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

function Fail([string]$Message) {
	throw $Message
}

function Strip-TrailingSlash([string]$Path) {
	while ($Path.Length -gt 1 -and ($Path.EndsWith("\") -or $Path.EndsWith("/"))) {
		$Path = $Path.Substring(0, $Path.Length - 1)
	}
	return $Path
}

function Read-OopsVersion([string]$Binary) {
	$out = & $Binary version 2>$null
	if (-not $out) { return $null }
	$line = @($out)[0]
	$line = [regex]::Replace([string]$line, "\x1b\[[0-9;]*[A-Za-z]", "")
	$parts = $line -split "\s+"
	if ($parts.Count -ge 2 -and $parts[1]) { return $parts[1] }
	return $null
}

if (-not $env:USERPROFILE) { Fail "USERPROFILE is not set" }

$DownloadBase = if ($env:OOPS_DOWNLOAD_BASE) { $env:OOPS_DOWNLOAD_BASE.TrimEnd("/") } else { "https://github.com/lintsinghua/oops/releases/download" }
$InstallDir = if ($env:OOPS_INSTALL_DIR) { Strip-TrailingSlash $env:OOPS_INSTALL_DIR } else { Strip-TrailingSlash (Join-Path $env:USERPROFILE ".local\bin") }
if (-not $InstallDir) { Fail "OOPS_INSTALL_DIR is empty" }

if ($env:OS -eq "Windows_NT") {
	$Goos = "windows"
	$Ext = "zip"
	$BinName = "oops.exe"
} elseif (Get-Variable -Name IsLinux -ErrorAction SilentlyContinue -ValueOnly) {
	$Goos = "linux"
	$Ext = "tar.gz"
	$BinName = "oops"
} elseif (Get-Variable -Name IsMacOS -ErrorAction SilentlyContinue -ValueOnly) {
	$Goos = "darwin"
	$Ext = "tar.gz"
	$BinName = "oops"
} else {
	Fail "unsupported OS"
}

$ArchRaw = $env:PROCESSOR_ARCHITECTURE
if ($env:PROCESSOR_ARCHITEW6432) { $ArchRaw = $env:PROCESSOR_ARCHITEW6432 }
if (-not $ArchRaw -and (Get-Command uname -ErrorAction SilentlyContinue)) { $ArchRaw = & uname -m }
switch ($ArchRaw) {
	"AMD64" { $Goarch = "amd64" }
	"X64" { $Goarch = "amd64" }
	"x86_64" { $Goarch = "amd64" }
	"ARM64" { $Goarch = "arm64" }
	"aarch64" { $Goarch = "arm64" }
	default { Fail "unsupported architecture: $ArchRaw" }
}

$Version = $env:OOPS_VERSION
if (-not $Version) {
	try {
		$rel = Invoke-RestMethod -Headers @{ "User-Agent" = "oops-install" } -Uri "https://api.github.com/repos/lintsinghua/oops/releases/latest"
	} catch {
		Fail "could not read the latest release; set OOPS_VERSION"
	}
	$Version = [string]$rel.tag_name
	if (-not $Version) { Fail "latest release has no tag_name; set OOPS_VERSION" }
}
if ($Version.StartsWith("v")) { $Version = $Version.Substring(1) }

$Asset = "oops_${Version}_${Goos}_${Goarch}.$Ext"
$Url = "$DownloadBase/v$Version/$Asset"
$SumUrl = "$DownloadBase/v$Version/checksums.txt"
[Console]::Error.WriteLine("Downloading oops $Version ($Goos/$Goarch)")

$Dest = Join-Path $InstallDir $BinName
$Previous = $null
if (Test-Path -LiteralPath $Dest) {
	$Previous = Read-OopsVersion $Dest
	if (-not $Previous) { $Previous = "unknown" }
}

$Tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("oops-install-" + [guid]::NewGuid().ToString("n"))
New-Item -ItemType Directory -Force -Path $Tmp | Out-Null
$staged = $null
try {
	$Archive = Join-Path $Tmp $Asset
	$SumFile = Join-Path $Tmp "checksums.txt"
	try {
		Invoke-WebRequest -UseBasicParsing -Uri $Url -OutFile $Archive
	} catch {
		Fail "download failed: $Url"
	}
	try {
		Invoke-WebRequest -UseBasicParsing -Uri $SumUrl -OutFile $SumFile
	} catch {
		Fail "download failed: $SumUrl"
	}

	$Expected = $null
	foreach ($line in Get-Content -LiteralPath $SumFile) {
		$line = ([string]$line).Trim()
		if (-not $line) { continue }
		$parts = $line -split "\s+", 2
		if ($parts.Count -lt 2) { continue }
		$file = $parts[1].Trim()
		if ($file.StartsWith("*")) { $file = $file.Substring(1) }
		if ($file -eq $Asset) {
			$Expected = $parts[0].Trim().ToLower()
			break
		}
	}
	if (-not $Expected) { Fail "checksums.txt has no entry for $Asset" }
	$Actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $Archive).Hash.ToLower()
	if ($Actual -ne $Expected) { Fail "checksum mismatch for $Asset" }

	$Extract = Join-Path $Tmp "extract"
	New-Item -ItemType Directory -Force -Path $Extract | Out-Null
	if ($Ext -eq "zip") {
		Expand-Archive -LiteralPath $Archive -DestinationPath $Extract -Force
	} else {
		& tar -xzf $Archive -C $Extract
		if ($LASTEXITCODE -ne 0) { Fail "extract failed" }
	}
	$hits = @(Get-ChildItem -LiteralPath $Extract -Recurse -File -Filter $BinName)
	if ($hits.Count -lt 1) { Fail "archive does not contain $BinName" }

	New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
	# A running oops.exe cannot be overwritten. Write the new bytes beside it,
	# rename the running file out of the way, then rename the new file into place.
	$staged = Join-Path $InstallDir ($BinName + ".new")
	Copy-Item -LiteralPath $hits[0].FullName -Destination $staged -Force
	if (Test-Path -LiteralPath $Dest) {
		$old = Join-Path $InstallDir ($BinName + ".old")
		if (Test-Path -LiteralPath $old) {
			Remove-Item -LiteralPath $old -Force -ErrorAction SilentlyContinue
		}
		if (Test-Path -LiteralPath $old) {
			$old = Join-Path $InstallDir ($BinName + ".old." + [guid]::NewGuid().ToString("n"))
		}
		Move-Item -LiteralPath $Dest -Destination $old -Force
		try {
			Move-Item -LiteralPath $staged -Destination $Dest -Force
		} catch {
			Move-Item -LiteralPath $old -Destination $Dest -Force -ErrorAction SilentlyContinue
			throw
		}
		Remove-Item -LiteralPath $old -Force -ErrorAction SilentlyContinue
	} else {
		Move-Item -LiteralPath $staged -Destination $Dest -Force
	}
} finally {
	if ($staged -and (Test-Path -LiteralPath $staged)) {
		Remove-Item -LiteralPath $staged -Force -ErrorAction SilentlyContinue
	}
	Remove-Item -LiteralPath $Tmp -Recurse -Force -ErrorAction SilentlyContinue
}

$Installed = Read-OopsVersion $Dest
if (-not $Installed) { Fail "installed binary did not print a version" }

if ($Previous) {
	Write-Output "upgraded from $Previous to $Installed"
} else {
	Write-Output "installed oops $Installed"
}

$PathSep = ";"
if ($Goos -ne "windows") { $PathSep = ":" }
$OnPath = $false
foreach ($entry in ($env:Path -split [regex]::Escape($PathSep))) {
	if (-not $entry) { continue }
	if ((Strip-TrailingSlash $entry.Trim()) -ieq $InstallDir) {
		$OnPath = $true
		break
	}
}
if (-not $OnPath) {
	if ($Goos -eq "windows") {
		$shown = $InstallDir.Replace("'", "''")
		Write-Output ('$env:Path = ''{0};'' + $env:Path' -f $shown)
	} else {
		Write-Output ('export PATH="{0}:$PATH"' -f $InstallDir)
	}
}

Write-Output "oops init"
