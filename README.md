# VM Management

`vmctl` is a CLI tool for provisioning per-user KVM/QEMU VMs on this host, with cloud-init and 9p shared storage.

---

## One-time Setup

Download the Ubuntu 22.04 cloud base image (required before creating any VM):

```bash
wget -P /mnt/nvme1/vms/base-images/ \
  https://cloud-images.ubuntu.com/jammy/current/jammy-server-cloudimg-amd64.img
```

## Install

```bash
cd vmctl && go build -o vmctl .
sudo cp vmctl /usr/local/bin/
```

## Quick Start

```bash
# 1. Set up a directory for the VM
mkdir -p ~/my-vm

# 2. (Optional) Add a config.yaml to override defaults (vcpus, ram, disk, etc.)
cat > ~/my-vm/config.yaml <<'EOF'
defaults:
  vcpus: 4
  ram_mb: 8192
EOF

# 3. Create the VM
sudo vmctl create -n my-vm -p ~/my-vm -k ~/.ssh/id_rsa.pub

# 4. SSH into it (waits for boot automatically)
vmctl ssh ~/my-vm

# 5. When done, delete it (preserves data/ directory)
sudo vmctl delete my-vm ~/my-vm
```

After creation, the VM directory contains:

```
~/my-vm/
├── config.yaml          # (optional) per-VM config overrides
├── .vm                  # metadata used by vmctl ssh/delete
├── my-vm.qcow2          # disk image
├── my-vm-seed.iso        # cloud-init seed ISO
└── data/                # shared directory, mounted at /mnt/host inside the VM
```

The `data/` directory is readable and writable from both the host and the VM, useful for transferring files without SSH/SCP.

---

## Configuration (optional)

Config is loaded in layers, with each layer overriding the previous:

1. **Built-in defaults** (shown below)
2. **Global config:** `~/.config/vmctl/config.yaml`
3. **Per-path config:** `<vm-path>/config.yaml`
4. **CLI flags** (always win)

```yaml
base_image: /mnt/nvme1/vms/base-images/jammy-server-cloudimg-amd64.img
network: default
defaults:
  user: ubuntu
  vcpus: 8
  ram_mb: 16384
  disk_gb: 100
```

To give a specific VM different settings, place a `config.yaml` in its directory before running `vmctl create`:

```bash
mkdir -p ~/my-vm
cat > ~/my-vm/config.yaml <<'EOF'
defaults:
  vcpus: 4
  ram_mb: 8192
  disk_gb: 50
EOF
sudo vmctl create -n my-vm -p ~/my-vm -k ~/.ssh/id_rsa.pub
```

---

## Commands

### `vmctl create` — Create a new VM

```
sudo vmctl create -n <vm-name> -p <vm-path> -k <ssh-pubkey-file> [options]

Required:
  -n, --name <name>       VM name (e.g. alice-vm)
  -p, --path <path>       Directory for this VM's files (image, seed ISO, shared data)
  -k, --key <keyfile>     Path to user's SSH public key file

Options:
  -u, --user <username>   Username inside VM (default: ubuntu)
  -c, --vcpus <vcpus>     Number of vCPUs (default: 8)
  -m, --memory <ram_mb>   RAM in MB (default: 16384)
  -d, --disk <disk_gb>    Disk size in GB (default: 100)
```

The command creates the following layout under `<path>`:

```
<path>/<name>.qcow2     VM disk image
<path>/<name>-seed.iso  Cloud-init seed ISO
<path>/data/            Shared directory — mounted at /mnt/host inside the VM
```

Both the host user and the VM user can read and write to `<path>/data/`.

**Examples:**

```bash
# Basic VM
sudo vmctl create -n alice-vm -p /home/alice/my-vm -k /home/alice/.ssh/id_rsa.pub

# Custom resources
sudo vmctl create -n alice-vm -p /home/alice/my-vm -k /home/alice/.ssh/id_rsa.pub -c 4 -m 8192 -d 200
```

After creation, connect with:

```bash
vmctl ssh /home/alice/my-vm

# Extra ssh args are forwarded:
vmctl ssh /home/alice/my-vm -- -L 8080:localhost:8080
```

From a remote machine, jump through the host:

```bash
ssh -J your_user@spokane1 -t your_user@spokane1 vmctl ssh /home/alice/my-vm
```

---

### `vmctl ssh` — SSH into a VM by its path

```bash
vmctl ssh <vm-path> [-- ssh-args...]
```

Reads the VM name and username from `<vm-path>/.vm` (written by `vmctl create`), looks up the IP via `virsh`, and connects. Polls for the IP for up to 30 seconds in case the VM is still booting.

---

### `vmctl add-share` — Mount a host directory into an existing VM

Shuts down the VM, attaches the share, and restarts it.

```
sudo vmctl add-share -n <vm-name> -s <host_dir> [options]

Required:
  -n, --name <name>       VM name
  -s, --share <host_dir>  Host directory to share

Options:
  -i, --ip <vm-ip>        VM IP — if provided, auto-configures /etc/fstab inside VM
  -u, --user <username>   VM username for SSH (default: ubuntu)
  -t, --tag <tag>         Mount tag name (default: hostshare); use different tags for multiple shares
```

**Examples:**

```bash
# Attach share; prints manual mount instructions
sudo vmctl add-share -n alice-vm -s /mnt/nvme1/alice

# Attach and auto-configure fstab inside VM
sudo vmctl add-share -n alice-vm -s /mnt/nvme1/alice -i 192.168.122.10

# Second share with a different tag
sudo vmctl add-share -n alice-vm -s /mnt/nvme1/datasets -t datasets
```

If `--ip` is not provided, run these inside the VM to mount manually:

```bash
sudo mkdir -p /mnt/host
sudo mount -t 9p -o trans=virtio,version=9p2000.L hostshare /mnt/host

# For persistence across VM reboots, add to /etc/fstab:
echo 'hostshare /mnt/host 9p trans=virtio,version=9p2000.L,rw,_netdev,nofail 0 0' | sudo tee -a /etc/fstab
```

---

### `vmctl delete` — Delete a VM and its files

```bash
sudo vmctl delete <vm-name> <vm-path>
```

Stops the VM, undefines it from libvirt, and removes the disk image and seed ISO from `<vm-path>`. The `<vm-path>/data/` directory is left intact — remove it manually if no longer needed.

```bash
sudo vmctl delete alice-vm /home/alice/my-vm
```

---

## Common `virsh` Commands

```bash
virsh list --all              # list all VMs and their state
virsh domifaddr alice-vm      # get VM IP address
virsh start alice-vm          # start a stopped VM
virsh shutdown alice-vm       # graceful shutdown (via ACPI)
virsh destroy alice-vm        # force off (like pulling the power)
virsh console alice-vm        # serial console (exit with Ctrl+])
virsh autostart alice-vm      # start VM automatically on host boot
```

## Resizing a VM's Disk

```bash
# 1. Shut down the VM
virsh shutdown alice-vm

# 2. Resize the disk image on the host (path is wherever you placed the VM)
qemu-img resize /home/alice/my-vm/alice-vm.qcow2 +50G

# 3. Start and SSH into the VM, then expand the partition and filesystem
virsh start alice-vm
ssh ubuntu@192.168.122.x

sudo growpart /dev/vda 1
sudo resize2fs /dev/vda1
```

> Disks can only be grown, not shrunk.

---

## Legacy

The original bash scripts (`create-vm.sh`, `delete-vm.sh`, `ssh-vm.sh`, `add-share.sh`) are in the `legacy/` directory. They are deprecated and will be removed in a future cleanup.
