package kubernetai

import (
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/kubernetes"

	"github.com/mholt/caddy"
)

func init() {
	caddy.RegisterPlugin(Name(), caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	k8i, err := Parse(c)
	if err != nil {
		return plugin.Error(Name(), err)
	}

	for _, k := range k8i.Kubernetes {
		err = k.InitKubeCache()
		if err != nil {
			return plugin.Error(Name(), err)
		}
		k.RegisterKubeCache(c)
	}

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		k8i.Next = next
		return k8i
	})

	return nil
}

// Parse parses multiple kubernetes into a kubernetai
func Parse(c *caddy.Controller) (*Kubernetai, error) {
	var k8i = &Kubernetai{
		autoPathSearch: searchFromResolvConf(),
		p:              &podHandler{},
	}
	var err error
	for c.Next() {
		var k8s *kubernetes.Kubernetes
		k8s, err = kubernetes.ParseStanza(c)
		if err != nil {
			return nil, err
		}
		k8i.Kubernetes = append(k8i.Kubernetes, k8s)
	}
	return k8i, nil
}
