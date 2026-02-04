FROM node:20-slim AS frontend
WORKDIR /app/web
COPY web/package.json web/package-lock.json* ./
# --legacy-peer-deps required: react-leaflet-heatmap-layer-v3 declares peer dep on React 17
# but works correctly with React 18. The package is a beta version without updated peer deps.
RUN npm ci --legacy-peer-deps
COPY web/ ./
RUN npm run build

FROM golang:1.24-bookworm AS backend
WORKDIR /app
RUN apt-get update && apt-get install -y gcc libsqlite3-dev
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/internal/server/web/dist ./internal/server/web/dist
RUN CGO_ENABLED=1 go build -o streammon ./cmd/streammon

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y ca-certificates libsqlite3-0 && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=backend /app/streammon .
COPY --from=backend /app/migrations ./migrations
EXPOSE 7935
VOLUME ["/app/data", "/app/geoip"]
CMD ["./streammon"]
