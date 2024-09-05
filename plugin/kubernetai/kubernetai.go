// Package kubernetai implements a plugin which can embed a number of kubernetes plugins in the same dns server.
package kubernetai

import (
	"context"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/kubernetes"
	"github.com/coredns/coredns/plugin/kubernetes/object"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/plugin/transfer"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
)

var log = clog.NewWithPlugin("kubernetai")

// embeddedKubernetesPluginInterface describes the kubernetes plugin interface that kubernetai requires/uses.
type embeddedKubernetesPluginInterface interface {
	plugin.Handler
	transfer.Transferer
	PodWithIP(ip string) (pod *object.Pod)
	Zones() (zones plugin.Zones)
}

// embeddedKubernetes wraps a real kubernetes plugin
type embeddedKubernetes struct {
	*kubernetes.Kubernetes
}

var _ embeddedKubernetesPluginInterface = &embeddedKubernetes{}

func newEmbeddedKubernetes(k *kubernetes.Kubernetes) *embeddedKubernetes {
	return &embeddedKubernetes{
		Kubernetes: k,
	}
}

// PodWithIP satisfies the embeddedKubernetesPluginInterface by adding this additional method not exported from the kubernetes plugin.
func (ek embeddedKubernetes) PodWithIP(ip string) *object.Pod {
	if ek.Kubernetes == nil {
		return nil
	}
	ps := ek.Kubernetes.APIConn.PodIndex(ip)
	if len(ps) == 0 {
		return nil
	}
	return ps[0]
}

// Zones satisfies the embeddedKubernetesPluginInterface by providing access to the kubernetes plugin Zones.
func (ek embeddedKubernetes) Zones() plugin.Zones {
	if ek.Kubernetes == nil {
		return nil
	}
	return plugin.Zones(ek.Kubernetes.Zones)
}

// Kubernetai handles multiple Kubernetes
type Kubernetai struct {
	Zones          []string
	Kubernetes     []embeddedKubernetesPluginInterface
	autoPathSearch []string // Local search path from /etc/resolv.conf. Needed for autopath.
}

// New creates a Kubernetai containing one Kubernetes with zones
func New(zones []string) (Kubernetai, *kubernetes.Kubernetes) {
	h := Kubernetai{
		autoPathSearch: searchFromResolvConf(),
	}
	k := kubernetes.New(zones)
	ek := newEmbeddedKubernetes(k)
	h.Kubernetes = append(h.Kubernetes, ek)
	return h, k
}

// ServeDNS routes requests to the authoritative kubernetes. It implements the plugin.Handler interface.
func (k8i Kubernetai) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (rcode int, err error) {
	return k8i.Kubernetes[0].ServeDNS(ctx, w, r)
}

// AutoPath routes AutoPath requests to the authoritative kubernetes.
func (k8i Kubernetai) AutoPath(state request.Request) []string {
	var searchPath []string

	// Abort if zone is not in kubernetai stanza.
	var zMatch bool
	for _, k8s := range k8i.Kubernetes {
		zone := k8s.Zones().Matches(state.Name())
		if zone != "" {
			zMatch = true
			break
		}
	}
	if !zMatch {
		return nil
	}

	// Add autopath result for the handled zones
	for _, k := range k8i.Kubernetes {
		pod := k.PodWithIP(state.IP())
		if pod == nil {
			return nil
		}

		search := make([]string, 3)
		for _, z := range k.Zones() {
			if z == "." {
				search[0] = pod.Namespace + ".svc."
				search[1] = "svc."
				search[2] = "."
			} else {
				search[0] = pod.Namespace + ".svc." + z
				search[1] = "svc." + z
				search[2] = z
			}
			searchPath = append(search, searchPath...)
		}
	}
	searchPath = append(searchPath, k8i.autoPathSearch...)
	searchPath = append(searchPath, "")
	log.Debugf("Autopath search path for '%s' will be '%v'", state.Name(), searchPath)
	return searchPath
}

// Transfer supports the transfer plugin, implementing the Transferer interface, by calling Transfer on each of the embedded plugins.
// It will return a channel to the FIRST kubernetai stanza that reports that it is authoritative for the requested zone.
func (k8i Kubernetai) Transfer(zone string, serial uint32) (retCh <-chan []dns.RR, err error) {
	for _, k := range k8i.Kubernetes {
		retCh, err = k.Transfer(zone, serial)
		if err == transfer.ErrNotAuthoritative {
			continue
		}
		return
	}
	// none of the embedded plugins were authoritative
	return nil, transfer.ErrNotAuthoritative
}

func searchFromResolvConf() []string {
	rc, err := dns.ClientConfigFromFile("/etc/resolv.conf")
	if err != nil {
		return nil
	}
	plugin.Zones(rc.Search).Normalize()
	return rc.Search
}

// Health implements the health.Healther interface.
func (k8i Kubernetai) Health() bool {
	healthy := true
	for _, k := range k8i.Kubernetes {
		healthy = healthy && k.(*embeddedKubernetes).APIConn.HasSynced()
		if !healthy {
			break
		}
	}
	return healthy
}

// Name implements the Handler interface.
func (Kubernetai) Name() string { return Name() }

// Name is the name of the plugin.
func Name() string { return "kubernetai" }
