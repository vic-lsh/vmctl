package cmd

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
	"vmctl/internal"

	"github.com/spf13/cobra"
)

var sshCmd = &cobra.Command{
	Use:                "ssh <vm-path> [-- ssh-args...]",
	Short:              "SSH into a VM by its path",
	Args:               cobra.MinimumNArgs(1),
	DisableFlagParsing: true,
	RunE:               runSSH,
}

func runSSH(cmd *cobra.Command, args []string) error {
	vmPath, err := filepath.Abs(args[0])
	if err != nil {
		return err
	}

	// Remaining args are passed to ssh
	var sshArgs []string
	if len(args) > 1 {
		sshArgs = args[1:]
	}

	name, username, err := internal.ReadVMInfo(vmPath)
	if err != nil {
		return err
	}

	state, err := internal.DomState(name)
	if err != nil {
		return fmt.Errorf("VM '%s' not found", name)
	}
	if state != "running" {
		return fmt.Errorf("VM '%s' is not running (state: %s)", name, state)
	}

	fmt.Printf("==> Looking up IP for %s...\n", name)
	var ip string
	for i := 1; i <= 10; i++ {
		ip, _ = internal.DomIfAddr(name)
		if ip != "" {
			break
		}
		fmt.Printf("    Waiting for IP... (%d/10)\n", i)
		time.Sleep(3 * time.Second)
	}

	if ip == "" {
		return fmt.Errorf("could not get IP for '%s'. Try again once it finishes booting", name)
	}

	// Wait for SSH port to be reachable
	fmt.Printf("==> Waiting for SSH on %s...\n", ip)
	sshReady := false
	for i := 1; i <= 20; i++ {
		conn, err := net.DialTimeout("tcp", ip+":22", 2*time.Second)
		if err == nil {
			conn.Close()
			sshReady = true
			break
		}
		fmt.Printf("    Waiting for SSH... (%d/20)\n", i)
		time.Sleep(3 * time.Second)
	}
	if !sshReady {
		return fmt.Errorf("SSH not reachable on %s:22. The VM may still be booting — try again shortly", ip)
	}

	fmt.Printf("==> Connecting to %s@%s\n", username, ip)

	// Build ssh command args
	sshBin, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("ssh not found in PATH")
	}

	execArgs := []string{"ssh", "-o", "StrictHostKeyChecking=no", fmt.Sprintf("%s@%s", username, ip)}
	execArgs = append(execArgs, sshArgs...)

	// Replace process with ssh (like bash exec)
	return syscall.Exec(sshBin, execArgs, os.Environ())
}
