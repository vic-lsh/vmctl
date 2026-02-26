package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"vmctl/internal"

	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete <vm-name> <vm-path>",
	Short: "Delete a VM and its associated files",
	Long:  "Stop and undefine a VM, remove its disk image and seed ISO. The data/ directory is preserved.",
	Args:  cobra.ExactArgs(2),
	RunE:  runDelete,
}

func runDelete(cmd *cobra.Command, args []string) error {
	name := args[0]
	vmPath := args[1]

	if err := internal.DomInfo(name); err != nil {
		return fmt.Errorf("VM '%s' not found", name)
	}

	fmt.Printf("==> Stopping VM: %s\n", name)
	internal.Destroy(name)

	fmt.Println("==> Undefining VM...")
	internal.Undefine(name)

	diskPath := filepath.Join(vmPath, name+".qcow2")
	isoPath := filepath.Join(vmPath, name+"-seed.iso")

	if _, err := os.Stat(diskPath); err == nil {
		os.Remove(diskPath)
		fmt.Printf("    Removed: %s\n", diskPath)
	}
	if _, err := os.Stat(isoPath); err == nil {
		os.Remove(isoPath)
		fmt.Printf("    Removed: %s\n", isoPath)
	}

	dataDir := filepath.Join(vmPath, "data")
	if info, err := os.Stat(dataDir); err == nil && info.IsDir() {
		fmt.Printf("    Preserved: %s  (remove manually if no longer needed)\n", dataDir)
	}

	fmt.Printf("==> VM '%s' deleted.\n", name)
	return nil
}
