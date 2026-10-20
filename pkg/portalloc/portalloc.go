// Package portalloc assigns local HTTP ports for workspace apps.
package portalloc

import "sort"

// WellKnownReserved are ports CoGS avoids when auto-assigning.
var WellKnownReserved = []int{
	80, 443, 3000, 3306, 5000, 5432, 6379, 8000, 8080, 8443, 8888, 9000, 27017,
}

// Config drives sequential port assignment.
type Config struct {
	BasePort int
	PortStep int
	Reserved map[int]struct{}
}

// DefaultConfig avoids 8080 and other common dev ports.
func DefaultConfig() Config {
	r := map[int]struct{}{}
	for _, p := range WellKnownReserved {
		r[p] = struct{}{}
	}
	return Config{BasePort: 18787, PortStep: 1, Reserved: r}
}

// ForIndex returns the port for app index (0-based), skipping reserved and used.
func (c Config) ForIndex(index int, used map[int]struct{}) int {
	if c.BasePort <= 0 {
		c.BasePort = 18787
	}
	if c.PortStep <= 0 {
		c.PortStep = 1
	}
	if c.Reserved == nil {
		c.Reserved = DefaultConfig().Reserved
	}
	start := c.BasePort + index*c.PortStep
	for p := start; p < 65535; p += c.PortStep {
		if _, bad := c.Reserved[p]; bad {
			continue
		}
		if used != nil {
			if _, taken := used[p]; taken {
				continue
			}
		}
		return p
	}
	return start
}

// CollectUsed reads ports from a list of manifest ports already set (>0).
func CollectUsed(ports []int) map[int]struct{} {
	out := map[int]struct{}{}
	for _, p := range ports {
		if p > 0 {
			out[p] = struct{}{}
		}
	}
	return out
}

// MergeReserved adds extra reserved ports to a set.
func MergeReserved(base map[int]struct{}, extra []int) map[int]struct{} {
	out := map[int]struct{}{}
	for p := range base {
		out[p] = struct{}{}
	}
	for _, p := range extra {
		out[p] = struct{}{}
	}
	return out
}

// Sorted returns sorted port list (for stable tests / display).
func Sorted(ports []int) []int {
	cp := append([]int(nil), ports...)
	sort.Ints(cp)
	return cp
}
