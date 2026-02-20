#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "Usage: $0 <tag-version> [dist-dir] [out-dir]" >&2
  exit 1
fi

VERSION_TAG="$1" # vX.Y.Z
DIST_DIR="${2:-dist}"
OUT_DIR="${3:-${DIST_DIR}/package-manifests}"
REPO="${AUTHMUX_REPO:-derekurban2001/authmux}"
WIN_PACKAGE_ID="${AUTHMUX_WINGET_PACKAGE_ID:-DerekUrban.AuthMux}"
PUBLISHER="${AUTHMUX_PUBLISHER:-Derek Urban}"
PUBLISHER_URL="${AUTHMUX_PUBLISHER_URL:-https://github.com/derekurban2001}"

VERSION="${VERSION_TAG#v}"
CHECKSUMS_FILE="${DIST_DIR}/checksums.txt"
BASE_URL="https://github.com/${REPO}/releases/download/${VERSION_TAG}"

need_file() {
  [[ -f "$1" ]] || {
    echo "Required file not found: $1" >&2
    exit 1
  }
}

checksum_for_asset() {
  local asset="$1"
  awk -v a="$asset" '$2 == a { print $1; exit }' "$CHECKSUMS_FILE"
}

require_checksum() {
  local asset="$1"
  local sum
  sum="$(checksum_for_asset "$asset")"
  if [[ -z "$sum" ]]; then
    echo "Missing checksum entry for asset: $asset" >&2
    exit 1
  fi
  printf "%s" "$sum"
}

need_file "$CHECKSUMS_FILE"

linux_amd64="authmux_${VERSION}_linux_amd64.tar.gz"
linux_arm64="authmux_${VERSION}_linux_arm64.tar.gz"
darwin_amd64="authmux_${VERSION}_darwin_amd64.tar.gz"
darwin_arm64="authmux_${VERSION}_darwin_arm64.tar.gz"
windows_amd64="authmux_${VERSION}_windows_amd64.zip"
windows_arm64="authmux_${VERSION}_windows_arm64.zip"

linux_amd64_sha="$(require_checksum "$linux_amd64")"
linux_arm64_sha="$(require_checksum "$linux_arm64")"
darwin_amd64_sha="$(require_checksum "$darwin_amd64")"
darwin_arm64_sha="$(require_checksum "$darwin_arm64")"
windows_amd64_sha="$(require_checksum "$windows_amd64")"
windows_arm64_sha="$(require_checksum "$windows_arm64")"

mkdir -p "$OUT_DIR"

homebrew_dir="${OUT_DIR}/homebrew"
winget_dir="${OUT_DIR}/winget/manifests/d/${WIN_PACKAGE_ID%%.*}/${WIN_PACKAGE_ID#*.}/${VERSION}"
scoop_dir="${OUT_DIR}/scoop"
choco_dir="${OUT_DIR}/chocolatey/tools"

mkdir -p "$homebrew_dir" "$winget_dir" "$scoop_dir" "$choco_dir"

cat > "${homebrew_dir}/authmux.rb" <<EOF
class Authmux < Formula
  desc "Profile-based authentication manager for Claude Code and OpenAI Codex CLI"
  homepage "https://github.com/${REPO}"
  version "${VERSION}"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "${BASE_URL}/${darwin_arm64}"
      sha256 "${darwin_arm64_sha}"
    else
      url "${BASE_URL}/${darwin_amd64}"
      sha256 "${darwin_amd64_sha}"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "${BASE_URL}/${linux_arm64}"
      sha256 "${linux_arm64_sha}"
    else
      url "${BASE_URL}/${linux_amd64}"
      sha256 "${linux_amd64_sha}"
    end
  end

  def install
    bin.install "authmux"
  end

  test do
    assert_match "authmux", shell_output("#{bin}/authmux --help")
  end
end
EOF

cat > "${winget_dir}/${WIN_PACKAGE_ID}.yaml" <<EOF
PackageIdentifier: ${WIN_PACKAGE_ID}
PackageVersion: ${VERSION}
DefaultLocale: en-US
ManifestType: version
ManifestVersion: 1.9.0
EOF

cat > "${winget_dir}/${WIN_PACKAGE_ID}.installer.yaml" <<EOF
PackageIdentifier: ${WIN_PACKAGE_ID}
PackageVersion: ${VERSION}
Installers:
- Architecture: x64
  InstallerType: zip
  NestedInstallerType: portable
  NestedInstallerFiles:
  - RelativeFilePath: authmux.exe
    PortableCommandAlias: authmux
  InstallerUrl: ${BASE_URL}/${windows_amd64}
  InstallerSha256: ${windows_amd64_sha}
