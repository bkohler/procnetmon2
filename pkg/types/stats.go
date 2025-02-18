package types

import (
	"fmt"
	"sync"
	"time"
)

// ProcessStats holds network statistics for a single process
type ProcessStats struct {
	PID       int32
	Comm      string    // Process name
	StartTime time.Time // Monitoring start time

	// Network statistics with mutex protection
	mu      sync.RWMutex
	Current NetworkStats
	Peak    NetworkStats
	Total   NetworkStats
}

// NetworkStats holds various network metrics
type NetworkStats struct {
	BytesIn        uint64
	BytesOut       uint64
	PacketsIn      uint64
	PacketsOut     uint64
	CurrentRateIn  float64 // bytes per second
	CurrentRateOut float64
	PeakRateIn     float64
	PeakRateOut    float64
	TCPConnections uint32
	UDPConnections uint32
	ActiveConns    map[string]ConnectionInfo // key: "srcIP:srcPort-dstIP:dstPort"
}

// ConnectionInfo represents an active network connection
type ConnectionInfo struct {
	Protocol    string // "tcp" or "udp"
	LocalAddr   string // "ip:port"
	RemoteAddr  string // "ip:port"
	State       string // TCP state (if applicable)
	LastUpdated time.Time
}

// NewProcessStats creates a new ProcessStats instance
func NewProcessStats(pid int32, comm string) *ProcessStats {
	return &ProcessStats{
		PID:       pid,
		Comm:      comm,
		StartTime: time.Now(),
		Current: NetworkStats{
			ActiveConns: make(map[string]ConnectionInfo),
		},
		Peak: NetworkStats{
			ActiveConns: make(map[string]ConnectionInfo),
		},
		Total: NetworkStats{
			ActiveConns: make(map[string]ConnectionInfo),
		},
	}
}

// Update atomically updates current statistics and updates peaks if necessary
func (ps *ProcessStats) Update(stats NetworkStats) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	ps.Current = stats

	// Update peak rates
	if stats.CurrentRateIn > ps.Peak.PeakRateIn {
		ps.Peak.PeakRateIn = stats.CurrentRateIn
	}
	if stats.CurrentRateOut > ps.Peak.PeakRateOut {
		ps.Peak.PeakRateOut = stats.CurrentRateOut
	}

	// Update totals
	ps.Total.BytesIn += stats.BytesIn
	ps.Total.BytesOut += stats.BytesOut
	ps.Total.PacketsIn += stats.PacketsIn
	ps.Total.PacketsOut += stats.PacketsOut
}

// GetStats safely retrieves current statistics
func (ps *ProcessStats) GetStats() (NetworkStats, NetworkStats, NetworkStats) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.Current, ps.Peak, ps.Total
}

// FormatRate converts bytes per second to a human-readable string
func FormatRate(bytesPerSec float64) string {
	const (
		_  = iota
		KB = 1 << (10 * iota) // 1024
		MB                    // 1048576
		GB                    // 1073741824
	)

	// Convert to bits per second
	bitsPerSec := bytesPerSec * 8

	switch {
	case bytesPerSec >= GB:
		return fmt.Sprintf("%.2f Gbps", bitsPerSec/(GB*8))
	case bytesPerSec >= MB:
		return fmt.Sprintf("%.2f Mbps", bitsPerSec/(MB*8))
	case bytesPerSec >= KB:
		return fmt.Sprintf("%.2f Kbps", bitsPerSec/(KB*8))
	default:
		return fmt.Sprintf("%.2f bps", bitsPerSec)
	}
}

// FormatBytes converts bytes to a human-readable string
func FormatBytes(bytes uint64) string {
	const (
		_  = iota
		KB = 1 << (10 * iota)
		MB
		GB
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
