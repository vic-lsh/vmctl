#!/bin/bash
# create-vm.sh - Provision a new KVM VM for a user
# Usage: ./create-vm.sh -n <name> -k <ssh-pubkey-file> [-u username] [-c vcpus] [-m ram_mb] [-d disk_gb] [-s host_dir]

set -euo pipefail

# --- Paths ---
VM_DIR="/mnt/nvme1/vms"
BASE_IMAGE="$VM_DIR/base-images/jammy-server-cloudimg-amd64.img"

# --- Defaults ---
DEFAULT_USER="ubuntu"
DEFAULT_VCPUS=8
DEFAULT_RAM=16384   # MB
DEFAULT_DISK=100    # GB
NETWORK="default"

usage() {
    cat <<EOF
Usage: $0 -n <vm-name> -k <ssh-pubkey-file> [options]

Required:
  -n <name>       VM name (e.g. alice-vm)
  -k <keyfile>    Path to SSH public key file

Options:
  -u <username>   Username inside VM (default: $DEFAULT_USER)
  -c <vcpus>      Number of vCPUs (default: $DEFAULT_VCPUS)
  -m <ram_mb>     RAM in MB (default: $DEFAULT_RAM)
  -d <disk_gb>    Disk size in GB (default: $DEFAULT_DISK)
  -s <host_dir>   Host directory to mount inside VM at /mnt/host
  -h              Show this help

Example:
  $0 -n alice-vm -k /home/alice/.ssh/id_rsa.pub -c 4 -m 8192 -d 50
  $0 -n alice-vm -k /home/alice/.ssh/id_rsa.pub -s /mnt/nvme1/alice
EOF
    exit 1
}

NAME=""
KEYFILE=""
USERNAME="$DEFAULT_USER"
VCPUS=$DEFAULT_VCPUS
RAM=$DEFAULT_RAM
DISK=$DEFAULT_DISK
SHARE_DIR=""

while getopts "n:k:u:c:m:d:s:h" opt; do
    case $opt in
        n) NAME="$OPTARG" ;;
        k) KEYFILE="$OPTARG" ;;
        u) USERNAME="$OPTARG" ;;
        c) VCPUS="$OPTARG" ;;
        m) RAM="$OPTARG" ;;
        d) DISK="$OPTARG" ;;
        s) SHARE_DIR="$OPTARG" ;;
        h) usage ;;
        *) usage ;;
    esac
done

# Validate inputs
[[ -z "$NAME" || -z "$KEYFILE" ]] && { echo "Error: -n and -k are required."; usage; }
[[ ! -f "$KEYFILE" ]] && { echo "Error: SSH key file not found: $KEYFILE"; exit 1; }
[[ ! -f "$BASE_IMAGE" ]] && { echo "Error: Base image not found: $BASE_IMAGE"; echo "Download it to: $BASE_IMAGE"; exit 1; }
[[ -n "$SHARE_DIR" && ! -d "$SHARE_DIR" ]] && { echo "Error: Share directory not found: $SHARE_DIR"; exit 1; }

SSH_KEY_CONTENT=$(cat "$KEYFILE")
DISK_PATH="$VM_DIR/disks/$NAME.qcow2"
CLOUD_INIT_ISO="$VM_DIR/cloud-init/$NAME-cloud-init.iso"

# Check if VM already exists
if virsh dominfo "$NAME" &>/dev/null; then
    echo "Error: VM '$NAME' already exists. Use: virsh destroy $NAME && virsh undefine $NAME --remove-all-storage"
    exit 1
fi

echo "==> Creating VM: $NAME"
echo "    User: $USERNAME | vCPUs: $VCPUS | RAM: ${RAM}MB | Disk: ${DISK}GB"
[[ -n "$SHARE_DIR" ]] && echo "    Host share: $SHARE_DIR -> /mnt/host"

# Create disk image (copy-on-write from base)
echo "==> Creating disk image..."
qemu-img create -f qcow2 -b "$BASE_IMAGE" -F qcow2 "$DISK_PATH" "${DISK}G"

# Create cloud-init config
echo "==> Writing cloud-init config..."
TMPDIR=$(mktemp -d)
trap "rm -rf $TMPDIR" EXIT

# Build optional mount section for cloud-init
MOUNT_SECTION=""
if [[ -n "$SHARE_DIR" ]]; then
    MOUNT_SECTION=$(cat <<'MOUNTEOF'

# Mount host-shared directory
runcmd:
  - mkdir -p /mnt/host
  - mount -t 9p -o trans=virtio,version=9p2000.L,rw hostshare /mnt/host

mounts:
  - [hostshare, /mnt/host, 9p, "trans=virtio,version=9p2000.L,rw,_netdev,nofail", "0", "0"]
MOUNTEOF
)
fi

cat > "$TMPDIR/user-data" <<EOF
#cloud-config
hostname: $NAME
fqdn: $NAME
manage_etc_hosts: true

users:
  - name: $USERNAME
    ssh_authorized_keys:
      - $SSH_KEY_CONTENT
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: /bin/bash
    lock_passwd: true

# Disable password auth
ssh_pwauth: false

# Grow root partition to fill disk
growpart:
  mode: auto
  devices: ['/']
resize_rootfs: true
$MOUNT_SECTION
EOF

cat > "$TMPDIR/meta-data" <<EOF
instance-id: $NAME
local-hostname: $NAME
EOF

genisoimage -output "$CLOUD_INIT_ISO" \
    -volid cidata -joliet -rock \
    "$TMPDIR/user-data" "$TMPDIR/meta-data" 2>/dev/null

# Launch VM
echo "==> Launching VM..."
VM_XML="$TMPDIR/vm.xml"

SHARE_ARGS=""
[[ -n "$SHARE_DIR" ]] && SHARE_ARGS="--filesystem $SHARE_DIR,hostshare,type=mount,accessmode=passthrough"

virt-install \
    --name "$NAME" \
    --vcpus "$VCPUS" \
    --memory "$RAM" \
    --disk "$DISK_PATH",format=qcow2 \
    --disk "$CLOUD_INIT_ISO",device=cdrom,readonly=on \
    --os-variant ubuntu22.04 \
    --network network="$NETWORK" \
    --import \
    ${SHARE_ARGS} \
    --print-xml > "$VM_XML"

# Use the system QEMU (AppArmor-approved) instead of any custom build
sed -i 's|<emulator>.*qemu-system-x86_64</emulator>|<emulator>/usr/bin/qemu-system-x86_64</emulator>|' "$VM_XML"

virsh define "$VM_XML"
virsh start "$NAME"

echo "==> VM '$NAME' created and booting..."
echo "    Check IP:   virsh domifaddr $NAME"
echo "    Console:    virsh console $NAME  (Ctrl+] to exit)"
echo "    SSH access: ssh -J <host_user>@$(hostname -f) $USERNAME@<vm-ip>"
