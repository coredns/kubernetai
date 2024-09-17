package kubernetai

import (
	"strings"
	"testing"

	"github.com/coredns/coredns/plugin/transfer"

	"github.com/miekg/dns"
)

func TestKubernetesTransferNonAuthZone(t *testing.T) {
	type fields struct {
		name          string
		kubernetes    []*mockK8sPlugin
		zone          string
		serial        uint32
		expectedZone  string
		expectedError error
	}
	tests := []fields{
		{
			name: "TestSingleKubernetesTransferNonAuthZone",
			kubernetes: []*mockK8sPlugin{
				{
					zones:       []string{"cluster.local"},
					transferErr: transfer.ErrNotAuthoritative,
				},
			},
			zone:          "example.com",
			expectedError: transfer.ErrNotAuthoritative,
		},
		{
			name: "TestSingleKubernetesTransferAuthZone",
			kubernetes: []*mockK8sPlugin{
				{
					zones: []string{"cluster.local"},
					transfer: `
cluster.local.	5	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 3 7200 1800 86400 5
cluster.local.	5	IN	NS	ns.dns.cluster.local.
ns.dns.cluster.local.	5	IN	A	10.0.0.10
cluster.local.	5	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 3 7200 1800 86400 5
`,
					transferErr: nil,
				},
			},
			zone: "cluster.local",
			expectedZone: `
cluster.local.	5	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 3 7200 1800 86400 5
cluster.local.	5	IN	NS	ns.dns.cluster.local.
ns.dns.cluster.local.	5	IN	A	10.0.0.10
cluster.local.	5	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 3 7200 1800 86400 5
`,
			expectedError: nil,
		},
		{
			name: "TestMultipleNonAuthorititativeSingleAuthoritative",
			kubernetes: []*mockK8sPlugin{
				{
					zones: []string{"fluster.local"},
					transfer: `
fluster.local.	5	IN	SOA	ns.dns.fluster.local. hostmaster.fluster.local. 3 7200 1800 86400 5
fluster.local.	5	IN	NS	ns.dns.fluster.local.
ns.dns.fluster.local.	5	IN	A	10.0.0.10
fluster.local.	5	IN	SOA	ns.dns.fluster.local. hostmaster.fluster.local. 3 7200 1800 86400 5
`,
					transferErr: transfer.ErrNotAuthoritative,
				},
				{
					zones:       []string{"bluster.local"},
					transferErr: transfer.ErrNotAuthoritative,
				},
				{
					zones: []string{"cluster.local"},
					transfer: `
cluster.local.	5	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 3 7200 1800 86400 5
cluster.local.	5	IN	NS	ns.dns.cluster.local.
ns.dns.cluster.local.	5	IN	A	10.0.0.10
cluster.local.	5	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 3 7200 1800 86400 5
`,
					transferErr: nil,
				},
				{
					zones:       []string{"muster.local"},
					transferErr: transfer.ErrNotAuthoritative,
				},
			},
			zone: "cluster.local",
			expectedZone: `
cluster.local.	5	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 3 7200 1800 86400 5
cluster.local.	5	IN	NS	ns.dns.cluster.local.
ns.dns.cluster.local.	5	IN	A	10.0.0.10
cluster.local.	5	IN	SOA	ns.dns.cluster.local. hostmaster.cluster.local. 3 7200 1800 86400 5
`,
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// create kubernetai with mock kubernetes plugins
			kai := Kubernetai{}
			for _, plug := range tt.kubernetes {
				kai.Kubernetes = append(kai.Kubernetes, plug)
			}

			// create a axfr test message with test zone
			dnsmsg := &dns.Msg{}
			dnsmsg.SetAxfr(tt.zone)

			// perform AXFR
			ch, err := kai.Transfer(tt.zone, tt.serial)
			if err != nil {
				if err != tt.expectedError {
					t.Errorf("expected error %+v but received %+v", tt.expectedError, err)
				}
				return
			}
			validateAXFR(t, ch, tt.expectedZone)
		})
	}
}

func validateAXFR(t *testing.T, ch <-chan []dns.RR, expectedZone string) {
	xfr := []dns.RR{}
	for rrs := range ch {
		xfr = append(xfr, rrs...)
	}
	if xfr[0].Header().Rrtype != dns.TypeSOA {
		t.Error("Invalid transfer response, does not start with SOA record")
	}

	zp := dns.NewZoneParser(strings.NewReader(expectedZone), "", "")
	i := 0
	for rr, ok := zp.Next(); ok; rr, ok = zp.Next() {
		if !dns.IsDuplicate(rr, xfr[i]) {
			t.Fatalf("Record %d, expected\n%v\n, got\n%v", i, rr, xfr[i])
		}
		i++
	}

	if err := zp.Err(); err != nil {
		t.Fatal(err)
	}
}
