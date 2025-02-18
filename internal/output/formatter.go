package output

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/bkohler/procnetmon2/pkg/types"
	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
)

// Formatter handles output formatting
type Formatter struct {
	useJSON     bool
	useColor    bool
	showDetails bool
}

// Config holds formatter configuration
type Config struct {
	JSONOutput  bool
	UseColor    bool
	ShowDetails bool
}

// processStats represents JSON output for a single process
type processStats struct {
	PID         int32                           `json:"pid"`
	Name        string                          `json:"name"`
	Runtime     string                          `json:"runtime"`
	Current     *types.NetworkStats             `json:"current"`
	Peak        *types.NetworkStats             `json:"peak"`
	Total       *types.NetworkStats             `json:"total"`
	Connections map[string]types.ConnectionInfo `json:"connections,omitempty"`
}

// aggregatedStats represents JSON output for combined statistics
type aggregatedStats struct {
	BytesIn        uint64  `json:"bytes_in"`
	BytesOut       uint64  `json:"bytes_out"`
	RateIn         float64 `json:"rate_in"`
	RateOut        float64 `json:"rate_out"`
	TCPConnections uint32  `json:"tcp_connections"`
	UDPConnections uint32  `json:"udp_connections"`
}

// jsonOutput represents the complete JSON output structure
type jsonOutput struct {
	Timestamp  string                  `json:"timestamp"`
	Processes  map[string]processStats `json:"processes"`
	Aggregated *aggregatedStats        `json:"aggregated,omitempty"`
}

// New creates a new formatter
func New(cfg Config) *Formatter {
	return &Formatter{
		useJSON:     cfg.JSONOutput,
		useColor:    cfg.UseColor,
		showDetails: cfg.ShowDetails,
	}
}

// FormatStats formats process statistics
func (f *Formatter) FormatStats(stats map[int32]*types.ProcessStats) string {
	if f.useJSON {
		return f.formatJSON(stats)
	}
	return f.formatTable(stats)
}

// formatJSON converts statistics to JSON
func (f *Formatter) formatJSON(stats map[int32]*types.ProcessStats) string {
	output := jsonOutput{
		Timestamp: time.Now().Format(time.RFC3339),
		Processes: make(map[string]processStats),
	}

	totalIn, totalOut := uint64(0), uint64(0)
	totalRateIn, totalRateOut := float64(0), float64(0)
	totalTCP, totalUDP := uint32(0), uint32(0)

	for pid, procStats := range stats {
		current, peak, total := procStats.GetStats()

		// Create process stats
		pStats := processStats{
			PID:     pid,
			Name:    procStats.Comm,
			Runtime: time.Since(procStats.StartTime).Round(time.Second).String(),
			Current: &current,
			Peak:    &peak,
			Total:   &total,
		}

		// Add connections if details are requested
		if f.showDetails {
			pStats.Connections = current.ActiveConns
		}

		// Add to output
		output.Processes[fmt.Sprintf("%d", pid)] = pStats

		totalIn += current.BytesIn
		totalOut += current.BytesOut
		totalRateIn += current.CurrentRateIn
		totalRateOut += current.CurrentRateOut
		totalTCP += current.TCPConnections
		totalUDP += current.UDPConnections
	}

	output.Aggregated = &aggregatedStats{
		BytesIn:        totalIn,
		BytesOut:       totalOut,
		RateIn:         totalRateIn,
		RateOut:        totalRateOut,
		TCPConnections: totalTCP,
		UDPConnections: totalUDP,
	}

	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error formatting JSON: %v", err)
	}

	return string(jsonData)
}

// formatTable creates a human-readable table
func (f *Formatter) formatTable(stats map[int32]*types.ProcessStats) string {
	var sb strings.Builder

	// Create table
	table := tablewriter.NewWriter(&sb)
	table.SetHeader([]string{
		"PID",
		"Name",
		"Runtime",
		"Rate In",
		"Rate Out",
		"Total In",
		"Total Out",
		"TCP",
		"UDP",
	})
	table.SetBorder(true)
	table.SetRowLine(true)

	// Color setup
	var greenFn, yellowFn func(a ...interface{}) string
	if f.useColor {
		greenFn = color.New(color.FgGreen).SprintFunc()
		yellowFn = color.New(color.FgYellow).SprintFunc()
	} else {
		greenFn = fmt.Sprint
		yellowFn = fmt.Sprint
	}

	// Wrapper functions to handle single string argument
	green := func(s string) string { return greenFn(s) }
	yellow := func(s string) string { return yellowFn(s) }

	// Add process rows
	totalIn, totalOut := uint64(0), uint64(0)
	totalRateIn, totalRateOut := float64(0), float64(0)
	totalTCP, totalUDP := uint32(0), uint32(0)

	for pid, procStats := range stats {
		current, _, total := procStats.GetStats()

		table.Append([]string{
			fmt.Sprintf("%d", pid),
			procStats.Comm,
			time.Since(procStats.StartTime).Round(time.Second).String(),
			green(types.FormatRate(current.CurrentRateIn)),
			green(types.FormatRate(current.CurrentRateOut)),
			yellow(types.FormatBytes(total.BytesIn)),
			yellow(types.FormatBytes(total.BytesOut)),
			fmt.Sprintf("%d", current.TCPConnections),
			fmt.Sprintf("%d", current.UDPConnections),
		})

		totalIn += total.BytesIn
		totalOut += total.BytesOut
		totalRateIn += current.CurrentRateIn
		totalRateOut += current.CurrentRateOut
		totalTCP += current.TCPConnections
		totalUDP += current.UDPConnections
	}

	// Add totals row
	table.Append([]string{
		"",
		"TOTAL",
		"",
		green(types.FormatRate(totalRateIn)),
		green(types.FormatRate(totalRateOut)),
		yellow(types.FormatBytes(totalIn)),
		yellow(types.FormatBytes(totalOut)),
		fmt.Sprintf("%d", totalTCP),
		fmt.Sprintf("%d", totalUDP),
	})

	table.Render()

	// Add connection details if requested
	if f.showDetails {
		sb.WriteString("\nActive Connections:\n")
		for pid, procStats := range stats {
			current, _, _ := procStats.GetStats()
			if len(current.ActiveConns) > 0 {
				sb.WriteString(fmt.Sprintf("\nPID %d (%s):\n", pid, procStats.Comm))
				for _, conn := range current.ActiveConns {
					sb.WriteString(fmt.Sprintf("  %s: %s -> %s (%s)\n",
						conn.Protocol,
						conn.LocalAddr,
						conn.RemoteAddr,
						conn.State,
					))
				}
			}
		}
	}

	return sb.String()
}
