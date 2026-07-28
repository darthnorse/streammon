package clientip

import (
	"strings"
	"testing"
)

func TestParseTrustedProxies(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		isSet   bool
		want    []string
		wantErr string
	}{
		{
			name:  "unset uses the RFC1918 default",
			isSet: false,
			want:  []string{"127.0.0.1/32", "::1/128", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"},
		},
		{name: "explicitly empty trusts nothing", raw: "", isSet: true, want: nil},
		{name: "none trusts nothing", raw: "none", isSet: true, want: nil},
		{name: "NONE is case-insensitive", raw: "NONE", isSet: true, want: nil},
		{
			name:  "bare IPv4 becomes a host prefix",
			raw:   "192.168.0.24",
			isSet: true,
			want:  []string{"192.168.0.24/32"},
		},
		{name: "bare IPv6 becomes a host prefix", raw: "::1", isSet: true, want: []string{"::1/128"}},
		{name: "explicit CIDR passes through", raw: "192.168.0.0/16", isSet: true, want: []string{"192.168.0.0/16"}},
		{
			name:  "comma separated with whitespace",
			raw:   " 127.0.0.1/32 , 192.168.0.24 ",
			isSet: true,
			want:  []string{"127.0.0.1/32", "192.168.0.24/32"},
		},
		{name: "unmasked CIDR is normalised", raw: "192.168.0.24/16", isSet: true, want: []string{"192.168.0.0/16"}},
		{name: "invalid entry names the entry", raw: "192.168.0.999", isSet: true, wantErr: "192.168.0.999"},
		{name: "invalid CIDR names the entry", raw: "10.0.0.0/99", isSet: true, wantErr: "10.0.0.0/99"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTrustedProxies(tt.raw, tt.isSet)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected an error mentioning %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error %q does not name the offending entry %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d prefixes %v, want %d %v", len(got), got, len(tt.want), tt.want)
			}
			for i, p := range got {
				if p.String() != tt.want[i] {
					t.Errorf("prefix %d = %s, want %s", i, p, tt.want[i])
				}
			}
		})
	}
}
