
// SPDX-License-Identifier: GPL-2.0
#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

char LICENSE[] SEC("license") = "GPL";

struct event {
    __u32 pid;
    __u64 bytes;
    __u32 kind; // 1=write, 2=tcp_sendmsg, 3=uprobe_appwrite
};

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 1 << 24);
} events SEC(".maps");

volatile const __u32 target_pid = 0;

SEC("tracepoint/syscalls/sys_enter_write")
int tp__sys_enter_write(struct trace_event_raw_sys_enter* ctx) {
    __u64 id = bpf_get_current_pid_tgid();
    __u32 pid = id >> 32;
    if (target_pid != 0 && pid != target_pid) return 0;

    struct event *e = bpf_ringbuf_reserve(&events, sizeof(*e), 0);
    if (!e) return 0;
    e->pid = pid;
    e->bytes = ctx->args[2];
    e->kind = 1;
    bpf_ringbuf_submit(e, 0);
    return 0;
}

SEC("kprobe/tcp_sendmsg")
int kprobe__tcp_sendmsg(struct pt_regs *ctx) {
    __u64 id = bpf_get_current_pid_tgid();
    __u32 pid = id >> 32;
    if (target_pid != 0 && pid != target_pid) return 0;

#if defined(__TARGET_ARCH_x86)
    __u64 size = PT_REGS_PARM3(ctx);
#else
    __u64 size = 0;
#endif
    struct event *e = bpf_ringbuf_reserve(&events, sizeof(*e), 0);
    if (!e) return 0;
    e->pid = pid;
    e->bytes = size;
    e->kind = 2;
    bpf_ringbuf_submit(e, 0);
    return 0;
}

/* Uprobe symbol 'AppWrite' in the Go app (first arg: size) */
SEC("uprobe/AppWrite")
int uprobe__AppWrite(struct pt_regs *ctx) {
    __u64 id = bpf_get_current_pid_tgid();
    __u32 pid = id >> 32;
    if (target_pid != 0 && pid != target_pid) return 0;
#if defined(__TARGET_ARCH_x86)
    __u64 size = PT_REGS_PARM1(ctx);
#else
    __u64 size = 0;
#endif
    struct event *e = bpf_ringbuf_reserve(&events, sizeof(*e), 0);
    if (!e) return 0;
    e->pid = pid;
    e->bytes = size;
    e->kind = 3;
    bpf_ringbuf_submit(e, 0);
    return 0;
}
