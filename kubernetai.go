package kubernetai

import (
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/kubernetes"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
	"github.com/coredns/coredns/request"
	"github.com/coredns/coredns/plugin/pkg/fall"
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

// ServeDNS implements the plugin.Handler interface.
func (k8i Kubernetai) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (rcode int, err error) {
	state := request.Request{W: w, Req: r}
	for i, k := range k8i.Kubernetes {
		zone := plugin.Zones(k.Zones).Matches(state.Name())
		if zone == "" {
			continue
		}

		// If fallthrough is enabled and there are more kubernetes in the list we
		// should continue to the next kubernetes in the list (not next plugin)
		if i != (len(k8i.Kubernetes) - 1) && k.Fall.Through(state.Name()){
			// temporarily disable fallthrough to prevent fallthrough to next plugin
			oFall := k.Fall
			k.Fall = fall.F{}

			rcode, err = k.ServeDNS(ctx, w, r)

			// restore and handle fallthrough
			k.Fall = oFall
			if k.IsNameError(err) {
				// continue to next kubernetes instead of next plugin
				continue
			}
		} else {
			rcode, err = k.ServeDNS(ctx, w, r)
		}

		return rcode, err
	}
	return plugin.NextOrFailure(k8i.Name(), k8i.Next, ctx, w, r)
}

// Name implements the Handler interface.
func (Kubernetai) Name() string { return Name() }

// Name is the name of the plugin.
func Name() string { return "kubernetai" }
