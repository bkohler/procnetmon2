package collector

import (
	"fmt"
	"sync"
	"time"

	"github.com/bkohler/procnetmon2/internal/bpf"
	"github.com/bkohler/procnetmon2/internal/process"
	"github.com/bkohler/procnetmon2/pkg/types"
)

// Collector handles network statistics collection and processing
type Collector struct {
	bpfMonitor *bpf.NetworkMonitor
	procMon    *process.Monitor
	config     Config

	// Rate calculation
	mu      sync.RWMutex
	samples map[int32][]sample
	stopped chan struct{}
}

// Config holds collector configuration
type Config struct {
	SampleInterval time.Duration
	WindowSize     int  // Number of samples to keep for rate calculation
	Continuous     bool // Whether to collect continuously
}

// sample represents a single statistics sample
type sample struct {
	timestamp time.Time
	stats     types.NetworkStats
}

// New creates a new statistics collector
func New(bpfMon *bpf.NetworkMonitor, procMon *process.Monitor, cfg Config) *Collector {
	if cfg.SampleInterval == 0 {
		cfg.SampleInterval = time.Second
	}
	if cfg.WindowSize == 0 {
		cfg.WindowSize = 10
	}

	return &Collector{
		bpfMonitor: bpfMon,
		procMon:    procMon,
		config:     cfg,
		samples:    make(map[int32][]sample),
		stopped:    make(chan struct{}),
	}
}

// Start begins statistics collection
func (c *Collector) Start() error {
	// Start collection goroutine
	go c.collect()
	return nil
}

// Stop stops statistics collection
func (c *Collector) Stop() {
	close(c.stopped)
}

// collect periodically fetches statistics from eBPF maps
func (c *Collector) collect() {
	ticker := time.NewTicker(c.config.SampleInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.updateStats()
		case <-c.stopped:
			return
		}
	}
}

// updateStats fetches current statistics and updates rates
func (c *Collector) updateStats() {
	pids := c.procMon.GetMonitoredPIDs()

	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()

	for _, pid := range pids {
		// Get current stats from eBPF
		stats, err := c.bpfMonitor.GetProcessStats(uint32(pid))
		if err != nil {
			continue // Skip this process if we can't get stats
		}

		// Get or initialize sample slice
		samples, exists := c.samples[pid]
		if !exists {
			samples = make([]sample, 0, c.config.WindowSize)
		}

		// Add new sample
		samples = append(samples, sample{
			timestamp: now,
			stats:     *stats,
		})

		// Keep only WindowSize most recent samples
		if len(samples) > c.config.WindowSize {
			samples = samples[1:]
		}

		// Calculate rates
		if len(samples) > 1 {
			latest := samples[len(samples)-1]
			previous := samples[len(samples)-2]
			duration := latest.timestamp.Sub(previous.timestamp).Seconds()

			if duration > 0 {
				stats.CurrentRateIn = float64(latest.stats.BytesIn-previous.stats.BytesIn) / duration
				stats.CurrentRateOut = float64(latest.stats.BytesOut-previous.stats.BytesOut) / duration
			}
		}

		// Update process statistics
		c.procMon.UpdateStats(pid, *stats)
		c.samples[pid] = samples
	}
}

// GetRates returns current transfer rates for a process
func (c *Collector) GetRates(pid int32) (in float64, out float64, err error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	samples, exists := c.samples[pid]
	if !exists || len(samples) < 2 {
		return 0, 0, fmt.Errorf("insufficient samples for PID %d", pid)
	}

	latest := samples[len(samples)-1]
	previous := samples[len(samples)-2]
	duration := latest.timestamp.Sub(previous.timestamp).Seconds()

	if duration > 0 {
		in = float64(latest.stats.BytesIn-previous.stats.BytesIn) / duration
		out = float64(latest.stats.BytesOut-previous.stats.BytesOut) / duration
	}

	return in, out, nil
}

// GetAggregatedStats returns combined statistics for all monitored processes
func (c *Collector) GetAggregatedStats() *types.NetworkStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	aggregated := &types.NetworkStats{
		ActiveConns: make(map[string]types.ConnectionInfo),
	}

	for pid := range c.samples {
		if stats, err := c.procMon.GetProcessStats(pid); err == nil {
			current, _, _ := stats.GetStats()
			aggregated.BytesIn += current.BytesIn
			aggregated.BytesOut += current.BytesOut
			aggregated.PacketsIn += current.PacketsIn
			aggregated.PacketsOut += current.PacketsOut
			aggregated.TCPConnections += current.TCPConnections
			aggregated.UDPConnections += current.UDPConnections

			// Merge connection maps
			for k, v := range current.ActiveConns {
				aggregated.ActiveConns[k] = v
			}
		}
	}

	return aggregated
}

// ClearStats removes all collected statistics
func (c *Collector) ClearStats() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.samples = make(map[int32][]sample)
	for _, pid := range c.procMon.GetMonitoredPIDs() {
		c.bpfMonitor.ClearProcessStats(uint32(pid))
	}
}
