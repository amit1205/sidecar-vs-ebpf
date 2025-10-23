
#!/usr/bin/env bash
set -euo pipefail
if [ $# -lt 2 ]; then
  echo "usage: $0 sidecar.txt ebpf.txt"; exit 2
fi
python3 parse_bench.py "$1" "$2"
