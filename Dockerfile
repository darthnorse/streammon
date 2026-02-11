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
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=backend /app/streammon .
COPY --from=backend /app/migrations ./migrations
EXPOSE 7935
VOLUME ["/app/data", "/app/geoip"]
CMD ["./streammon"]
