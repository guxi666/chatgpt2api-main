param(
  [Parameter(Mandatory = $true)]
  [string]$GitHubUser,
  [string]$ImageName = "chatgpt2api",
  [string]$Tag = "billing-v1",
  [string]$Branch = "main",
  [string]$CommitMessage = "feat: email register + yipay billing"
)

$ErrorActionPreference = "Stop"

function Require-Command([string]$Name) {
  if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
    throw "Required command not found: $Name"
  }
}

Require-Command "git"
Require-Command "docker"

$repoRoot = Split-Path -Parent $PSScriptRoot
Set-Location $repoRoot

$image = "ghcr.io/$GitHubUser/$ImageName`:$Tag"

Write-Host "[1/6] Checking git status..."
$status = git status --porcelain
if ($status) {
  Write-Host "[2/6] Committing changes..."
  git add .
  git commit -m $CommitMessage
} else {
  Write-Host "[2/6] No local changes to commit."
}

Write-Host "[3/6] Pushing source to origin/$Branch..."
git push origin $Branch

Write-Host "[4/6] Logging in to ghcr.io..."
docker login ghcr.io

Write-Host "[5/6] Building image $image ..."
docker build -t $image .

Write-Host "[6/6] Pushing image $image ..."
docker push $image

Write-Host ""
Write-Host "Done."
Write-Host "Use this image on server:"
Write-Host "  $image"
