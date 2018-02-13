# Makefile for building CoreDNS with Kubernetai
GITCOMMIT:=$(shell git describe --dirty --always)
BINARY:=coredns
SYSTEM:=
VERBOSE:=-v

all: coredns

.PHONY: coredns
coredns:
	CGO_ENABLED=0 $(SYSTEM) go build $(VERBOSE) -ldflags="-s -w -X github.com/coredns/coredns/coremain.GitCommit=$(GITCOMMIT)" -o $(BINARY)
