package kubernetai

import (
	"context"
	"testing"

	"github.com/miekg/dns"

	"github.com/coredns/coredns/plugin/kubernetes"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
)

func TestSetup(t *testing.T) {
	tests := []struct {
		input     string
		instances int
		hasNext   bool
	}{
		{
			input: `
				kubernetai cluster.local {
				  endpoint http://192.168.99.100
				}
			`,
			instances: 1,
			hasNext:   false,
		},
		{
			input: `
				kubernetai cluster.local {
				  endpoint http://192.168.99.100
				}
				kubernetai assemblage.local {
				  endpoint http://192.168.99.101
				}
			`,
			instances: 2,
			hasNext:   false,
		}, {
			input: `
				kubernetai cluster.local {
				  endpoint http://192.168.99.100
				}
				kubernetai assemblage.local {
				  endpoint http://192.168.99.101
				}
				kubernetai conglomeration.local {
				  endpoint http://192.168.99.102
				}
			`,
			instances: 3,
			hasNext:   false,
		},
		{
			input: `
				kubernetai cluster.local {
				  endpoint http://192.168.99.100
				}
			`,
			instances: 1,
			hasNext:   true,
		},
		{
			input: `
				kubernetai cluster.local {
				  endpoint http://192.168.99.100
				}
				kubernetai assemblage.local {
				  endpoint http://192.168.99.101
				}
			`,
			instances: 2,
			hasNext:   true,
		}, {
			input: `
				kubernetai cluster.local {
				  endpoint http://192.168.99.100
				}
				kubernetai assemblage.local {
				  endpoint http://192.168.99.101
				}
				kubernetai conglomeration.local {
				  endpoint http://192.168.99.102
				}
			`,
			instances: 3,
			hasNext:   true,
		},
	}

	for i, test := range tests {
		var nextHandler plugin.Handler
		if test.hasNext {
			handlerFunc := plugin.HandlerFunc(func(_ context.Context, _ dns.ResponseWriter, _ *dns.Msg) (int, error) {
				return 0, nil
			})
			nextHandler = &handlerFunc
		}

		c := caddy.NewTestController("dns", test.input)

		if err := setup(c); err != nil {
			t.Fatalf("Test %d: %v", i, err)
		}

		plugins := dnsserver.GetConfig(c).Plugin
		if n := len(plugins); n != 1 {
			t.Fatalf("Test %d: Expected plugin length on controller to be 1, got %d", i, n)
		}

		handler := plugins[0](nextHandler)

		k8i, ok := handler.(*Kubernetai)
		if !ok {
			t.Fatalf("Test %d: Expected handler to be Kubernetai, got %T", i, handler)
		}

		if n := len(k8i.Kubernetes); n != test.instances {
			t.Fatalf("Test %d: Expected kubernetes length on handler to be %d, got %d", i, test.instances, n)
		}

		prev := &kubernetes.Kubernetes{
			Next: k8i.Kubernetes[0],
		}
		for j, k := range k8i.Kubernetes {
			if prev.Next != k {
				t.Fatalf("Test %d: Expected kubernetes instance %d to be referencing kubernetes instance %d as next, got %v", i, j-1, j, prev.Next)
			}

			prev = k
		}

		if prev.Next != nextHandler {
			t.Fatalf("Test %d: Expected last kubernetes instance to be referencing nextHandler as next, got %v", i, prev.Next)
		}
	}
}
