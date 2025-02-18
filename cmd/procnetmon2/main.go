package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bkohler/procnetmon2/internal/bpf"
	"github.com/bkohler/procnetmon2/internal/collector"
	"github.com/bkohler/procnetmon2/internal/output"
	"github.com/bkohler/procnetmon2/internal/process"
	"github.com/spf13/cobra"
)

var (
	// CLI flags
	pids        []string
	interface_  string
	jsonOutput  bool
	sampleTime  string
	aggregate   bool
	continuous  bool
	showDetails bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "procnetmon2",
		Short: "Process-specific network monitoring tool using eBPF",
		Long: `ProcNetMon2 is a high-performance network monitoring tool that uses eBPF 
to track process-specific network statistics in real-time. It provides detailed 
insights into network usage with minimal system overhead.`,
		RunE: run,
	}

	// Add flags
	rootCmd.Flags().StringSliceVarP(&pids, "pids", "p", []string{}, "Comma-separated list of process IDs to monitor")
	rootCmd.Flags().StringVarP(&interface_, "interface", "i", "", "Network interface to monitor (default: all)")
	rootCmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output in JSON format")
	rootCmd.Flags().StringVarP(&sampleTime, "time", "t", "", "Time-based sampling period (e.g., 60s, 5m)")
	rootCmd.Flags().BoolVarP(&aggregate, "aggregate", "a", false, "Aggregate statistics across monitored processes")
	rootCmd.Flags().BoolVarP(&continuous, "continuous", "c", true, "Enable continuous monitoring")
	rootCmd.Flags().BoolVarP(&showDetails, "details", "d", false, "Show detailed connection information")

	// Make pids required
	rootCmd.MarkFlagRequired("pids")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	// Parse sampling duration
	var samplingDuration time.Duration
	if sampleTime != "" {
		var err error
		samplingDuration, err = time.ParseDuration(sampleTime)
		if err != nil {
			return fmt.Errorf("invalid sampling time format: %w", err)
		}
	}

	// Initialize process monitor
	procMon := process.New()

	// Add processes to monitor
	for _, pidStr := range pids {
		if err := procMon.AddProcess(pidStr); err != nil {
			return fmt.Errorf("failed to add process %s: %w", pidStr, err)
		}
	}

	// Initialize eBPF monitor
	bpfMon, err := bpf.New(bpf.Config{
		Interface: interface_,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize eBPF monitor: %w", err)
	}
	defer bpfMon.Stop()

	// Start eBPF monitoring
	if err := bpfMon.Start(); err != nil {
		return fmt.Errorf("failed to start eBPF monitor: %w", err)
	}

	// Initialize statistics collector
	collector := collector.New(bpfMon, procMon, collector.Config{
		SampleInterval: time.Second,
		WindowSize:     10,
		Continuous:     continuous,
	})

	// Start collection
	if err := collector.Start(); err != nil {
		return fmt.Errorf("failed to start collector: %w", err)
	}
	defer collector.Stop()

	// Initialize output formatter
	formatter := output.New(output.Config{
		JSONOutput:  jsonOutput,
		UseColor:    !jsonOutput && os.Stdout.Fd() == 1, // Use color if not JSON and stdout is a terminal
		ShowDetails: showDetails,
	})

	// Setup signal handling for clean shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Clear screen and hide cursor
	fmt.Print("\033[2J\033[H\033[?25l")
	defer fmt.Print("\033[?25h") // Show cursor on exit

	// Start monitoring loop
	fmt.Printf("Starting network monitoring...\n")
	if interface_ != "" {
		fmt.Printf("Filtering on interface: %s\n", interface_)
	}
	fmt.Printf("Monitoring PIDs: %s\n\n", strings.Join(pids, ", "))

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	start := time.Now()
	for {
		select {
		case <-ticker.C:
			// Clear screen and move cursor to top
			fmt.Print("\033[H")

			// Get and format statistics
			stats := procMon.GetAllStats()
			fmt.Print(formatter.FormatStats(stats))

			// Check sampling duration
			if samplingDuration > 0 && time.Since(start) >= samplingDuration {
				return nil
			}

		case <-sigChan:
			fmt.Print("\033[2J\033[H") // Clear screen
			return nil
		}
	}
}
