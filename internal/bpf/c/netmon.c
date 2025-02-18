#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>
#include <bpf/bpf_tracing.h>

// Protocol definitions
#define ETH_P_IP    0x0800      /* Internet Protocol packet */
#define IPPROTO_TCP 6           /* Transmission Control Protocol */
#define IPPROTO_UDP 17          /* User Datagram Protocol */

// Map to store process network statistics
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 10000);
    __type(key, __u32);    // PID or TGID
    __type(value, struct network_stats);
} process_stats SEC(".maps");

// Map to store interface filtering
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 256);
    __type(key, __u32);    // Interface index
    __type(value, __u8);   // Enabled flag
} interface_filter SEC(".maps");

// Map to track process hierarchy
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 10000);
    __type(key, __u32);    // Child PID
    __type(value, __u32);  // Parent PID
} process_hierarchy SEC(".maps");

// Network statistics structure matching user space
struct network_stats {
    __u64 bytes_in;
    __u64 bytes_out;
    __u64 packets_in;
    __u64 packets_out;
    __u32 tcp_connections;
    __u32 udp_connections;
};

// Track process creation
SEC("tp/sched/sched_process_fork")
int trace_fork(struct trace_event_raw_sched_process_fork *ctx)
{
    __u32 parent_pid = bpf_get_current_pid_tgid() >> 32;
    __u32 child_pid = ctx->child_pid;

    // Store parent-child relationship
    bpf_map_update_elem(&process_hierarchy, &child_pid, &parent_pid, BPF_ANY);
    return 0;
}

static __always_inline __u32 get_root_pid(__u32 pid)
{
    // Traverse up the process hierarchy to find the root monitored process
    for (int i = 0; i < 5; i++) { // Limit traversal depth
        __u32 *parent = bpf_map_lookup_elem(&process_hierarchy, &pid);
        if (!parent)
            break;
        pid = *parent;
    }
    return pid;
}

static __always_inline int handle_skb(struct __sk_buff *skb, bool ingress)
{
    // Check interface filter if enabled
    __u32 ifindex = skb->ifindex;
    __u8 *enabled = bpf_map_lookup_elem(&interface_filter, &ifindex);
    if (enabled && !*enabled) {
        return 1; // Interface filtered out
    }

    // Get process ID
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;
    if (!pid) {
        // Try to get socket info if available
        struct bpf_sock *sk = skb->sk;
        if (!sk) {
            return 1; // No socket associated
        }
        // Use socket cookie as identifier
        __u64 cookie = bpf_get_socket_cookie(skb);
        if (!cookie) {
            return 1;
        }
        pid = (__u32)cookie;
    }

    // Get root process ID
    __u32 root_pid = get_root_pid(pid);
    if (!root_pid) {
        root_pid = pid; // Use current PID if no parent found
    }

    // Get or create statistics entry
    struct network_stats *stats, new_stats = {};
    stats = bpf_map_lookup_elem(&process_stats, &root_pid);
    if (!stats) {
        stats = &new_stats;
    }

    // Update packet and byte counts
    if (ingress) {
        stats->packets_in++;
        stats->bytes_in += skb->len;
    } else {
        stats->packets_out++;
        stats->bytes_out += skb->len;
    }

    // Protocol specific counting
    void *data = (void *)(long)skb->data;
    void *data_end = (void *)(long)skb->data_end;
    
    struct ethhdr *eth = data;
    if ((void*)(eth + 1) > data_end)
        goto update;

    if (eth->h_proto != bpf_htons(ETH_P_IP))
        goto update;

    struct iphdr *ip = (void*)(eth + 1);
    if ((void*)(ip + 1) > data_end)
        goto update;

    if (ip->protocol == IPPROTO_TCP) {
        struct tcphdr *tcp = (void*)(ip + 1);
        if ((void*)(tcp + 1) <= data_end) {
            if (tcp->syn && !tcp->ack)
                stats->tcp_connections++;
        }
    } else if (ip->protocol == IPPROTO_UDP) {
        struct udphdr *udp = (void*)(ip + 1);
        if ((void*)(udp + 1) <= data_end) {
            stats->udp_connections++;
        }
    }

update:
    if (stats == &new_stats) {
        bpf_map_update_elem(&process_stats, &root_pid, stats, BPF_ANY);
    } else {
        bpf_map_update_elem(&process_stats, &root_pid, stats, BPF_EXIST);
    }

    return 1;
}

SEC("classifier/ingress")
int tc_ingress(struct __sk_buff *skb)
{
    return handle_skb(skb, true);
}

SEC("classifier/egress")
int tc_egress(struct __sk_buff *skb)
{
    return handle_skb(skb, false);
}

char LICENSE[] SEC("license") = "GPL";