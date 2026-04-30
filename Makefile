BINARY     := feishu-wol
MODULE     := github.com/LE-saber/Wrt-Wol
CMD        := ./cmd/feishu-wol

VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "1.0.0")
LDFLAGS    := -s -w -X main.version=$(VERSION)
BUILD_ARGS := -trimpath -ldflags "$(LDFLAGS)"

OUTDIR     := dist
GOPROXY    ?= https://goproxy.cn,direct
export GOPROXY

.PHONY: all build build-mipsel build-mips build-arm build-arm64 build-x86 \
        build-all ipk clean fmt vet tidy

all: build

$(OUTDIR):
	mkdir -p $(OUTDIR)

## ── Native (amd64) ───────────────────────────────────────────────────────────
build: $(OUTDIR)
	CGO_ENABLED=0 go build $(BUILD_ARGS) -o $(OUTDIR)/$(BINARY) $(CMD)
	@echo "Built: $(OUTDIR)/$(BINARY)"

## ── OpenWrt cross-compilation ────────────────────────────────────────────────
build-mipsel: $(OUTDIR)
	GOOS=linux GOARCH=mipsle CGO_ENABLED=0 \
	  go build $(BUILD_ARGS) -o $(OUTDIR)/$(BINARY)-mipsel $(CMD)

build-mips: $(OUTDIR)
	GOOS=linux GOARCH=mips CGO_ENABLED=0 \
	  go build $(BUILD_ARGS) -o $(OUTDIR)/$(BINARY)-mips $(CMD)

build-arm: $(OUTDIR)
	GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 \
	  go build $(BUILD_ARGS) -o $(OUTDIR)/$(BINARY)-arm $(CMD)

build-arm64: $(OUTDIR)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
	  go build $(BUILD_ARGS) -o $(OUTDIR)/$(BINARY)-arm64 $(CMD)

build-x86: $(OUTDIR)
	GOOS=linux GOARCH=386 CGO_ENABLED=0 \
	  go build $(BUILD_ARGS) -o $(OUTDIR)/$(BINARY)-x86 $(CMD)

build-all: build build-mipsel build-mips build-arm build-arm64 build-x86
	@ls -lh $(OUTDIR)/

## ── .ipk package ─────────────────────────────────────────────────────────────
# Usage:
#   make ipk                          → x86_64 (ImmortalWrt x86)
#   make ipk ARCH=mipsel BINARY=dist/feishu-wol-mipsel
ipk: build
	VERSION=$(VERSION) ARCH=$(shell uname -m) \
	  BINARY=$(OUTDIR)/$(BINARY) OUTDIR=$(OUTDIR) \
	  python3 scripts/build-ipk.py

## Cross-compile + package for a specific OpenWrt target, e.g.:
##   make ipk-mipsel   → Xiaomi / TP-Link MIPS routers
##   make ipk-arm      → ARM-based routers
##   make ipk-x86      → x86 32-bit

ipk-mipsel: build-mipsel
	VERSION=$(VERSION) ARCH=mipsel BINARY=$(OUTDIR)/$(BINARY)-mipsel OUTDIR=$(OUTDIR) \
	  python3 scripts/build-ipk.py

ipk-mips: build-mips
	VERSION=$(VERSION) ARCH=mips BINARY=$(OUTDIR)/$(BINARY)-mips OUTDIR=$(OUTDIR) \
	  python3 scripts/build-ipk.py

ipk-arm: build-arm
	VERSION=$(VERSION) ARCH=arm BINARY=$(OUTDIR)/$(BINARY)-arm OUTDIR=$(OUTDIR) \
	  python3 scripts/build-ipk.py

ipk-arm64: build-arm64
	VERSION=$(VERSION) ARCH=aarch64 BINARY=$(OUTDIR)/$(BINARY)-arm64 OUTDIR=$(OUTDIR) \
	  python3 scripts/build-ipk.py

ipk-x86: build-x86
	VERSION=$(VERSION) ARCH=i386 BINARY=$(OUTDIR)/$(BINARY)-x86 OUTDIR=$(OUTDIR) \
	  python3 scripts/build-ipk.py

ipk-all: build-all
	$(MAKE) ipk-mipsel ipk-mips ipk-arm ipk-arm64 ipk-x86

## ── Dev helpers ──────────────────────────────────────────────────────────────
fmt:
	gofmt -w .

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -rf $(OUTDIR)
