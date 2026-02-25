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
sudo ./create-vm.sh -n <vm-name> -k <ssh-pubkey-file> [options]

Required:
  -n <name>       VM name (e.g. alice-vm)
  -k <keyfile>    Path to user's SSH public key file

Options:
  -u <username>   Username inside VM (default: ubuntu)
  -c <vcpus>      Number of vCPUs (default: 8)
  -m <ram_mb>     RAM in MB (default: 16384)
  -d <disk_gb>    Disk size in GB (default: 100)
  -s <host_dir>   Host directory to mount inside VM at /mnt/host
```

**Examples:**

```bash
# Basic VM
sudo ./create-vm.sh -n alice-vm -k /home/alice/.ssh/id_rsa.pub

# Custom resources
sudo ./create-vm.sh -n alice-vm -k /home/alice/.ssh/id_rsa.pub -c 4 -m 8192 -d 200

# With a host directory mounted at /mnt/host inside the VM
sudo ./create-vm.sh -n alice-vm -k /home/alice/.ssh/id_rsa.pub -s /mnt/nvme1/alice
```

After creation, find the VM's IP and connect:

```bash
virsh domifaddr alice-vm

# From the host:
ssh ubuntu@192.168.122.x

# From a remote machine (SSH jump through host):
ssh -J your_user@spokane1 ubuntu@192.168.122.x
```

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

> **Note on permissions:** Host shares use passthrough mode — the VM sees the host's UIDs/GIDs.
> The default `ubuntu` user inside the VM has UID 1000. If the host directory is owned by a
> different user, adjust permissions: `chown -R 1000:1000 /path/to/dir`

---

### `delete-vm.sh` — Delete a VM and all its disks

```bash
sudo ./delete-vm.sh alice-vm
```

This stops the VM, undefines it from libvirt, and removes its disk image.

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

# 2. Resize the disk image on the host
qemu-img resize /mnt/nvme1/vms/disks/alice-vm.qcow2 +50G

# 3. Start and SSH into the VM, then expand the partition and filesystem
virsh start alice-vm
ssh ubuntu@192.168.122.x

sudo growpart /dev/vda 1
sudo resize2fs /dev/vda1
```

> Disks can only be grown, not shrunk.
