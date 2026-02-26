#!/bin/bash
# delete-vm.sh - Remove a VM and its associated files

set -euo pipefail

[[ $# -ne 2 ]] && { echo "Usage: $0 <vm-name> <vm-path>"; exit 1; }

NAME="$1"
VM_PATH="$2"

if ! virsh dominfo "$NAME" &>/dev/null; then
    echo "Error: VM '$NAME' not found."
    exit 1
fi

echo "==> Stopping VM: $NAME"
virsh destroy "$NAME" 2>/dev/null || true

echo "==> Undefining VM..."
virsh undefine "$NAME" 2>/dev/null || true

# Remove disk image and cloud-init seed ISO from the VM path.
# The data/ directory is left intact — it contains user data.
DISK_PATH="$VM_PATH/$NAME.qcow2"
CLOUD_INIT_ISO="$VM_PATH/$NAME-seed.iso"

[[ -f "$DISK_PATH" ]]      && rm -f "$DISK_PATH"      && echo "    Removed: $DISK_PATH"
[[ -f "$CLOUD_INIT_ISO" ]] && rm -f "$CLOUD_INIT_ISO" && echo "    Removed: $CLOUD_INIT_ISO"

DATA_DIR="$VM_PATH/data"
if [[ -d "$DATA_DIR" ]]; then
    echo "    Preserved: $DATA_DIR  (remove manually if no longer needed)"
fi

echo "==> VM '$NAME' deleted."
