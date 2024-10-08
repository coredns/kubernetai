package kubernetai

import (
	"context"
	"errors"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/kubernetes"
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

	prev := &kubernetes.Kubernetes{}
	for _, k := range k8i.Kubernetes {
		onStart, onShut, err := k.(*embeddedKubernetes).InitKubeCache(context.Background())
		if err != nil {
			return plugin.Error(Name(), err)
		}
		if onShut != nil {
			c.OnShutdown(onShut)
		}
		if onStart != nil {
			c.OnStartup(onStart)
		}
		// set Next of the previous kubernetes instance to the current instance
		prev.Next = k.(*embeddedKubernetes).Kubernetes
		prev = k.(*embeddedKubernetes).Kubernetes
	}

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		// set Next of the last kubernetes instance to the next plugin
		k8i.Kubernetes[len(k8i.Kubernetes)-1].(*embeddedKubernetes).Next = next
		return k8i
	})

	return nil
}

// Parse parses multiple kubernetes into a kubernetai
func Parse(c *caddy.Controller) (*Kubernetai, error) {
	var k8i = &Kubernetai{
		autoPathSearch: searchFromResolvConf(),
	}

	for c.Next() {
		k8s, err := kubernetes.ParseStanza(c)
		if err != nil {
			return nil, err
		}
		k8i.Kubernetes = append(k8i.Kubernetes, newEmbeddedKubernetes(k8s))
	}

	if len(k8i.Kubernetes) == 0 {
		return nil, errors.New("no kubernetes instance was parsed")
	}

	return k8i, nil
}
