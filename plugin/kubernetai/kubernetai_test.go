package kubernetai

import (
	"context"
	"net"
	"reflect"
	"strings"
	"testing"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/kubernetes/object"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
)

var (
	podip string
)

// mockK8sPlugin satisfies the embeddedKubernetesPluginInterface interface and provides a mock kubernetes plugin that can be used to test kubernetai behaviour.
type mockK8sPlugin struct {
	zones       []string
	transfer    string
	transferErr error
}

var _ embeddedKubernetesPluginInterface = &mockK8sPlugin{}

// PodWithIP always returns a pod with the given ip address in the namespace 'test-1'.
func (mkp *mockK8sPlugin) PodWithIP(ip string) *object.Pod {
	if ip == "" {
		return nil
	}
	pod := &object.Pod{
		Namespace: "test-1",
		PodIP:     ip,
	}
	return pod
}

// Name satisfies the plugin.Handler interface but is not used for tests.
func (mkp *mockK8sPlugin) Name() string {
	return ""
}

// ServeDNS satisfies the plugin.Handler interface but is not used for tests.
func (mkp *mockK8sPlugin) ServeDNS(_ context.Context, _ dns.ResponseWriter, _ *dns.Msg) (rcode int, err error) {
	return 0, nil
}

// Transfer satisfies the transfer.Transferer interface by playing back canned transfer responses.
// The canned transfer response is stored in a textual representation.
func (mkp *mockK8sPlugin) Transfer(_ string, _ uint32) (<-chan []dns.RR, error) {
	if mkp.transferErr != nil {
		return nil, mkp.transferErr
	}

	ch := make(chan []dns.RR)
	go func() {
		zp := dns.NewZoneParser(strings.NewReader(mkp.transfer), "", "")
		for rr, ok := zp.Next(); ok; rr, ok = zp.Next() {
			ch <- []dns.RR{rr}
		}
		close(ch)
	}()

	return ch, nil
}

// Zones satisfies the embeddedKubernetesPluginInterface interface by returning pre-configured zones.
func (mkp *mockK8sPlugin) Zones() plugin.Zones {
	return plugin.Zones(mkp.zones)
}

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
		Kubernetes     []embeddedKubernetesPluginInterface
		autoPathSearch []string
	}
	type args struct {
		state request.Request
	}

	w := &responseWriterTest{}

	k8sClusterLocal := &mockK8sPlugin{
		zones: []string{
			"cluster.local.",
		},
	}
	k8sFlusterLocal := &mockK8sPlugin{
		zones: []string{
			"fluster.local.",
		},
	}
	defaultK8iConfig := fields{
		Kubernetes: []embeddedKubernetesPluginInterface{
			k8sFlusterLocal,
			k8sClusterLocal,
		},
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
			}
			podip = tt.ip
			if got := k8i.AutoPath(tt.args.state); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Kubernetai.AutoPath() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
