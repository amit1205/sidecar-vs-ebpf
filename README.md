# Sidecar vs eBPF: Containerized Benchmark (Go Implementation)

This repo demonstrates a simple Go HTTP service and three observability approaches:

1) **Sidecar logging** via Fluent Bit (tails file logs)
2) **eBPF tracing** via bpftrace (tracepoint demo)
3) **CO-RE eBPF observer in Go** (cilium/ebpf) exporting Prometheus

Plus a tiny **bench** load generator and an optional **Prometheus + Grafana** stack.

## Quick Start

### Prereqs
- Docker + Docker Compose
- Linux host for eBPF modes (kprobe/tracepoint/uprobe)
- Kernel BTF available at `/sys/kernel/btf/vmlinux` (most modern distros)

### Build
```bash
make build
```

### Run with **Sidecar**
```bash
docker compose -f docker-compose.sidecar.yml up --remove-orphans --build
```

# In another terminal:

```bash
docker compose -f docker-compose.sidecar.yml run --rm bench   -url http://app:8080/ping -conns 50 -dur 20s
```

### Run with **bpftrace** (tracepoint demo)
```bash
docker compose -f docker-compose.ebpf.yml up --remove-orphans --build
# In another terminal:
docker compose -f docker-compose.ebpf.yml run --rm bench   -url http://app:8080/ping -conns 50 -dur 20s
```

### Run **CO-RE eBPF Observer (Prometheus exporter)**
```bash
docker compose -f docker-compose.ebpf-core.yml up --remove-orphans --build
# In another terminal:
docker compose -f docker-compose.ebpf-core.yml run --rm bench   -url http://app:8080/ping -conns 100 -dur 30s
# Metrics: http://localhost:9100/metrics
```

### Full Observability (App + Observer + Prometheus + Grafana)
```bash
docker compose -f docker-compose.observability.yml up --build -d
# Drive load as above
```
Open:
- Prometheus → http://localhost:9090
- Grafana → http://localhost:3000 (anonymous enabled)
  - Dashboard: **Sidecar vs eBPF (App + Observer)**

## Comparing Performance

Capture identical runs and compare:
```bash
# Sidecar
docker compose -f docker-compose.sidecar.yml run --rm bench -url http://app:8080/ping -conns 100 -dur 30s > sidecar.txt
# eBPF (CO-RE)
docker compose -f docker-compose.ebpf-core.yml run --rm bench -url http://app:8080/ping -conns 100 -dur 30s > ebpf.txt

# Diff
bash scripts/compare.sh sidecar.txt ebpf.txt
# or JSON
python3 scripts/parse_bench.py sidecar.txt ebpf.txt --json
```

For more stress, try CPU work:
```bash
docker compose -f docker-compose.sidecar.yml run --rm bench -url 'http://app:8080/work?ms=2' -conns 100 -dur 30s
docker compose -f docker-compose.ebpf-core.yml run --rm bench -url 'http://app:8080/work?ms=2' -conns 100 -dur 30s
```

## Metrics

### From the App (`/metrics` on :8080)
- `app_http_requests_total{path,method,code}`
- `app_http_request_duration_seconds_bucket` (+ use PromQL for p50/p95/p99)

### From the CO-RE eBPF observer (`/metrics` on :9100)
- `app_write_bytes_total`, `app_write_events_total` (tracepoint `sys_enter_write`)
- `app_tcp_send_bytes_total`, `app_tcp_send_events_total` (**kprobe** `tcp_sendmsg`)
- `app_uprobe_bytes_total`, `app_uprobe_calls_total` (**uprobe** on Go `AppWrite`) — calls/sec ≈ CPS

## How the CO-RE observer works

- Builds a BPF program at container start using the host BTF (`bpftool btf dump …` + `bpf2go`).
- Attaches:
  - tracepoint: `sys_enter_write`
  - kprobe: `tcp_sendmsg`
  - uprobe: `AppWrite` (exported marker in the Go app, called with response size)
- Streams events via ring buffer, updates Prometheus counters.

## Security Notes
- Observer runs `privileged: true` for simplicity and mounts: `/sys/kernel/btf`, `/sys/fs/bpf`, `/sys/kernel/debug`, and `/var/run/docker.sock` (to resolve the app PID).
- For production, prefer scoped capabilities and pre-built CO-RE objects.

## Repo Layout
```
.
├── cmd/app/main.go
├── bench/bench.go
├── observer/
│   ├── bpf/trace_write.bpf.c
│   ├── cmd/observer/main.go
│   ├── utils.go
│   └── entrypoint.sh
├── ebpf/http_bytes.bt
├── fluent-bit/fluent-bit.conf
├── Dockerfile.app
├── Dockerfile.bench
├── Dockerfile.sidecar
├── Dockerfile.observer
├── docker-compose.sidecar.yml
├── docker-compose.ebpf.yml
├── docker-compose.ebpf-core.yml
├── docker-compose.observability.yml
├── prometheus/prometheus.yml
├── grafana/ (provisioning + dashboard)
├── scripts/parse_bench.py
├── scripts/compare.sh
├── Makefile
└── README.md
```

## Troubleshooting
- Ensure your kernel provides BTF at `/sys/kernel/btf/vmlinux`.
- Some kernels require `--privileged` or specific caps (e.g. `CAP_BPF`, `CAP_PERFMON`, `CAP_SYS_ADMIN`).
- On non-x86 arch, adjust argument extraction for kprobe/uprobe call conventions.
