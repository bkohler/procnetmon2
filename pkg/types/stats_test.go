package types

import (
	"testing"
	"time"
)

func TestNewProcessStats(t *testing.T) {
	pid := int32(1234)
	comm := "test-process"

	stats := NewProcessStats(pid, comm)

	if stats.PID != pid {
		t.Errorf("Expected PID %d, got %d", pid, stats.PID)
	}
	if stats.Comm != comm {
		t.Errorf("Expected Comm %s, got %s", comm, stats.Comm)
	}
	if stats.Current.ActiveConns == nil {
		t.Error("Expected non-nil ActiveConns map in Current")
	}
	if stats.Peak.ActiveConns == nil {
		t.Error("Expected non-nil ActiveConns map in Peak")
	}
	if stats.Total.ActiveConns == nil {
		t.Error("Expected non-nil ActiveConns map in Total")
	}
}

func TestProcessStatsUpdate(t *testing.T) {
	stats := NewProcessStats(1234, "test-process")

	// Create test network stats
	update := NetworkStats{
		BytesIn:        1000,
		BytesOut:       2000,
		PacketsIn:      10,
		PacketsOut:     20,
		CurrentRateIn:  100.0,
		CurrentRateOut: 200.0,
		TCPConnections: 2,
		UDPConnections: 1,
		ActiveConns:    make(map[string]ConnectionInfo),
	}

	// First update
	stats.Update(update)
	current, peak, total := stats.GetStats()

	// Check current stats
	if current.BytesIn != update.BytesIn {
		t.Errorf("Expected current BytesIn %d, got %d", update.BytesIn, current.BytesIn)
	}
	if current.BytesOut != update.BytesOut {
		t.Errorf("Expected current BytesOut %d, got %d", update.BytesOut, current.BytesOut)
	}

	// Check peak rates
	if peak.PeakRateIn != update.CurrentRateIn {
		t.Errorf("Expected peak RateIn %.2f, got %.2f", update.CurrentRateIn, peak.PeakRateIn)
	}
	if peak.PeakRateOut != update.CurrentRateOut {
		t.Errorf("Expected peak RateOut %.2f, got %.2f", update.CurrentRateOut, peak.PeakRateOut)
	}

	// Check totals
	if total.BytesIn != update.BytesIn {
		t.Errorf("Expected total BytesIn %d, got %d", update.BytesIn, total.BytesIn)
	}
	if total.BytesOut != update.BytesOut {
		t.Errorf("Expected total BytesOut %d, got %d", update.BytesOut, total.BytesOut)
	}

	// Test peak rate updates
	higherRates := NetworkStats{
		BytesIn:        2000,
		BytesOut:       4000,
		PacketsIn:      20,
		PacketsOut:     40,
		CurrentRateIn:  150.0, // Higher than previous
		CurrentRateOut: 250.0, // Higher than previous
		TCPConnections: 2,
		UDPConnections: 1,
		ActiveConns:    make(map[string]ConnectionInfo),
	}

	stats.Update(higherRates)
	_, peak, total = stats.GetStats()

	// Check new peak rates
	if peak.PeakRateIn != higherRates.CurrentRateIn {
		t.Errorf("Expected peak RateIn %.2f, got %.2f", higherRates.CurrentRateIn, peak.PeakRateIn)
	}
	if peak.PeakRateOut != higherRates.CurrentRateOut {
		t.Errorf("Expected peak RateOut %.2f, got %.2f", higherRates.CurrentRateOut, peak.PeakRateOut)
	}

	// Check accumulated totals
	expectedTotalIn := update.BytesIn + higherRates.BytesIn
	expectedTotalOut := update.BytesOut + higherRates.BytesOut
	if total.BytesIn != expectedTotalIn {
		t.Errorf("Expected accumulated total BytesIn %d, got %d", expectedTotalIn, total.BytesIn)
	}
	if total.BytesOut != expectedTotalOut {
		t.Errorf("Expected accumulated total BytesOut %d, got %d", expectedTotalOut, total.BytesOut)
	}
}

func TestFormatRate(t *testing.T) {
	const (
		_  = iota
		KB = 1 << (10 * iota)
		MB
		GB
	)

	tests := []struct {
		bytesPerSec float64
		expected    string
	}{
		{500, "4000.00 bps"},       // 500 B/s * 8 = 4000 bps
		{KB * 128, "1024.00 Kbps"}, // 128 KiB/s = 1024 Kbps
		{MB, "8.00 Mbps"},          // 1 MiB/s = 8 Mbps
		{MB * 128, "1024.00 Mbps"}, // 128 MiB/s = 1024 Mbps
	}

	for _, test := range tests {
		result := FormatRate(test.bytesPerSec)
		if result != test.expected {
			t.Errorf("FormatRate(%.2f) = %s; expected %s", test.bytesPerSec, result, test.expected)
		}
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    uint64
		expected string
	}{
		{500, "500 B"},
		{1500, "1.46 KB"},
		{1500000, "1.43 MB"},
		{1500000000, "1.40 GB"},
	}

	for _, test := range tests {
		result := FormatBytes(test.bytes)
		if result != test.expected {
			t.Errorf("FormatBytes(%d) = %s; expected %s", test.bytes, result, test.expected)
		}
	}
}

func TestConnectionInfo(t *testing.T) {
	now := time.Now()
	conn := ConnectionInfo{
		Protocol:    "tcp",
		LocalAddr:   "127.0.0.1:8080",
		RemoteAddr:  "192.168.1.100:45123",
		State:       "ESTABLISHED",
		LastUpdated: now,
	}

	stats := NewProcessStats(1234, "test-process")
	current := NetworkStats{
		ActiveConns: map[string]ConnectionInfo{
			"conn1": conn,
		},
	}

	stats.Update(current)
	currentStats, _, _ := stats.GetStats()

	if len(currentStats.ActiveConns) != 1 {
		t.Errorf("Expected 1 active connection, got %d", len(currentStats.ActiveConns))
	}

	savedConn, exists := currentStats.ActiveConns["conn1"]
	if !exists {
		t.Error("Expected connection 'conn1' to exist")
	}

	if savedConn.Protocol != conn.Protocol {
		t.Errorf("Expected protocol %s, got %s", conn.Protocol, savedConn.Protocol)
	}
	if savedConn.LocalAddr != conn.LocalAddr {
		t.Errorf("Expected local addr %s, got %s", conn.LocalAddr, savedConn.LocalAddr)
	}
	if savedConn.RemoteAddr != conn.RemoteAddr {
		t.Errorf("Expected remote addr %s, got %s", conn.RemoteAddr, savedConn.RemoteAddr)
	}
	if savedConn.State != conn.State {
		t.Errorf("Expected state %s, got %s", conn.State, savedConn.State)
	}
	if !savedConn.LastUpdated.Equal(conn.LastUpdated) {
		t.Errorf("Expected last updated %v, got %v", conn.LastUpdated, savedConn.LastUpdated)
	}
}
