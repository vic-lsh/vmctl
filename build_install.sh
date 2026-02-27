#!/bin/bash
set -euo pipefail

cd "$(dirname "$0")"

echo "==> Building vmctl..."
go build -o vmctl .

echo "==> Installing to /usr/local/bin/vmctl..."
sudo cp vmctl /usr/local/bin/vmctl

echo "==> Done. Version check:"
vmctl --help | head -1
