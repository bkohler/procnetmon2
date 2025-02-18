# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
BINARY_NAME=procnetmon2

# eBPF parameters
CLANG ?= clang
ARCH := $(shell uname -m | sed 's/x86_64/amd64/g')
CFLAGS := -O2 -g -Wall -Werror \
    -target bpf \
    -D__TARGET_ARCH_$(ARCH) \
    -I/usr/include/bpf \
    -I/usr/include/x86_64-linux-gnu \
    -I/usr/include \
    -I/usr/src/linux-headers-$(shell uname -r)/arch/x86/include \
    -I/usr/src/linux-headers-$(shell uname -r)/arch/x86/include/generated \
    -I/usr/src/linux-headers-$(shell uname -r)/include

.PHONY: all build clean test run generate

all: generate build

build: generate
	$(GOBUILD) -v -o $(BINARY_NAME) ./cmd/procnetmon2

clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f internal/bpf/c/netmon.o

test:
	$(GOTEST) -v ./...

run: build
	sudo ./$(BINARY_NAME)

generate: internal/bpf/c/netmon.o
	$(GOCMD) generate ./internal/bpf

internal/bpf/c/netmon.o: internal/bpf/c/netmon.c
	$(CLANG) $(CFLAGS) -c $< -o $@

deps:
	$(GOGET) -v ./...