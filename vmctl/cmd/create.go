package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"vmctl/internal"

	"github.com/spf13/cobra"
)

var emulatorRegexp = regexp.MustCompile(`<emulator>.*qemu-system-x86_64</emulator>`)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new KVM VM",
	Long: `Create a new KVM VM with cloud-init provisioning and 9p shared storage.

Layout created under <path>:
  <path>/<name>.qcow2     VM disk image
  <path>/<name>-seed.iso  Cloud-init seed ISO
  <path>/data/            Shared directory (mounted at /mnt/host inside VM)`,
	Example: `  vmctl create -n alice-vm -p /home/alice/my-vm -k ~/.ssh/id_rsa.pub
  vmctl create -n alice-vm -p /home/alice/my-vm -k ~/.ssh/id_rsa.pub -c 4 -m 8192 -d 50`,
	RunE: runCreate,
}

func init() {
	f := createCmd.Flags()
	f.StringP("name", "n", "", "VM name (required)")
	f.StringP("path", "p", "", "Directory for VM files (required)")
	f.StringP("key", "k", "", "Path to SSH public key file (required)")
	f.StringP("user", "u", "", "Username inside VM")
	f.IntP("vcpus", "c", 0, "Number of vCPUs")
	f.IntP("memory", "m", 0, "RAM in MB")
	f.IntP("disk", "d", 0, "Disk size in GB")
	createCmd.MarkFlagRequired("name")
	createCmd.MarkFlagRequired("path")
	createCmd.MarkFlagRequired("key")
}

func runCreate(cmd *cobra.Command, args []string) error {
	name, _ := cmd.Flags().GetString("name")
	vmPath, _ := cmd.Flags().GetString("path")
	keyFile, _ := cmd.Flags().GetString("key")

	// Load config with per-path overrides
	cfg := GetConfigForPath(vmPath)
	username, _ := cmd.Flags().GetString("user")
	vcpus, _ := cmd.Flags().GetInt("vcpus")
	ramMB, _ := cmd.Flags().GetInt("memory")
	diskGB, _ := cmd.Flags().GetInt("disk")

	// Apply config defaults where flags weren't set
	if username == "" {
		username = cfg.Defaults.User
	}
	if vcpus == 0 {
		vcpus = cfg.Defaults.VCPUs
	}
	if ramMB == 0 {
		ramMB = cfg.Defaults.RAMMB
	}
	if diskGB == 0 {
		diskGB = cfg.Defaults.DiskGB
	}

	// Validate inputs
	keyData, err := os.ReadFile(keyFile)
	if err != nil {
		return fmt.Errorf("SSH key file not found: %s", keyFile)
	}
	sshKey := strings.TrimSpace(string(keyData))

	if _, err := os.Stat(cfg.BaseImage); os.IsNotExist(err) {
		return fmt.Errorf("base image not found: %s", cfg.BaseImage)
	}

	vmPath, err = filepath.Abs(vmPath)
	if err != nil {
		return err
	}

	// Check if VM already exists
	if err := internal.DomInfo(name); err == nil {
		return fmt.Errorf("VM '%s' already exists. Use: virsh destroy %s && virsh undefine %s --remove-all-storage", name, name, name)
	}

	diskPath := filepath.Join(vmPath, name+".qcow2")
	isoPath := filepath.Join(vmPath, name+"-seed.iso")
	dataDir := filepath.Join(vmPath, "data")

	hostUser := os.Getenv("SUDO_USER")
	if hostUser == "" {
		hostUser = os.Getenv("USER")
	}

	// Create directories
	if err := os.MkdirAll(vmPath, 0755); err != nil {
		return fmt.Errorf("creating VM directory: %w", err)
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("creating data directory: %w", err)
	}

	// chown VM path to host user
	if out, err := exec.Command("chown", hostUser+":", vmPath).CombinedOutput(); err != nil {
		return fmt.Errorf("chown %s: %w\n%s", vmPath, err, out)
	}

	// Set up data dir ownership and permissions for 9p mapped access
	if out, err := exec.Command("chown", "libvirt-qemu:libvirt-qemu", dataDir).CombinedOutput(); err != nil {
		return fmt.Errorf("chown data dir: %w\n%s", err, out)
	}
	if err := os.Chmod(dataDir, 02777); err != nil {
		return fmt.Errorf("chmod data dir: %w", err)
	}
	if out, err := exec.Command("usermod", "-aG", "libvirt-qemu", hostUser).CombinedOutput(); err != nil {
		return fmt.Errorf("usermod: %w\n%s", err, out)
	}

	fmt.Printf("==> Creating VM: %s\n", name)
	fmt.Printf("    User: %s | vCPUs: %d | RAM: %dMB | Disk: %dGB\n", username, vcpus, ramMB, diskGB)
	fmt.Printf("    VM path:    %s\n", vmPath)
	fmt.Printf("    Shared dir: %s -> /mnt/host (inside VM)\n", dataDir)

	// Create disk image
	fmt.Println("==> Creating disk image...")
	if err := internal.QemuImgCreate(cfg.BaseImage, diskPath, diskGB); err != nil {
		return fmt.Errorf("creating disk image: %w", err)
	}

	// Create cloud-init seed ISO
	fmt.Println("==> Writing cloud-init config...")
	tmpDir, err := os.MkdirTemp("", "vmctl-cloudinit-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	if err := internal.GenerateSeedISO(tmpDir, isoPath, name, username, sshKey); err != nil {
		return fmt.Errorf("generating seed ISO: %w", err)
	}

	// Generate VM XML via virt-install --print-xml
	fmt.Println("==> Launching VM...")
	xmlContent, err := internal.VirtInstallPrintXML(name, vcpus, ramMB, diskPath, isoPath, cfg.Network, dataDir)
	if err != nil {
		return fmt.Errorf("generating VM XML: %w", err)
	}

	// Fix emulator path to system QEMU
	xmlContent = emulatorRegexp.ReplaceAllString(xmlContent, "<emulator>/usr/bin/qemu-system-x86_64</emulator>")

	// Write XML to temp file, define, and start
	xmlPath := filepath.Join(tmpDir, "vm.xml")
	if err := os.WriteFile(xmlPath, []byte(xmlContent), 0644); err != nil {
		return err
	}

	if err := internal.Define(xmlPath); err != nil {
		return fmt.Errorf("defining VM: %w", err)
	}
	if err := internal.Start(name); err != nil {
		return fmt.Errorf("starting VM: %w", err)
	}

	// Save metadata
	if err := internal.WriteVMInfo(vmPath, name, username); err != nil {
		return fmt.Errorf("writing VM info: %w", err)
	}

	hostname, _ := os.Hostname()
	fmt.Printf("==> VM '%s' created and booting...\n", name)
	fmt.Printf("    Shared dir: %s  <->  /mnt/host (inside VM)\n", dataDir)
	fmt.Printf("    Check IP:   virsh domifaddr %s\n", name)
	fmt.Printf("    Console:    virsh console %s  (Ctrl+] to exit)\n", name)
	fmt.Printf("    SSH access: ssh -J <host_user>@%s %s@<vm-ip>\n", hostname, username)
	fmt.Println()
	fmt.Printf("    NOTE: '%s' was added to the libvirt-qemu group.\n", hostUser)
	fmt.Println("          Log out and back in (or run 'newgrp libvirt-qemu') for it to take effect.")

	return nil
}
