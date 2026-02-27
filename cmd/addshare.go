package cmd

import (
	"fmt"
	"strings"
	"time"
	"vmctl/internal"

	"github.com/spf13/cobra"
)

var addShareCmd = &cobra.Command{
	Use:   "add-share",
	Short: "Add a host directory mount to an existing VM (requires restart)",
	Example: `  vmctl add-share -n alice-vm -s /mnt/nvme1/alice
  vmctl add-share -n alice-vm -s /mnt/nvme1/alice -i 192.168.122.10 -u ubuntu`,
	RunE: runAddShare,
}

func init() {
	f := addShareCmd.Flags()
	f.StringP("name", "n", "", "VM name (required)")
	f.StringP("share", "s", "", "Host directory to share (required)")
	f.StringP("ip", "i", "", "VM IP (to auto-configure /etc/fstab inside VM)")
	f.StringP("user", "u", "", "VM username for SSH (default: from config)")
	f.StringP("tag", "t", "hostshare", "Mount tag name, visible inside VM")
	addShareCmd.MarkFlagRequired("name")
	addShareCmd.MarkFlagRequired("share")
}

func runAddShare(cmd *cobra.Command, args []string) error {
	cfg := GetConfig()

	name, _ := cmd.Flags().GetString("name")
	shareDir, _ := cmd.Flags().GetString("share")
	vmIP, _ := cmd.Flags().GetString("ip")
	username, _ := cmd.Flags().GetString("user")
	tag, _ := cmd.Flags().GetString("tag")

	if username == "" {
		username = cfg.Defaults.User
	}

	// Check VM exists
	if err := internal.DomInfo(name); err != nil {
		return fmt.Errorf("VM '%s' not found", name)
	}

	// Check tag not already in use
	xml, err := internal.DumpXML(name)
	if err != nil {
		return fmt.Errorf("dumping VM XML: %w", err)
	}
	if strings.Contains(xml, fmt.Sprintf("target dir='%s'", tag)) {
		return fmt.Errorf("mount tag '%s' is already attached to VM '%s'. Use -t to specify a different tag name", tag, name)
	}

	// Try to grab IP before shutdown if not provided
	if vmIP == "" {
		vmIP, _ = internal.DomIfAddr(name)
	}

	// Shut down VM if running
	state, err := internal.DomState(name)
	if err != nil {
		return err
	}
	if state != "shut off" {
		fmt.Printf("==> Shutting down VM: %s...\n", name)
		if err := internal.Shutdown(name); err != nil {
			return err
		}

		shutDown := false
		for i := 1; i <= 30; i++ {
			time.Sleep(2 * time.Second)
			s, _ := internal.DomState(name)
			if s == "shut off" {
				shutDown = true
				break
			}
		}
		if !shutDown {
			fmt.Println("    Graceful shutdown timed out, forcing off...")
			internal.Destroy(name)
			time.Sleep(2 * time.Second)
		}
	}

	// Attach filesystem device
	fmt.Printf("==> Attaching filesystem: %s (tag: %s)...\n", shareDir, tag)
	deviceXML := fmt.Sprintf(`<filesystem type='mount' accessmode='passthrough'>
  <source dir='%s'/>
  <target dir='%s'/>
</filesystem>`, shareDir, tag)

	if err := internal.AttachDevice(name, deviceXML); err != nil {
		return fmt.Errorf("attaching device: %w", err)
	}

	// Start VM
	fmt.Printf("==> Starting VM: %s...\n", name)
	if err := internal.Start(name); err != nil {
		return fmt.Errorf("starting VM: %w", err)
	}

	// If we have an IP, SSH in and configure fstab
	if vmIP != "" {
		fmt.Println("==> Waiting for VM to boot (30s)...")
		time.Sleep(30 * time.Second)

		fmt.Println("==> Configuring /etc/fstab inside VM...")
		sshCommand := fmt.Sprintf(
			"sudo mkdir -p /mnt/host && "+
				"grep -q '%s' /etc/fstab || echo '%s /mnt/host 9p trans=virtio,version=9p2000.L,rw,_netdev,nofail 0 0' | sudo tee -a /etc/fstab && "+
				"sudo mount /mnt/host && "+
				"echo 'Mounted at /mnt/host'",
			tag, tag,
		)
		if err := internal.SSHRun(username, vmIP, sshCommand); err != nil {
			return fmt.Errorf("configuring fstab: %w", err)
		}
		fmt.Println("==> Done.")
	} else {
		fmt.Println("==> VM started. To finish setup inside the VM, run:")
		fmt.Println("    sudo mkdir -p /mnt/host")
		fmt.Printf("    sudo mount -t 9p -o trans=virtio,version=9p2000.L %s /mnt/host\n", tag)
		fmt.Println("    # For persistence across VM reboots, add to /etc/fstab:")
		fmt.Printf("    echo '%s /mnt/host 9p trans=virtio,version=9p2000.L,rw,_netdev,nofail 0 0' | sudo tee -a /etc/fstab\n", tag)
	}

	return nil
}
