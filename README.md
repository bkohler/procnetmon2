# ProcNetMon2

ProcNetMon2 is a high-performance network monitoring tool that uses eBPF to track process-specific network statistics in real-time. It provides detailed insights into network usage with minimal system overhead.

## Features

- Real-time network usage monitoring for specific processes
- Track both incoming and outgoing network traffic
- Display bandwidth usage in Kbps and Mbps
- Per-process network statistics including:
  - Total bytes transferred
  - Current transfer rates
  - Peak transfer rates
  - Active network connections
  - Protocol distribution (TCP/UDP)
- Interface filtering support
- Output in both human-readable and JSON formats
- Support for continuous monitoring or time-based sampling
- Aggregated statistics across multiple processes

## Requirements

- Linux kernel 5.5 or later
- LLVM/Clang for eBPF compilation
- Go 1.21 or later
- Linux headers
- CAP_BPF capability or root access

### Ubuntu/Debian

```bash
sudo apt-get update
sudo apt-get install -y \
    make \
    clang \
    llvm \
    linux-headers-$(uname -r) \
    golang-1.21
```

### Fedora/RHEL

```bash
sudo dnf install -y \
    make \
    clang \
    llvm \
    kernel-headers \
    golang
```

## Building

1. Clone the repository:
```bash
git clone https://github.com/bkohler/procnetmon2.git
cd procnetmon2
```

2. Build the project:
```bash
make
```

This will:
- Compile the eBPF program
- Generate Go bindings
- Build the main binary

## Usage

Basic usage requires root privileges or CAP_BPF capability:

```bash
# Monitor a single process
sudo ./procnetmon2 -p 1234

# Monitor multiple processes
sudo ./procnetmon2 -p 1234,5678

# Monitor with interface filtering
sudo ./procnetmon2 -p 1234 -i eth0

# Output in JSON format
sudo ./procnetmon2 -p 1234 --json

# Time-based sampling (e.g., 60 seconds)
sudo ./procnetmon2 -p 1234 -t 60s

# Show detailed connection information
sudo ./procnetmon2 -p 1234 --details

# Aggregate statistics across processes
sudo ./procnetmon2 -p 1234,5678 --aggregate
```

### Options

```
Flags:
  -p, --pids string        Comma-separated list of process IDs to monitor (required)
  -i, --interface string   Network interface to monitor (default: all)
  -j, --json              Output in JSON format
  -t, --time string       Time-based sampling period (e.g., 60s, 5m)
  -a, --aggregate         Aggregate statistics across monitored processes
  -c, --continuous        Enable continuous monitoring (default: true)
  -d, --details          Show detailed connection information
  -h, --help             Help for procnetmon2
```

## Output Example

### Human-readable format:
```
PID    Name      Rate In    Rate Out    Total In    Total Out    TCP    UDP
1234   nginx     1.5 Mbps   2.3 Mbps    1.2 GB      2.1 GB       12     0
5678   python    256 Kbps   128 Kbps    150 MB      75 MB         3     1
─────────────────────────────────────────────────────────────────────────
TOTAL            1.7 Mbps   2.4 Mbps    1.3 GB      2.2 GB       15     1
```

### JSON format:
```json
{
  "timestamp": "2025-02-18T17:48:42+01:00",
  "processes": {
    "1234": {
      "pid": 1234,
      "name": "nginx",
      "runtime": "1h2m3s",
      "current": {
        "bytes_in": 1288490188,
        "bytes_out": 2254857429,
        "rate_in": 1572864,
        "rate_out": 2411724,
        "tcp_connections": 12,
        "udp_connections": 0
      }
    }
  },
  "aggregated": {
    "bytes_in": 1288490188,
    "bytes_out": 2254857429,
    "rate_in": 1572864,
    "rate_out": 2411724,
    "tcp_connections": 12,
    "udp_connections": 0
  }
}
```
