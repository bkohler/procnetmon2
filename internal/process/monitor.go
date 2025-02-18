package process

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bkohler/procnetmon2/pkg/types"
)

// Monitor handles process monitoring and validation
type Monitor struct {
	mu      sync.RWMutex
	pids    map[int32]*types.ProcessStats
	stopped chan struct{}
}

// New creates a new process monitor
func New() *Monitor {
	return &Monitor{
		pids:    make(map[int32]*types.ProcessStats),
		stopped: make(chan struct{}),
	}
}

// AddProcess adds a process to be monitored
func (m *Monitor) AddProcess(pidStr string) error {
	pid, err := strconv.ParseInt(pidStr, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid PID format: %w", err)
	}

	// Validate process exists and we have permissions
	comm, err := m.getProcessName(int32(pid))
	if err != nil {
		return fmt.Errorf("failed to access process %d: %w", pid, err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Add to monitoring
	m.pids[int32(pid)] = types.NewProcessStats(int32(pid), comm)
	return nil
}

// RemoveProcess stops monitoring a process
func (m *Monitor) RemoveProcess(pid int32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.pids, pid)
}

// GetMonitoredPIDs returns a list of currently monitored PIDs
func (m *Monitor) GetMonitoredPIDs() []int32 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pids := make([]int32, 0, len(m.pids))
	for pid := range m.pids {
		pids = append(pids, pid)
	}
	return pids
}

// GetProcessStats returns statistics for a specific PID
func (m *Monitor) GetProcessStats(pid int32) (*types.ProcessStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats, exists := m.pids[pid]
	if !exists {
		return nil, fmt.Errorf("process %d not monitored", pid)
	}
	return stats, nil
}

// Start begins process monitoring
func (m *Monitor) Start() {
	go m.monitor()
}

// Stop stops process monitoring
func (m *Monitor) Stop() {
	close(m.stopped)
}

// monitor periodically checks process existence and updates metadata
func (m *Monitor) monitor() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.checkProcesses()
		case <-m.stopped:
			return
		}
	}
}

// checkProcesses verifies monitored processes still exist
func (m *Monitor) checkProcesses() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for pid := range m.pids {
		if _, err := m.getProcessName(pid); err != nil {
			// Process no longer exists or accessible
			delete(m.pids, pid)
		}
	}
}

// getProcessName reads the process name from /proc/[pid]/comm
func (m *Monitor) getProcessName(pid int32) (string, error) {
	commPath := filepath.Join("/proc", strconv.FormatInt(int64(pid), 10), "comm")
	data, err := os.ReadFile(commPath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// ValidatePermissions checks if we have necessary permissions to monitor a process
func (m *Monitor) ValidatePermissions(pid int32) error {
	// Check if process exists
	if _, err := m.getProcessName(pid); err != nil {
		return fmt.Errorf("process does not exist or permission denied: %w", err)
	}

	// Check if we can read process information
	procPath := filepath.Join("/proc", strconv.FormatInt(int64(pid), 10))
	if _, err := os.Stat(procPath); err != nil {
		return fmt.Errorf("cannot access process information: %w", err)
	}

	return nil
}

// UpdateStats updates the statistics for a process
func (m *Monitor) UpdateStats(pid int32, stats types.NetworkStats) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	procStats, exists := m.pids[pid]
	if !exists {
		return fmt.Errorf("process %d not monitored", pid)
	}

	procStats.Update(stats)
	return nil
}

// GetAllStats returns statistics for all monitored processes
func (m *Monitor) GetAllStats() map[int32]*types.ProcessStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Create a copy to avoid external modifications
	stats := make(map[int32]*types.ProcessStats, len(m.pids))
	for pid, procStats := range m.pids {
		stats[pid] = procStats
	}
	return stats
}
