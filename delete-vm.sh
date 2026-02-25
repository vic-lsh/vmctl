#!/bin/bash
# delete-vm.sh - Remove a VM and all its disk images

set -euo pipefail

[[ $# -ne 1 ]] && { echo "Usage: $0 <vm-name>"; exit 1; }

NAME="$1"
VM_DIR="/mnt/nvme1/vms"

if ! virsh dominfo "$NAME" &>/dev/null; then
    echo "Error: VM '$NAME' not found."
    exit 1
fi

echo "==> Stopping VM: $NAME"
virsh destroy "$NAME" 2>/dev/null || true

echo "==> Undefining VM and removing storage..."
virsh undefine "$NAME" --remove-all-storage 2>/dev/null || true

# Also clean up cloud-init ISO (not tracked by virsh storage pool)
CLOUD_INIT_ISO="$VM_DIR/cloud-init/$NAME-cloud-init.iso"
[[ -f "$CLOUD_INIT_ISO" ]] && rm -f "$CLOUD_INIT_ISO" && echo "    Removed: $CLOUD_INIT_ISO"

echo "==> VM '$NAME' deleted."
