#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "Usage: $0 <ghcr-image> [project-dir]"
  echo "Example: $0 ghcr.io/yourname/chatgpt2api:billing-v1 /opt/chatgpt2api"
  exit 1
fi

IMAGE="$1"
PROJECT_DIR="${2:-$(pwd)}"

cd "$PROJECT_DIR"

if ! command -v docker >/dev/null 2>&1; then
  echo "docker not found"
  exit 1
fi

cat > docker-compose.override.yml <<EOF
services:
  app:
    image: ${IMAGE}
    pull_policy: always
EOF

echo "[1/4] docker login ghcr.io"
if [[ -n "${GHCR_USERNAME:-}" && -n "${GHCR_TOKEN:-}" ]]; then
  echo "${GHCR_TOKEN}" | docker login ghcr.io -u "${GHCR_USERNAME}" --password-stdin
else
  echo "Skip login (public image). Set GHCR_USERNAME/GHCR_TOKEN if image is private."
fi

echo "[2/4] docker compose pull"
docker compose pull

echo "[3/4] docker compose up -d"
docker compose up -d

echo "[4/4] docker ps (chatgpt2api)"
docker ps --filter "name=chatgpt2api"

echo "Deploy complete."
