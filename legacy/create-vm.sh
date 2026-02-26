#!/bin/bash
# create-vm.sh - Provision a new KVM VM for a user
# Usage: ./create-vm.sh -n <name> -p <vm-path> -k <ssh-pubkey-file> [options]

set -euo pipefail

# --- Paths ---
BASE_IMAGE="/mnt/nvme1/vms/base-images/jammy-server-cloudimg-amd64.img"

# --- Defaults ---
DEFAULT_USER="ubuntu"
DEFAULT_VCPUS=8
DEFAULT_RAM=16384   # MB
DEFAULT_DISK=100    # GB
NETWORK="default"

usage() {
    cat <<EOF
Usage: $0 -n <vm-name> -p <vm-path> -k <ssh-pubkey-file> [options]

Required:
  -n <name>       VM name (e.g. alice-vm)
  -p <path>       Directory for this VM's files (image, seed ISO, shared data)
  -k <keyfile>    Path to SSH public key file

Options:
  -u <username>   Username inside VM (default: $DEFAULT_USER)
  -c <vcpus>      Number of vCPUs (default: $DEFAULT_VCPUS)
  -m <ram_mb>     RAM in MB (default: $DEFAULT_RAM)
  -d <disk_gb>    Disk size in GB (default: $DEFAULT_DISK)
  -h              Show this help

Layout created under <path>:
  <path>/<name>.qcow2     VM disk image
  <path>/<name>-seed.iso  Cloud-init seed ISO
  <path>/data/            Shared directory (mounted at /mnt/host inside VM)

Both the host user and the VM user can read and write to <path>/data/.

Example:
  $0 -n alice-vm -p /home/alice/my-vm -k /home/alice/.ssh/id_rsa.pub
  $0 -n alice-vm -p /home/alice/my-vm -k /home/alice/.ssh/id_rsa.pub -c 4 -m 8192 -d 50
EOF
    exit 1
}

NAME=""
VM_PATH=""
KEYFILE=""
USERNAME="$DEFAULT_USER"
VCPUS=$DEFAULT_VCPUS
RAM=$DEFAULT_RAM
DISK=$DEFAULT_DISK

while getopts "n:p:k:u:c:m:d:h" opt; do
    case $opt in
        n) NAME="$OPTARG" ;;
        p) VM_PATH="$OPTARG" ;;
        k) KEYFILE="$OPTARG" ;;
        u) USERNAME="$OPTARG" ;;
        c) VCPUS="$OPTARG" ;;
        m) RAM="$OPTARG" ;;
        d) DISK="$OPTARG" ;;
        h) usage ;;
        *) usage ;;
    esac
done

# Validate inputs
[[ -z "$NAME" ]]    && { echo "Error: -n is required."; usage; }
[[ -z "$VM_PATH" ]] && { echo "Error: -p is required."; usage; }
[[ -z "$KEYFILE" ]] && { echo "Error: -k is required."; usage; }
[[ ! -f "$KEYFILE" ]]    && { echo "Error: SSH key file not found: $KEYFILE"; exit 1; }
[[ ! -f "$BASE_IMAGE" ]] && { echo "Error: Base image not found: $BASE_IMAGE"; exit 1; }

VM_PATH=$(realpath -m "$VM_PATH")
SSH_KEY_CONTENT=$(cat "$KEYFILE")
DISK_PATH="$VM_PATH/$NAME.qcow2"
CLOUD_INIT_ISO="$VM_PATH/$NAME-seed.iso"
DATA_DIR="$VM_PATH/data"

# Check if VM already exists
if virsh dominfo "$NAME" &>/dev/null; then
    echo "Error: VM '$NAME' already exists. Use: virsh destroy $NAME && virsh undefine $NAME --remove-all-storage"
    exit 1
fi

HOST_USER="${SUDO_USER:-$(id -un)}"

[[ ! -d "$VM_PATH" ]] && echo "==> Creating VM directory: $VM_PATH" && mkdir -p "$VM_PATH"
[[ ! -d "$DATA_DIR" ]] && echo "==> Creating shared data directory: $DATA_DIR" && mkdir -p "$DATA_DIR"

chown "$HOST_USER": "$VM_PATH"

# The 9p share uses accessmode=mapped: all host-side I/O runs as libvirt-qemu,
# with UID/GID stored in xattrs so the VM sees correct ownership.
# Give libvirt-qemu ownership of the data dir and add the host user to the
# libvirt-qemu group so they can read and write files that QEMU creates.
# The setgid bit ensures files created by either party inherit the libvirt-qemu
# group, keeping both sides in the same group.
chown libvirt-qemu:libvirt-qemu "$DATA_DIR"
chmod 2777 "$DATA_DIR"
usermod -aG libvirt-qemu "$HOST_USER"

echo "==> Creating VM: $NAME"
echo "    User: $USERNAME | vCPUs: $VCPUS | RAM: ${RAM}MB | Disk: ${DISK}GB"
echo "    VM path:    $VM_PATH"
echo "    Shared dir: $DATA_DIR -> /mnt/host (inside VM)"

# Create disk image (copy-on-write from base)
echo "==> Creating disk image..."
qemu-img create -f qcow2 -b "$BASE_IMAGE" -F qcow2 "$DISK_PATH" "${DISK}G"

# Create cloud-init config
echo "==> Writing cloud-init config..."
TMPDIR=$(mktemp -d)
trap "rm -rf $TMPDIR" EXIT

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

# Mount the host-shared directory.
runcmd:
  - mkdir -p /mnt/host
  - mount -t 9p -o trans=virtio,version=9p2000.L,rw hostshare /mnt/host

mounts:
  - [hostshare, /mnt/host, 9p, "trans=virtio,version=9p2000.L,rw,nofail", "0", "0"]
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

virt-install \
    --name "$NAME" \
    --vcpus "$VCPUS" \
    --memory "$RAM" \
    --disk "$DISK_PATH",format=qcow2 \
    --disk "$CLOUD_INIT_ISO",device=cdrom,readonly=on \
    --os-variant ubuntu22.04 \
    --network network="$NETWORK" \
    --import \
    --filesystem "$DATA_DIR",hostshare,type=mount,accessmode=mapped \
    --print-xml > "$VM_XML"

# Use the system QEMU (AppArmor-approved) instead of any custom build
sed -i 's|<emulator>.*qemu-system-x86_64</emulator>|<emulator>/usr/bin/qemu-system-x86_64</emulator>|' "$VM_XML"

virsh define "$VM_XML"
virsh start "$NAME"

# Save VM metadata so ssh-vm.sh can find it by path
cat > "$VM_PATH/.vm" <<EOF
NAME=$NAME
USERNAME=$USERNAME
EOF

echo "==> VM '$NAME' created and booting..."
echo "    Shared dir: $DATA_DIR  <->  /mnt/host (inside VM)"
echo "    Check IP:   virsh domifaddr $NAME"
echo "    Console:    virsh console $NAME  (Ctrl+] to exit)"
echo "    SSH access: ssh -J <host_user>@$(hostname -f) $USERNAME@<vm-ip>"
echo ""
echo "    NOTE: '$HOST_USER' was added to the libvirt-qemu group."
echo "          Log out and back in (or run 'newgrp libvirt-qemu') for it to take effect."
