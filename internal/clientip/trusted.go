// Package clientip resolves the HTTP client's IP address, honouring
// X-Forwarded-For only for requests that arrive from a configured trusted proxy.
package clientip

import (
	"fmt"
	"net/netip"
	"strings"
)

// DefaultTrustedProxies applies when TRUSTED_PROXIES is unset: loopback plus the
// RFC1918 ranges, which covers the reverse proxy in a typical self-hosted
// deployment without configuration. Every device in those ranges can forge its
// apparent client IP; set TRUSTED_PROXIES=none to opt out.
var DefaultTrustedProxies = []string{
	"127.0.0.1/32",
	"::1/128",
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"100.64.0.0/10", // CGNAT, as used by Tailscale/Headscale
	"fc00::/7",      // IPv6 unique-local (covers fc00::-fdff::)
}

// ParseTrustedProxies resolves the TRUSTED_PROXIES setting. isSet reports whether
// the variable was present at all (os.LookupEnv), which is what distinguishes
// "unset, use the default" from "explicitly empty, trust nothing".
func ParseTrustedProxies(raw string, isSet bool) ([]netip.Prefix, error) {
	if !isSet {
		return parseEntries(DefaultTrustedProxies)
	}

	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || strings.EqualFold(trimmed, "none") {
		return nil, nil
	}

	var entries []string
	for _, part := range strings.Split(trimmed, ",") {
		if p := strings.TrimSpace(part); p != "" {
			entries = append(entries, p)
		}
	}
	return parseEntries(entries)
}

func parseEntries(entries []string) ([]netip.Prefix, error) {
	prefixes := make([]netip.Prefix, 0, len(entries))
	for _, e := range entries {
		p, err := parseEntry(e)
		if err != nil {
			return nil, err
		}
		prefixes = append(prefixes, p)
	}
	return prefixes, nil
}

// parseEntry accepts a CIDR or a bare address. Normalising a bare address to a
// host prefix matters: chi calls netip.MustParsePrefix, which panics on input
// without a "/", so a bare IP would otherwise kill the process at startup.
func parseEntry(entry string) (netip.Prefix, error) {
	if strings.Contains(entry, "/") {
		p, err := netip.ParsePrefix(entry)
		if err != nil {
			return netip.Prefix{}, fmt.Errorf("invalid trusted proxy %q: %w", entry, err)
		}
		if p.Addr().Is4In6() {
			bits := p.Bits() - 96
			if bits < 0 {
				return netip.Prefix{}, fmt.Errorf("invalid trusted proxy %q: IPv4-mapped prefix shorter than /96", entry)
			}
			p = netip.PrefixFrom(p.Addr().Unmap(), bits)
		}
		return p.Masked(), nil
	}

	addr, err := netip.ParseAddr(entry)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("invalid trusted proxy %q: %w", entry, err)
	}
	// Peers are unmapped before the prefix check, so a v4-mapped entry kept as a
	// 128-bit prefix could never match — it would be silently ineffective.
	addr = addr.Unmap()
	return netip.PrefixFrom(addr, addr.BitLen()), nil
}
