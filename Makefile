
.PHONY: build up-sidecar up-ebpf up-core up-observability clean

build:
	docker build -t sidecar-vs-ebpf/app:latest -f Dockerfile.app .
	docker build -t sidecar-vs-ebpf/bench:latest -f Dockerfile.bench .
	docker build -t sidecar-vs-ebpf/sidecar:latest -f Dockerfile.sidecar .
	docker build -t sidecar-vs-ebpf/observer:latest -f Dockerfile.observer .

up-sidecar:
	docker compose -f docker-compose.sidecar.yml up --build

up-ebpf:
	docker compose -f docker-compose.ebpf.yml up --build

up-core:
	docker compose -f docker-compose.ebpf-core.yml up --build

up-observability:
	docker compose -f docker-compose.observability.yml up --build -d

clean:
	docker compose -f docker-compose.sidecar.yml down --remove-orphans -v || true
	docker compose -f docker-compose.ebpf.yml down --remove-orphans -v || true
	docker compose -f docker-compose.ebpf-core.yml down --remove-orphans -v || true
	docker compose -f docker-compose.observability.yml down --remove-orphans -v || true
