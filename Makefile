# Makefile for building CoreDNS with Kubernetai for ci test automation
# This makefile is for testing and automation convenience only.
# To build a CoreDNS with the kubernetai plugin, build from the coredns/coredns repo.
# See docs in https://github.com/coredns/coredns/blob/master/plugin.cfg

GITCOMMIT:=$(shell git describe --dirty --always)
COVER = cd plugin/kubernetai && GO111MODULE=on go test -v -coverprofile=$(COVEROUT) -covermode=atomic -race
COVEROUT = cover.out
BINARY:=coredns
SYSTEM:=
VERBOSE:=-v

all: coredns

.PHONY: coredns
coredns:
	GO111MODULE=on CGO_ENABLED=0 $(SYSTEM) go build $(VERBOSE) -ldflags="-s -w -X github.com/coredns/coredns/coremain.GitCommit=$(GITCOMMIT)" -o $(BINARY)

.PHONY: test
test:
	@echo "Running tests..."
	@$(COVER)
