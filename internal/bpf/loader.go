package bpf

import (
	"fmt"
	"net"

	"github.com/bkohler/procnetmon2/pkg/types"
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/rlimit"
	"github.com/vishvananda/netlink"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang netmon ./c/netmon.c -- -I/usr/include/bpf

// NetworkMonitor represents the eBPF program and its resources
type NetworkMonitor struct {
	programs    *netmonPrograms
	maps        *netmonMaps
	tcIngress   link.Link
	tcEgress    link.Link
	interfaceID uint32
}

// Config holds configuration for the network monitor
type Config struct {
	Interface string // Interface to monitor (empty for all)
}

// New creates a new NetworkMonitor instance
func New(cfg Config) (*NetworkMonitor, error) {
	// Allow the current process to lock memory for eBPF resources
	if err := rlimit.RemoveMemlock(); err != nil {
		return nil, fmt.Errorf("failed to remove memlock limit: %w", err)
	}

	// Load pre-compiled programs
	objs := netmonObjects{}
	if err := loadNetmonObjects(&objs, nil); err != nil {
		return nil, fmt.Errorf("failed to load objects: %w", err)
	}

	nm := &NetworkMonitor{
		programs: &objs.netmonPrograms,
		maps:     &objs.netmonMaps,
	}

	// Get network interface if specified
	if cfg.Interface != "" {
		iface, err := net.InterfaceByName(cfg.Interface)
		if err != nil {
			objs.Close()
			return nil, fmt.Errorf("failed to find interface %s: %w", cfg.Interface, err)
		}
		nm.interfaceID = uint32(iface.Index)

		// Enable monitoring for this interface
		if err := nm.maps.InterfaceFilter.Put(nm.interfaceID, uint8(1)); err != nil {
			objs.Close()
			return nil, fmt.Errorf("failed to update interface filter: %w", err)
		}
	}

	return nm, nil
}

// Start attaches the eBPF programs to the network interface
func (nm *NetworkMonitor) Start() error {
	// Attach TC programs
	if nm.interfaceID != 0 {
		// Get interface
		link, err := netlink.LinkByIndex(int(nm.interfaceID))
		if err != nil {
			return fmt.Errorf("failed to get interface: %w", err)
		}

		// Remove existing qdisc if it exists
		qdiscs, err := netlink.QdiscList(link)
		if err != nil {
			return fmt.Errorf("failed to list qdiscs: %w", err)
		}

		for _, qdisc := range qdiscs {
			if qdisc.Type() == "clsact" {
				if err := netlink.QdiscDel(qdisc); err != nil {
					return fmt.Errorf("failed to remove existing qdisc: %w", err)
				}
				break
			}
		}

		// Add qdisc
		qdisc := &netlink.GenericQdisc{
			QdiscAttrs: netlink.QdiscAttrs{
				LinkIndex: link.Attrs().Index,
				Handle:    netlink.MakeHandle(0xffff, 0),
				Parent:    netlink.HANDLE_CLSACT,
			},
			QdiscType: "clsact",
		}

		if err := netlink.QdiscAdd(qdisc); err != nil {
			return fmt.Errorf("failed to add qdisc: %w", err)
		}

		// Add ingress filter
		filterIngress := &netlink.BpfFilter{
			FilterAttrs: netlink.FilterAttrs{
				LinkIndex: link.Attrs().Index,
				Parent:    netlink.HANDLE_MIN_INGRESS,
				Handle:    1,
				Protocol:  3,
			},
			Fd:           nm.programs.TcIngress.FD(),
			Name:         "ingress",
			DirectAction: true,
		}

		if err := netlink.FilterAdd(filterIngress); err != nil {
			return fmt.Errorf("failed to add ingress filter: %w", err)
		}

		// Add egress filter
		filterEgress := &netlink.BpfFilter{
			FilterAttrs: netlink.FilterAttrs{
				LinkIndex: link.Attrs().Index,
				Parent:    netlink.HANDLE_MIN_EGRESS,
				Handle:    1,
				Protocol:  3,
			},
			Fd:           nm.programs.TcEgress.FD(),
			Name:         "egress",
			DirectAction: true,
		}

		if err := netlink.FilterAdd(filterEgress); err != nil {
			return fmt.Errorf("failed to add egress filter: %w", err)
		}
	}

	return nil
}

// Stop detaches the eBPF programs and cleans up resources
func (nm *NetworkMonitor) Stop() error {
	if nm.tcIngress != nil {
		nm.tcIngress.Close()
	}
	if nm.tcEgress != nil {
		nm.tcEgress.Close()
	}
	if nm.programs != nil {
		nm.programs.Close()
	}
	return nil
}

// GetProcessStats retrieves network statistics for a specific PID
func (nm *NetworkMonitor) GetProcessStats(pid uint32) (*types.NetworkStats, error) {
	var stats netmonNetworkStats
	err := nm.maps.ProcessStats.Lookup(pid, &stats)
	if err != nil {
		if err == ebpf.ErrKeyNotExist {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to lookup stats: %w", err)
	}

	return &types.NetworkStats{
		BytesIn:        stats.BytesIn,
		BytesOut:       stats.BytesOut,
		PacketsIn:      stats.PacketsIn,
		PacketsOut:     stats.PacketsOut,
		TCPConnections: stats.TcpConnections,
		UDPConnections: stats.UdpConnections,
		ActiveConns:    make(map[string]types.ConnectionInfo),
	}, nil
}

// ClearProcessStats removes statistics for a specific PID
func (nm *NetworkMonitor) ClearProcessStats(pid uint32) error {
	return nm.maps.ProcessStats.Delete(pid)
}
