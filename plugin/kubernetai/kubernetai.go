package kubernetai

import (
	"context"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/kubernetes"
	"github.com/coredns/coredns/plugin/pkg/fall"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/plugin/pkg/nonwriter"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
)

var log = clog.NewWithPlugin("kubernetai")

// Kubernetai handles multiple Kubernetes
type Kubernetai struct {
	Zones          []string
	Next           plugin.Handler
	Kubernetes     []*kubernetes.Kubernetes
	Fall           []fall.F
	autoPathSearch []string // Local search path from /etc/resolv.conf. Needed for autopath.
	p              podHandlerItf
}

// New creates a Kubernetai containing one Kubernetes with zones
func New(zones []string) (Kubernetai, *kubernetes.Kubernetes) {
	h := Kubernetai{
		autoPathSearch: searchFromResolvConf(),
		p:              &podHandler{},
	}
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
		if i != (len(k8i.Kubernetes)-1) && k8i.Fall[i].Through(state.Name()) {
			// Use a non-writer so we don't write NXDOMAIN to client
			nw := nonwriter.New(w)

			_, err := k.ServeDNS(ctx, nw, r)

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
	var searchPath []string

	// Abort if zone is not in kubernetai stanza.
	var zMatch bool
	for _, k8s := range k8i.Kubernetes {
		zone := plugin.Zones(k8s.Zones).Matches(state.Name())
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
		pod := k8i.p.PodWithIP(*k, state.IP())
		if pod == nil {
			return nil
		}

		search := make([]string, 3)
		for _, z := range k.Zones {
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
