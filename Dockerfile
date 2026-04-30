# ── Stage 1: Build ────────────────────────────────────────────────────────────
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /src

# Cache dependency layer separately.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
      -trimpath \
      -ldflags "-s -w -X main.version=${VERSION}" \
      -o /feishu-wol \
      ./cmd/feishu-wol

# ── Stage 2: Runtime ──────────────────────────────────────────────────────────
# Use alpine (not scratch) so that timezone data and CA certs are available
# for outbound HTTPS calls to the Feishu API.
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone

COPY --from=builder /feishu-wol /usr/local/bin/feishu-wol

# Default config location; mount your own at runtime.
RUN mkdir -p /etc/feishu-wol

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/feishu-wol"]
CMD ["-config", "/etc/feishu-wol/config.yaml"]
