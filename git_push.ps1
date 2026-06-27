param(
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$ArgsList
)

$ErrorActionPreference = "Stop"

$rootDir = $PSScriptRoot
$dryRun = $false
$commitArgs = @()

foreach ($arg in $ArgsList) {
    if ($arg -eq "--dry-run") {
        $dryRun = $true
    } else {
        $commitArgs += $arg
    }
}

function Fail([string]$Message) {
    Write-Host "ERROR: $Message" -ForegroundColor Red
    exit 1
}

function Run-Git {
    param(
        [Parameter(Mandatory = $true)]
        [string[]]$GitArgs
    )

    & git @GitArgs
    if ($LASTEXITCODE -ne 0) {
        Fail ("Git command failed: git " + ($GitArgs -join " "))
    }
}

Write-Host "Start git push script..."

Write-Host "[1/6] Enter project directory..."
Set-Location -Path $rootDir

Write-Host "[2/6] Check Git..."
if (-not (Get-Command git -ErrorAction SilentlyContinue)) {
    Fail "Git is not installed or not added to PATH."
}

Write-Host "[3/6] Check repository..."
if (-not (Test-Path (Join-Path $rootDir ".git"))) {
    Fail "Current directory is not a Git repository."
}

Write-Host "[4/6] Detect current branch..."
$currentBranch = (& git branch --show-current).Trim()
if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($currentBranch)) {
    Fail "Failed to detect the current branch."
}
Write-Host "Current branch: $currentBranch"

Write-Host '[5/6] Check remote "origin"...'
& git remote get-url origin | Out-Null
if ($LASTEXITCODE -ne 0) {
    Fail 'Remote "origin" is not configured.'
}

Write-Host "[6/6] Stage all changes..."
Run-Git -GitArgs @("add", "--all")

& git diff --cached --quiet
$hasChanges = ($LASTEXITCODE -ne 0)

if ($hasChanges) {
    $commitMessage = ($commitArgs -join " ").Trim()
    if ([string]::IsNullOrWhiteSpace($commitMessage)) {
        $commitMessage = "Update $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')"
    }

    Write-Host "Changes detected."
    Write-Host "Commit message: $commitMessage"

    if ($dryRun) {
        Write-Host "Dry run enabled. Skip commit and push."
        Write-Host ""
        Write-Host "Done."
        Write-Host "Remote: origin"
        Write-Host "Branch: $currentBranch"
        exit 0
    }

    Write-Host "Commit changes..."
    Run-Git -GitArgs @("commit", "-m", $commitMessage)
} else {
    Write-Host "No file changes to commit."
    if ($dryRun) {
        Write-Host "Dry run enabled. Skip push."
        Write-Host ""
        Write-Host "Done."
        Write-Host "Remote: origin"
        Write-Host "Branch: $currentBranch"
        exit 0
    }
}

Write-Host "Push branch `"$currentBranch`" to origin..."
Run-Git -GitArgs @("push", "-u", "origin", $currentBranch)

Write-Host ""
Write-Host "Done."
Write-Host "Remote: origin"
Write-Host "Branch: $currentBranch"
