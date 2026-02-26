# VM Management

Scripts for provisioning per-user KVM/QEMU VMs on this host.
---

## One-time Setup

Download the Ubuntu 22.04 cloud base image (required before creating any VM):

```bash
wget -P /mnt/nvme1/vms/base-images/ \
  https://cloud-images.ubuntu.com/jammy/current/jammy-server-cloudimg-amd64.img
```

---

## Scripts

### `create-vm.sh` — Create a new VM

```
sudo ./create-vm.sh -n <vm-name> -p <vm-path> -k <ssh-pubkey-file> [options]

Required:
  -n <name>       VM name (e.g. alice-vm)
  -p <path>       Directory for this VM's files (image, seed ISO, shared data)
  -k <keyfile>    Path to user's SSH public key file

Options:
  -u <username>   Username inside VM (default: ubuntu)
  -c <vcpus>      Number of vCPUs (default: 8)
  -m <ram_mb>     RAM in MB (default: 16384)
  -d <disk_gb>    Disk size in GB (default: 100)
```

The script creates the following layout under `<path>`:

```
<path>/<name>.qcow2     VM disk image
<path>/<name>-seed.iso  Cloud-init seed ISO
<path>/data/            Shared directory — mounted at /mnt/host inside the VM
```

Both the host user and the VM user can read and write to `<path>/data/`.

**Examples:**

```bash
# Basic VM
sudo ./create-vm.sh -n alice-vm -p /home/alice/my-vm -k /home/alice/.ssh/id_rsa.pub

# Custom resources
sudo ./create-vm.sh -n alice-vm -p /home/alice/my-vm -k /home/alice/.ssh/id_rsa.pub -c 4 -m 8192 -d 200
```

After creation, connect with:

```bash
./ssh-vm.sh /home/alice/my-vm

# Extra ssh args are forwarded:
./ssh-vm.sh /home/alice/my-vm -L 8080:localhost:8080
```

From a remote machine, jump through the host:

```bash
ssh -J your_user@spokane1 -t your_user@spokane1 ./vms/ssh-vm.sh /home/alice/my-vm
```

---

### `ssh-vm.sh` — SSH into a VM by its path

```bash
./ssh-vm.sh <vm-path> [ssh-args...]
```

Reads the VM name and username from `<vm-path>/.vm` (written by `create-vm.sh`), looks up the IP via `virsh`, and connects. Polls for the IP for up to 30 seconds in case the VM is still booting.

---

### `add-share.sh` — Mount a host directory into an existing VM

Shuts down the VM, attaches the share, and restarts it.

```
sudo ./add-share.sh -n <vm-name> -s <host_dir> [options]

Required:
  -n <name>       VM name
  -s <host_dir>   Host directory to share

Options:
  -i <vm-ip>      VM IP — if provided, auto-configures /etc/fstab inside VM
  -u <username>   VM username for SSH (default: ubuntu)
  -t <tag>        Mount tag name (default: hostshare); use different tags for multiple shares
```

**Examples:**

```bash
# Attach share; prints manual mount instructions
sudo ./add-share.sh -n alice-vm -s /mnt/nvme1/alice

# Attach and auto-configure fstab inside VM
sudo ./add-share.sh -n alice-vm -s /mnt/nvme1/alice -i 192.168.122.10

# Second share with a different tag
sudo ./add-share.sh -n alice-vm -s /mnt/nvme1/datasets -t datasets
```

If `-i` is not provided, run these inside the VM to mount manually:

```bash
sudo mkdir -p /mnt/host
sudo mount -t 9p -o trans=virtio,version=9p2000.L hostshare /mnt/host

# For persistence across VM reboots, add to /etc/fstab:
echo 'hostshare /mnt/host 9p trans=virtio,version=9p2000.L,rw,_netdev,nofail 0 0' | sudo tee -a /etc/fstab
```

---

### `delete-vm.sh` — Delete a VM and its files

```bash
sudo ./delete-vm.sh <vm-name> <vm-path>
```

Stops the VM, undefines it from libvirt, and removes the disk image and seed ISO from `<vm-path>`. The `<vm-path>/data/` directory is left intact — remove it manually if no longer needed.

```bash
sudo ./delete-vm.sh alice-vm /home/alice/my-vm
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
