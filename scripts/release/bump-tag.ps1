#!/usr/bin/env pwsh
param(
    [switch]$Patch,
    [switch]$Minor,
    [switch]$Major,
    [Alias("dry-run")]
    [switch]$DryRun,
    [string]$Remote = "origin",
    [Alias("no-fetch")]
    [switch]$NoFetch,
    [Alias("allow-dirty")]
    [switch]$AllowDirty,
    [switch]$Help
)

$ErrorActionPreference = "Stop"

function Fail([string]$Message) {
    Write-Error $Message
    exit 1
}

function ShowUsage() {
    Write-Host "Usage:"
    Write-Host "  ./scripts/release/bump-tag.ps1 --patch|--minor|--major [options]"
    Write-Host ""
    Write-Host "Options:"
    Write-Host "  --dry-run        Print actions without creating/pushing tag"
    Write-Host "  --remote <name>  Remote to push to (default: origin)"
    Write-Host "  --no-fetch       Skip fetching tags before calculating next version"
    Write-Host "  --allow-dirty    Allow running with uncommitted changes"
}

function RunGit([string[]]$GitArgs) {
    & git @GitArgs
    if ($LASTEXITCODE -ne 0) {
        Fail ("git {0} failed with exit code {1}" -f ($GitArgs -join " "), $LASTEXITCODE)
    }
}

function GitOut([string[]]$GitArgs) {
    $output = & git @GitArgs
    if ($LASTEXITCODE -ne 0) {
        Fail ("git {0} failed with exit code {1}" -f ($GitArgs -join " "), $LASTEXITCODE)
    }
    return ($output -join "`n").Trim()
}

$selected = 0
if ($Patch) { $selected += 1 }
if ($Minor) { $selected += 1 }
if ($Major) { $selected += 1 }
if ($Help) {
    ShowUsage
    exit 0
}
if ($selected -ne 1) {
    Fail "Choose exactly one bump type: --patch, --minor, or --major"
}

$bump = if ($Patch) { "patch" } elseif ($Minor) { "minor" } else { "major" }

[void](GitOut @("rev-parse", "--is-inside-work-tree"))
[void](GitOut @("remote", "get-url", $Remote))

if (-not $AllowDirty) {
    $dirty = GitOut @("status", "--porcelain")
    if (-not [string]::IsNullOrWhiteSpace($dirty)) {
        Fail "Working tree is dirty. Commit/stash changes or rerun with --allow-dirty."
    }
}

if (-not $NoFetch) {
    RunGit @("fetch", "--tags", "--prune", $Remote)
}

$branch = GitOut @("rev-parse", "--abbrev-ref", "HEAD")
if ($branch -eq "HEAD") {
    Fail "Detached HEAD is not supported. Checkout a branch first."
}

$tags = GitOut @("tag", "--list", "v*", "--sort=-v:refname")
$latest = "v0.0.0"
if (-not [string]::IsNullOrWhiteSpace($tags)) {
    foreach ($t in ($tags -split "`n")) {
        $tag = $t.Trim()
        if ($tag -match '^v\d+\.\d+\.\d+$') {
            $latest = $tag
            break
        }
    }
}

if ($latest -notmatch '^v(\d+)\.(\d+)\.(\d+)$') {
    Fail "Latest version tag is invalid: $latest"
}

$majorV = [int]$Matches[1]
$minorV = [int]$Matches[2]
$patchV = [int]$Matches[3]

switch ($bump) {
    "major" { $majorV += 1; $minorV = 0; $patchV = 0 }
    "minor" { $minorV += 1; $patchV = 0 }
    "patch" { $patchV += 1 }
}

$next = "v$majorV.$minorV.$patchV"

$exists = & git rev-parse -q --verify "refs/tags/$next"
if ($LASTEXITCODE -eq 0) {
    Fail "Tag already exists: $next"
}

Write-Host "Current version tag: $latest"
Write-Host "Next version tag:    $next"
Write-Host "Branch:              $branch"
Write-Host "Remote:              $Remote"

if ($DryRun) {
    Write-Host ""
    Write-Host "Dry run - no changes made."
    Write-Host "Would run:"
    Write-Host "  git tag -a $next -m `"release $next`""
    Write-Host "  git push $Remote $branch"
    Write-Host "  git push $Remote $next"
    exit 0
}

RunGit @("tag", "-a", $next, "-m", "release $next")
RunGit @("push", $Remote, $branch)
RunGit @("push", $Remote, $next)

Write-Host ""
Write-Host "Release tag pushed: $next"
