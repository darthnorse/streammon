.PHONY: dev-backend dev-frontend build docker test

dev-backend:
	CGO_ENABLED=0 go run ./cmd/streammon

dev-frontend:
	cd web && npm run dev

build:
	cd web && npm ci --legacy-peer-deps && npm run build
	CGO_ENABLED=0 go build -ldflags "-X main.Version=$$(git describe --tags --always 2>/dev/null || echo dev)" -o streammon ./cmd/streammon

docker:
	docker compose build

test:
	CGO_ENABLED=0 go test ./...
	cd web && npm test
