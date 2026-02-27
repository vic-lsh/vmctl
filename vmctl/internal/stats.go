package internal

import (
	"fmt"
	"strconv"
	"strings"
)

// DomainStats holds resource stats for a single running VM.
type DomainStats struct {
	Name           string
	CPUTimeNs      uint64
	VCPUs          int
	BalloonRSS     uint64 // KiB
	BalloonMaximum uint64 // KiB
}

// DomStats queries virsh domstats for all running VMs and returns parsed results.
func DomStats() (map[string]*DomainStats, error) {
	out, err := runCmd("virsh", "domstats", "--cpu-total", "--balloon", "--vcpu", "--list-running")
	if err != nil {
		return nil, fmt.Errorf("virsh domstats: %w", err)
	}

	return parseDomStats(out), nil
}

func parseDomStats(output string) map[string]*DomainStats {
	result := make(map[string]*DomainStats)
	var current *DomainStats

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "Domain: '") {
			name := strings.TrimPrefix(line, "Domain: '")
			name = strings.TrimSuffix(name, "'")
			current = &DomainStats{Name: name}
			result[name] = current
			continue
		}

		if current == nil {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch key {
		case "cpu.time":
			current.CPUTimeNs, _ = strconv.ParseUint(val, 10, 64)
		case "vcpu.current":
			current.VCPUs, _ = strconv.Atoi(val)
		case "balloon.rss":
			current.BalloonRSS, _ = strconv.ParseUint(val, 10, 64)
		case "balloon.maximum":
			current.BalloonMaximum, _ = strconv.ParseUint(val, 10, 64)
		}
	}

	return result
}
