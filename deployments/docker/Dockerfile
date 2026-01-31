# Build stage
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git make

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binaries
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /chronos ./cmd/chronos
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /chronosctl ./cmd/chronosctl

# Final stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -S chronos && adduser -S chronos -G chronos

# Copy binaries
COPY --from=builder /chronos /usr/local/bin/chronos
COPY --from=builder /chronosctl /usr/local/bin/chronosctl

# Create data directory
RUN mkdir -p /var/lib/chronos && chown chronos:chronos /var/lib/chronos

# Copy default config
COPY chronos.yaml.example /etc/chronos/chronos.yaml

USER chronos

WORKDIR /var/lib/chronos

EXPOSE 8080 7000 9000

VOLUME ["/var/lib/chronos"]

ENTRYPOINT ["chronos"]
CMD ["--config", "/etc/chronos/chronos.yaml"]
