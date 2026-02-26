#!/bin/bash
# ssh-vm.sh - SSH into a VM by its path

set -euo pipefail

[[ $# -lt 1 ]] && { echo "Usage: $0 <vm-path> [ssh-args...]"; exit 1; }

VM_PATH=$(realpath "$1")
shift

VM_INFO="$VM_PATH/.vm"
[[ ! -f "$VM_INFO" ]] && { echo "Error: No VM info found at $VM_INFO"; exit 1; }

NAME=$(grep ^NAME=     "$VM_INFO" | cut -d= -f2)
USERNAME=$(grep ^USERNAME= "$VM_INFO" | cut -d= -f2)

STATE=$(virsh domstate "$NAME" 2>/dev/null || true)
[[ "$STATE" != "running" ]] && { echo "Error: VM '$NAME' is not running (state: ${STATE:-not found})"; exit 1; }

echo "==> Looking up IP for $NAME..."
IP=""
for i in $(seq 1 10); do
    IP=$(virsh domifaddr "$NAME" 2>/dev/null | grep -oP '\d+\.\d+\.\d+\.\d+' | head -1 || true)
    [[ -n "$IP" ]] && break
    echo "    Waiting for IP... ($i/10)"
    sleep 3
done

[[ -z "$IP" ]] && { echo "Error: Could not get IP for '$NAME'. Try again once it finishes booting."; exit 1; }

echo "==> Connecting to $USERNAME@$IP"
exec ssh -o StrictHostKeyChecking=no "$USERNAME@$IP" "$@"
