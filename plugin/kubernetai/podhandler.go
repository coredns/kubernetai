package kubernetai

import (
	"github.com/coredns/coredns/plugin/kubernetes"
	"github.com/coredns/coredns/plugin/kubernetes/object"
)

type podHandlerItf interface {
	PodWithIP(k kubernetes.Kubernetes, ip string) *object.Pod
}

type podHandler struct{}

// podWithIP return the api.Pod for source IP ip. It returns nil if nothing can be found.
func (p *podHandler) PodWithIP(k kubernetes.Kubernetes, ip string) *object.Pod {
	ps := k.APIConn.PodIndex(ip)
	if len(ps) == 0 {
		return nil
	}
	return ps[0]
}
