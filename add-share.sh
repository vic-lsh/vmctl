#!/bin/bash
# add-share.sh - Add a host directory mount to an existing VM (requires restart)

set -euo pipefail

usage() {
    cat <<EOF
Usage: $0 -n <vm-name> -s <host_dir> [-i <vm-ip>] [-u <username>] [-t <mount-tag>]

Required:
  -n <name>       VM name
  -s <host_dir>   Host directory to share

Options:
  -i <vm-ip>      VM IP (to auto-configure /etc/fstab inside VM)
  -u <username>   VM username for SSH (default: ubuntu)
  -t <tag>        Mount tag name, visible inside VM (default: hostshare)
  -h              Show this help

Example:
  $0 -n alice-vm -s /mnt/nvme1/alice
  $0 -n alice-vm -s /mnt/nvme1/alice -i 192.168.122.10 -u ubuntu
EOF
    exit 1
}

NAME=""
SHARE_DIR=""
VM_IP=""
USERNAME="ubuntu"
TAG="hostshare"

while getopts "n:s:i:u:t:h" opt; do
    case $opt in
        n) NAME="$OPTARG" ;;
        s) SHARE_DIR="$OPTARG" ;;
        i) VM_IP="$OPTARG" ;;
        u) USERNAME="$OPTARG" ;;
        t) TAG="$OPTARG" ;;
        h) usage ;;
        *) usage ;;
    esac
done

[[ -z "$NAME" || -z "$SHARE_DIR" ]] && { echo "Error: -n and -s are required."; usage; }
[[ ! -d "$SHARE_DIR" ]] && { echo "Error: Directory not found: $SHARE_DIR"; exit 1; }

if ! virsh dominfo "$NAME" &>/dev/null; then
    echo "Error: VM '$NAME' not found."
    exit 1
fi

# Check if tag already in use
if virsh dumpxml "$NAME" | grep -q "target dir='$TAG'"; then
    echo "Error: Mount tag '$TAG' is already attached to VM '$NAME'."
    echo "Use -t to specify a different tag name."
    exit 1
fi

# Try to grab IP before shutdown in case user didn't provide it
if [[ -z "$VM_IP" ]]; then
    VM_IP=$(virsh domifaddr "$NAME" 2>/dev/null | grep -oP '\d+\.\d+\.\d+\.\d+' | head -1 || true)
fi

# Shut down VM
STATE=$(virsh domstate "$NAME")
if [[ "$STATE" != "shut off" ]]; then
    echo "==> Shutting down VM: $NAME..."
    virsh shutdown "$NAME"
    for i in $(seq 1 30); do
        sleep 2
        [[ "$(virsh domstate "$NAME")" == "shut off" ]] && break
    done
    if [[ "$(virsh domstate "$NAME")" != "shut off" ]]; then
        echo "    Graceful shutdown timed out, forcing off..."
        virsh destroy "$NAME"
        sleep 2
    fi
fi

# Attach filesystem device (--config = persist across reboots)
echo "==> Attaching filesystem: $SHARE_DIR (tag: $TAG)..."
virsh attach-device "$NAME" --config /dev/stdin <<EOF
<filesystem type='mount' accessmode='passthrough'>
  <source dir='$SHARE_DIR'/>
  <target dir='$TAG'/>
</filesystem>
EOF

# Start VM
echo "==> Starting VM: $NAME..."
virsh start "$NAME"

# If we have an IP, SSH in and configure fstab + mount
if [[ -n "$VM_IP" ]]; then
    echo "==> Waiting for VM to boot (30s)..."
    sleep 30

    echo "==> Configuring /etc/fstab inside VM..."
    ssh -o StrictHostKeyChecking=no -o ConnectTimeout=10 "$USERNAME@$VM_IP" \
        "sudo mkdir -p /mnt/host && \
         grep -q '$TAG' /etc/fstab || echo '$TAG /mnt/host 9p trans=virtio,version=9p2000.L,rw,_netdev,nofail 0 0' | sudo tee -a /etc/fstab && \
         sudo mount /mnt/host && \
         echo 'Mounted at /mnt/host'"
    echo "==> Done."
else
    echo "==> VM started. To finish setup inside the VM, run:"
    echo "    sudo mkdir -p /mnt/host"
    echo "    sudo mount -t 9p -o trans=virtio,version=9p2000.L $TAG /mnt/host"
    echo "    # For persistence across VM reboots, add to /etc/fstab:"
    echo "    echo '$TAG /mnt/host 9p trans=virtio,version=9p2000.L,rw,_netdev,nofail 0 0' | sudo tee -a /etc/fstab"
fi
