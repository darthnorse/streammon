ARG VERSION=dev

FROM node:20-slim AS frontend
WORKDIR /app/web
COPY web/package.json web/package-lock.json* ./
# --legacy-peer-deps required: react-leaflet-heatmap-layer-v3 declares peer dep on React 17
# but works correctly with React 18. The package is a beta version without updated peer deps.
RUN npm ci --legacy-peer-deps
COPY web/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.24-bookworm AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/internal/server/web/dist ./internal/server/web/dist
ARG VERSION=dev
ARG TARGETOS TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags "-X main.Version=${VERSION}" -o streammon ./cmd/streammon

FROM alpine:3.21
RUN apk add --no-cache ca-certificates shadow && \
    addgroup -g 10000 -S streammon && adduser -u 10000 -S streammon -G streammon
RUN mkdir -p /app/data /app/geoip && chown streammon:streammon /app /app/data /app/geoip
WORKDIR /app
COPY --chown=streammon:streammon --from=backend /app/streammon .
COPY --chown=streammon:streammon --from=backend /app/migrations ./migrations
COPY --chown=streammon:streammon entrypoint.sh .
RUN chmod +x entrypoint.sh
EXPOSE 7935
VOLUME ["/app/data", "/app/geoip"]
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO /dev/null http://localhost:7935/api/health || exit 1
ENTRYPOINT ["./entrypoint.sh"]
