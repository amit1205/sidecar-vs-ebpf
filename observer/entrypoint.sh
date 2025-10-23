
#!/usr/bin/env bash
set -euo pipefail

if ! command -v bpftool >/dev/null; then
  echo "bpftool is required"; exit 1
fi

if ! command -v docker >/dev/null; then
  echo "docker CLI is required (to resolve app PID via /var/run/docker.sock)"; exit 1
fi

APP_PID="${APP_PID:-}"
if [[ -z "${APP_PID}" ]]; then
  APP_PID="$(docker inspect --format '{{.State.Pid}}' app)"
fi
if [[ -z "${APP_PID}" || "${APP_PID}" == "0" ]]; then
  echo "Failed to resolve APP_PID"; exit 2
fi

cd /src/observer

echo "[observer] generating vmlinux.h and bpf2go bindings..."
go generate ./...

echo "[observer] building observer..."
go build -o /usr/local/bin/observer ./cmd/observer

echo "[observer] starting observer for PID ${APP_PID}"
exec /usr/local/bin/observer -pid "${APP_PID}" -listen ":9100"
