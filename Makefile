.PHONY: dev-backend dev-frontend build docker test

dev-backend:
	go run ./cmd/streammon

dev-frontend:
	cd web && npm run dev

build:
	cd web && npm ci --legacy-peer-deps && npm run build
	CGO_ENABLED=1 go build -o streammon ./cmd/streammon

docker:
	docker compose build

test:
	go test ./...
	cd web && npm test
