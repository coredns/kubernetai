package kubernetai

import (
	"context"
	"fmt"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/etcd/msg"
	"github.com/coredns/coredns/plugin/kubernetes"
	"github.com/coredns/coredns/plugin/pkg/fall"
	"github.com/coredns/coredns/plugin/pkg/nonwriter"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

// Kubernetai handles multiple Kubernetes
type Kubernetai struct {
	Zones      []string
	Next       plugin.Handler
	Kubernetes []*kubernetes.Kubernetes
}

// New creates a Kubernetai containing one Kubernetes with zones
func New(zones []string) (Kubernetai, *kubernetes.Kubernetes) {
	h := Kubernetai{}
	k := kubernetes.New(zones)
	h.Kubernetes = append(h.Kubernetes, k)
	return h, k
}

// ServeDNS routes requests to the authoritative kubernetes. It implements the plugin.Handler interface.
func (k8i Kubernetai) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (rcode int, err error) {
	state := request.Request{W: w, Req: r}
	for i, k := range k8i.Kubernetes {
		zone := plugin.Zones(k.Zones).Matches(state.Name())
		if zone == "" {
			continue
		}

		// If fallthrough is enabled and there are more kubernetes in the list, then we
		// should continue to the next kubernetes in the list (not next plugin) when
		// ServeDNS results in NXDOMAIN.
		if i != (len(k8i.Kubernetes)-1) && k.Fall.Through(state.Name()) {
			// Use a non-writer so we don't write NXDOMAIN to client
			nw := nonwriter.New(w)

			// Temporarily disable fallthrough to prevent going to the next plugin in kubernetes.ServeDNS
			oFall := k.Fall
			k.Fall = fall.F{}

			_, err := k.ServeDNS(ctx, nw, r)

			// Restore fallthrough
			k.Fall = oFall

			// Return SERVFAIL if error
			if err != nil {
				return dns.RcodeServerFailure, err
			}

			// If NXDOMAIN, continue to next kubernetes instead of next plugin
			if nw.Msg.Rcode == dns.RcodeNameError {
				continue
			}

			// Otherwise write message to client
			m := nw.Msg
			state.SizeAndDo(m)
			m = state.Scrub(m)
			w.WriteMsg(m)

			return m.Rcode, err

		} else {
			rcode, err = k.ServeDNS(ctx, w, r)
		}

		return rcode, err
	}
	return plugin.NextOrFailure(k8i.Name(), k8i.Next, ctx, w, r)
}

// AutoPath routes AutoPath requests to the authoritative kubernetes.
func (k8i Kubernetai) AutoPath(state request.Request) []string {
	for _, k := range k8i.Kubernetes {
		zone := plugin.Zones(k.Zones).Matches(state.Name())
		if zone == "" {
			continue
		}
		return k.AutoPath(state)
	}
	return nil
}

// Federations routes Federations requests to the authoritative kubernetes.
func (k8i Kubernetai) Federations(state request.Request, fname, fzone string) (msg.Service, error) {
	for _, k := range k8i.Kubernetes {
		zone := plugin.Zones(k.Zones).Matches(state.Name())
		if zone == "" {
			continue
		}
		return k.Federations(state, fname, fzone)
	}
	return msg.Service{}, fmt.Errorf("could not find a kubernetes authoritative for %v", state.Name())
}

// Health implements the health.Healther interface.
func (k8i Kubernetai) Health() bool {
	healthy := true
	for _, k := range k8i.Kubernetes {
		healthy = healthy && k.APIConn.HasSynced()
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