- Architecture: arm64
  InstallerType: zip
  NestedInstallerType: portable
  NestedInstallerFiles:
  - RelativeFilePath: authmux.exe
    PortableCommandAlias: authmux
  InstallerUrl: ${BASE_URL}/${windows_arm64}
  InstallerSha256: ${windows_arm64_sha}
Commands:
- authmux
ManifestType: installer
ManifestVersion: 1.9.0
EOF

cat > "${winget_dir}/${WIN_PACKAGE_ID}.locale.en-US.yaml" <<EOF
PackageIdentifier: ${WIN_PACKAGE_ID}
PackageVersion: ${VERSION}
PackageLocale: en-US
Publisher: ${PUBLISHER}
PublisherUrl: ${PUBLISHER_URL}
PublisherSupportUrl: https://github.com/${REPO}/issues
PackageName: AuthMux
PackageUrl: https://github.com/${REPO}
ShortDescription: Profile-based authentication manager for Claude Code and OpenAI Codex CLI
License: MIT
LicenseUrl: https://github.com/${REPO}/blob/main/LICENSE
Moniker: authmux
Tags:
- auth
- claude
- codex
ManifestType: defaultLocale
ManifestVersion: 1.9.0
EOF

cat > "${scoop_dir}/authmux.json" <<EOF
{
  "version": "${VERSION}",
  "description": "Profile-based authentication manager for Claude Code and OpenAI Codex CLI",
  "homepage": "https://github.com/${REPO}",
  "license": "MIT",
  "architecture": {
    "64bit": {
      "url": "${BASE_URL}/${windows_amd64}",
      "hash": "${windows_amd64_sha}"
    },
    "arm64": {
      "url": "${BASE_URL}/${windows_arm64}",
      "hash": "${windows_arm64_sha}"
    }
  },
  "bin": "authmux.exe",
  "checkver": {
    "github": "${REPO}"
  },
  "autoupdate": {
    "architecture": {
      "64bit": {
        "url": "https://github.com/${REPO}/releases/download/v\$version/authmux_\$version_windows_amd64.zip"
      },
      "arm64": {
        "url": "https://github.com/${REPO}/releases/download/v\$version/authmux_\$version_windows_arm64.zip"
      }
    }
  }
}
EOF

cat > "${choco_dir}/../authmux.nuspec" <<EOF
<?xml version="1.0"?>
<package>
  <metadata>
    <id>authmux</id>
    <version>${VERSION}</version>
    <title>AuthMux</title>
    <authors>${PUBLISHER}</authors>
    <projectUrl>https://github.com/${REPO}</projectUrl>
    <licenseUrl>https://github.com/${REPO}/blob/main/LICENSE</licenseUrl>
    <requireLicenseAcceptance>false</requireLicenseAcceptance>
    <description>Profile-based authentication manager for Claude Code and OpenAI Codex CLI.</description>
    <summary>Manage multiple auth contexts for Claude Code and Codex CLI.</summary>
    <tags>authmux auth claude codex</tags>
  </metadata>
</package>
EOF

cat > "${choco_dir}/chocolateyinstall.ps1" <<EOF
\$ErrorActionPreference = 'Stop'

\$packageName = 'authmux'
\$toolsDir = Split-Path \$MyInvocation.MyCommand.Definition

\$url64 = '${BASE_URL}/${windows_amd64}'
\$checksum64 = '${windows_amd64_sha}'
\$urlArm64 = '${BASE_URL}/${windows_arm64}'
\$checksumArm64 = '${windows_arm64_sha}'

\$isArm64 = (\$env:PROCESSOR_ARCHITECTURE -eq 'ARM64' -or \$env:PROCESSOR_ARCHITEW6432 -eq 'ARM64')
\$downloadUrl = if (\$isArm64) { \$urlArm64 } else { \$url64 }
\$downloadChecksum = if (\$isArm64) { \$checksumArm64 } else { \$checksum64 }

Install-ChocolateyZipPackage -PackageName \$packageName -Url64bit \$downloadUrl -Checksum64 \$downloadChecksum -ChecksumType64 'sha256' -UnzipLocation \$toolsDir

Install-BinFile -Name 'authmux' -Path (Join-Path \$toolsDir 'authmux.exe')
EOF

cat > "${choco_dir}/chocolateyuninstall.ps1" <<EOF
\$ErrorActionPreference = 'Stop'
Uninstall-BinFile -Name 'authmux'
EOF

cat > "${OUT_DIR}/metadata.json" <<EOF
{
  "version": "${VERSION}",
  "tag": "${VERSION_TAG}",
  "repo": "${REPO}",
  "release_url": "${BASE_URL}"
}
EOF

echo "Generated package manifests in ${OUT_DIR}"
