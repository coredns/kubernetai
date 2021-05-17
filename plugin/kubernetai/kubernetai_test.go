package kubernetai

import (
	"net"
	"reflect"
	"testing"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/kubernetes"
	"github.com/coredns/coredns/plugin/kubernetes/object"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
)

type k8iPodHandlerTester struct{}

var podip string

func (k8i *k8iPodHandlerTester) PodWithIP(k kubernetes.Kubernetes, ip string) *object.Pod {
	if ip == "" {
		return nil
	}
	pod := &object.Pod{
		Namespace: "test-1",
		PodIP:     ip,
	}
	return pod
}

var k8iPodHandlerTest k8iPodHandlerTester

type responseWriterTest struct {
	dns.ResponseWriter
}

func (res *responseWriterTest) RemoteAddr() net.Addr {
	ip := net.ParseIP(podip)
	return &net.UDPAddr{
		IP:   ip,
		Port: 53,
	}
}

func TestKubernetai_AutoPath(t *testing.T) {
	type fields struct {
		Zones          []string
		Next           plugin.Handler
		Kubernetes     []*kubernetes.Kubernetes
		autoPathSearch []string
		p              *k8iPodHandlerTester
	}
	type args struct {
		state request.Request
	}

	w := &responseWriterTest{}

	k8sClusterLocal := &kubernetes.Kubernetes{
		Zones: []string{
			"cluster.local.",
		},
	}
	k8sFlusterLocal := &kubernetes.Kubernetes{
		Zones: []string{
			"fluster.local.",
		},
	}
	defaultK8iConfig := fields{
		Kubernetes: []*kubernetes.Kubernetes{
			k8sFlusterLocal,
			k8sClusterLocal,
		},
		p: &k8iPodHandlerTest,
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		want   []string
		ip     string
	}{
		{
			name:   "standard autopath cluster.local",
			fields: defaultK8iConfig,
			args: args{
				state: request.Request{
					W: w,
					Req: &dns.Msg{
						Question: []dns.Question{
							{Name: "svc-1-a.test-1.svc.cluster.local.", Qtype: 1, Qclass: 1},
						},
					},
				},
			},
			want: []string{"test-1.svc.cluster.local.", "svc.cluster.local.", "cluster.local.", "test-1.svc.fluster.local.", "svc.fluster.local.", "fluster.local.", ""},
			ip:   "172.17.0.7",
		},
		{
			name:   "standard autopath servicename.svc",
			fields: defaultK8iConfig,
			args: args{
				state: request.Request{
					W: w,
					Req: &dns.Msg{
						Question: []dns.Question{
							{Name: "svc-2-a.test-2.test-1.svc.cluster.local.", Qtype: 1, Qclass: 1},
						},
					},
				},
			},
			want: []string{"test-1.svc.cluster.local.", "svc.cluster.local.", "cluster.local.", "test-1.svc.fluster.local.", "svc.fluster.local.", "fluster.local.", ""},
			ip:   "172.17.0.7",
		},
		{
			name:   "standard autopath lookup fluster in cluster.local",
			fields: defaultK8iConfig,
			args: args{
				state: request.Request{
					W: w,
					Req: &dns.Msg{
						Question: []dns.Question{
							{Name: "svc-d.test-2.svc.fluster.local.svc.cluster.local.", Qtype: 1, Qclass: 1},
						},
					},
				},
			},
			want: []string{"test-1.svc.cluster.local.", "svc.cluster.local.", "cluster.local.", "test-1.svc.fluster.local.", "svc.fluster.local.", "fluster.local.", ""},
			ip:   "172.17.0.7",
		},
		{
			name:   "not in zone",
			fields: defaultK8iConfig,
			args: args{
				state: request.Request{
					W: w,
					Req: &dns.Msg{
						Question: []dns.Question{
							{Name: "svc-1-a.test-1.svc.zone.local.", Qtype: 1, Qclass: 1},
						},
					},
				},
			},
			ip:   "172.17.0.7",
			want: nil,
		},
		{
			name:   "requesting pod does not exist",
			fields: defaultK8iConfig,
			args: args{
				state: request.Request{
					W: w,
					Req: &dns.Msg{
						Question: []dns.Question{
							{Name: "svc-1-a.test-1.svc.zone.local.", Qtype: 1, Qclass: 1},
						},
					},
				},
			},
			ip:   "",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k8i := Kubernetai{
				Zones:          tt.fields.Zones,
				Kubernetes:     tt.fields.Kubernetes,
				autoPathSearch: tt.fields.autoPathSearch,
				p:              tt.fields.p,
			}
			podip = tt.ip
			if got := k8i.AutoPath(tt.args.state); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Kubernetai.AutoPath() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
