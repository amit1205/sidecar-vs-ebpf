
#!/usr/bin/env python3
import re, sys, json

def parse(path):
    data = {"samples": None, "rps": None, "p50": None, "p95": None, "p99": None}
    pat = {
        "samples": re.compile(r"^Samples:\s+(\d+)"),
        "rps": re.compile(r"^RPS:\s+([0-9.]+)"),
        "p50": re.compile(r"^p50:\s+([0-9.a-zµ]+)"),
        "p95": re.compile(r"^p95:\s+([0-9.a-zµ]+)"),
        "p99": re.compile(r"^p99:\s+([0-9.a-zµ]+)"),
    }
    def to_ms(s):
        if s.endswith('ms'): return float(s[:-2])
        if s.endswith('s'): return float(s[:-1]) * 1000.0
        if s.endswith('us') or s.endswith('µs'): return float(s[:-2]) / 1000.0
        return float(s)
    with open(path, 'r', errors='ignore') as f:
        for line in f:
            line = line.strip()
            for k, rx in pat.items():
                m = rx.match(line)
                if m:
                    v = m.group(1)
                    if k in ("p50","p95","p99"):
                        data[k] = to_ms(v)
                    elif k == "samples":
                        data[k] = int(v)
                    elif k == "rps":
                        data[k] = float(v)
    return data

def delta(a, b):
    out = {}
    for k in ["rps","p50","p95","p99","samples"]:
        if a.get(k) is None or b.get(k) is None:
            out[k] = None
        else:
            out[k] = b[k] - a[k]
    return out

if __name__ == "__main__":
    if len(sys.argv) < 3:
        print("usage: parse_bench.py SIDEcar.txt EBPF.txt [--json]")
        sys.exit(2)
    a = parse(sys.argv[1])
    b = parse(sys.argv[2])
    d = delta(a,b)
    if "--json" in sys.argv:
        print(json.dumps({"sidecar": a, "ebpf": b, "delta": d}, indent=2))
    else:
        print("=== Sidecar ==="); print(a)
        print("=== eBPF   ==="); print(b)
        print("=== Delta (eBPF - Sidecar) ==="); print(d)
        if a.get("rps") and b.get("rps"):
            rps_gain = (b["rps"] - a["rps"]) / a["rps"] * 100.0
            print(f"RPS change: {rps_gain:+.2f}%")
        for q in ["p50","p95","p99"]:
            if a.get(q) and b.get(q):
                lat_change = (b[q] - a[q]) / a[q] * 100.0
                print(f"{q} latency change: {lat_change:+.2f}% (ms)")
