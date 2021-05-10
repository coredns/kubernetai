package main

import (
	"github.com/coredns/coredns/core/dnsserver"
	_ "github.com/coredns/coredns/core/plugin"
	"github.com/coredns/coredns/coremain"

	"github.com/coredns/kubernetai/plugin/kubernetai"
)

func init() {
	for i, d := range dnsserver.Directives {
		if d != "kubernetes" {
			continue
		}
		dnsserver.Directives[i] = kubernetai.Name()
		return
	}
}

func main() {
	coremain.Run()
}
