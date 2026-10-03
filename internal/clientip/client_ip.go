package clientip

import (
	"fmt"
	"net/netip"
	"strings"
)

type Resolver struct{ trusted []netip.Prefix }

func New(proxies []string) (*Resolver, error) {
	trusted := make([]netip.Prefix, 0, len(proxies))
	for _, value := range proxies {
		value = strings.TrimSpace(value)
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			addr, err := netip.ParseAddr(value)
			if err != nil || addr.Zone() != "" {
				return nil, fmt.Errorf("invalid trusted proxy %q: expected an IP address or CIDR", value)
			}
			addr = addr.Unmap()
			prefix = netip.PrefixFrom(addr, addr.BitLen())
		}
		trusted = append(trusted, prefix.Masked())
	}
	return &Resolver{trusted: trusted}, nil
}

func (r *Resolver) IP(peer string, forwarded []string) string {
	addr, err := netip.ParseAddr(peer)
	if err != nil {
		return peer
	}
	addr = addr.Unmap()
	var chain []string
	for _, value := range forwarded {
		chain = append(chain, strings.Split(value, ",")...)
	}
	for i := len(chain) - 1; i >= 0 && trustedAddress(addr, r.trusted); i-- {
		next, err := netip.ParseAddr(strings.TrimSpace(chain[i]))
		if err != nil || next.Zone() != "" {
			return peer
		}
		addr = next.Unmap()
	}
	return addr.String()
}

func trustedAddress(addr netip.Addr, proxies []netip.Prefix) bool {
	for _, proxy := range proxies {
		if proxy.Contains(addr) {
			return true
		}
	}
	return false
}
