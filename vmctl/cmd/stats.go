package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"sort"
	"text/tabwriter"
	"time"
	"vmctl/internal"

	"github.com/spf13/cobra"
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show live resource usage for running VMs",
	Args:  cobra.NoArgs,
	RunE:  runStats,
}

var (
	statsInterval time.Duration
	statsNoStream bool
)

func init() {
	statsCmd.Flags().DurationVarP(&statsInterval, "interval", "n", 3*time.Second, "refresh interval")
	statsCmd.Flags().BoolVar(&statsNoStream, "no-stream", false, "print once and exit")
	rootCmd.AddCommand(statsCmd)
}

type vmDisplay struct {
	Name     string
	CPUPct   float64
	MemUsage string
	MemPct   float64
	VCPUs    int
}

func runStats(cmd *cobra.Command, args []string) error {
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
	defer stop()

	prev, err := internal.DomStats()
	if err != nil {
		return err
	}
	prevTime := time.Now()

	if statsNoStream {
		// With no-stream, just show current memory/vcpu (no CPU % on first sample)
		vms := computeDisplay(nil, prev, 0)
		printTable(vms)
		return nil
	}

	// Wait for first interval to compute CPU %
	select {
	case <-time.After(statsInterval):
	case <-ctx.Done():
		return nil
	}

	for {
		curr, err := internal.DomStats()
		if err != nil {
			return err
		}
		now := time.Now()
		elapsed := now.Sub(prevTime)

		vms := computeDisplay(prev, curr, elapsed)
		clearScreen()
		printTable(vms)

		prev = curr
		prevTime = now

		select {
		case <-time.After(statsInterval):
		case <-ctx.Done():
			return nil
		}
	}
}

func computeDisplay(prev, curr map[string]*internal.DomainStats, elapsed time.Duration) []vmDisplay {
	var vms []vmDisplay

	for name, c := range curr {
		vm := vmDisplay{
			Name:  name,
			VCPUs: c.VCPUs,
		}

		// Memory
		if c.BalloonMaximum > 0 {
			vm.MemUsage = fmt.Sprintf("%s / %s", formatKiB(c.BalloonRSS), formatKiB(c.BalloonMaximum))
			vm.MemPct = float64(c.BalloonRSS) / float64(c.BalloonMaximum) * 100
		} else {
			vm.MemUsage = fmt.Sprintf("%s / --", formatKiB(c.BalloonRSS))
		}

		// CPU %
		if prev != nil && elapsed > 0 {
			if p, ok := prev[name]; ok {
				deltaCPU := float64(c.CPUTimeNs - p.CPUTimeNs)
				deltaWall := float64(elapsed.Nanoseconds())
				vm.CPUPct = deltaCPU / deltaWall * 100
			}
		}

		vms = append(vms, vm)
	}

	sort.Slice(vms, func(i, j int) bool {
		return vms[i].Name < vms[j].Name
	})

	return vms
}

func printTable(vms []vmDisplay) {
	if len(vms) == 0 {
		fmt.Println("No running VMs.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tCPU %\tMEM USAGE / LIMIT\tMEM %\tVCPUS")
	for _, vm := range vms {
		fmt.Fprintf(w, "%s\t%.2f%%\t%s\t%.2f%%\t%d\n",
			vm.Name, vm.CPUPct, vm.MemUsage, vm.MemPct, vm.VCPUs)
	}
	w.Flush()
}

func formatKiB(kib uint64) string {
	mib := float64(kib) / 1024
	if mib >= 1024 {
		return fmt.Sprintf("%.2fGiB", mib/1024)
	}
	return fmt.Sprintf("%.2fMiB", mib)
}

func clearScreen() {
	fmt.Print("\033[H\033[2J")
}
