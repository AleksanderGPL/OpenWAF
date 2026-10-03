package api

import (
	"fmt"
	"net/netip"
	"strings"

	"github.com/gofiber/fiber/v3"
)

func ClientIPKey(proxies []string) (func(fiber.Ctx) string, error) {
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
	return func(c fiber.Ctx) string {
		peer := c.RequestCtx().RemoteIP().String()
		addr, err := netip.ParseAddr(peer)
		if err != nil {
			return peer
		}
		addr = addr.Unmap()
		var chain []string
		for _, value := range c.Request().Header.PeekAll("X-Forwarded-For") {
			chain = append(chain, strings.Split(string(value), ",")...)
		}
		for i := len(chain) - 1; i >= 0 && trustedAddress(addr, trusted); i-- {
			next, err := netip.ParseAddr(strings.TrimSpace(chain[i]))
			if err != nil || next.Zone() != "" {
				return peer
			}
			addr = next.Unmap()
		}
		return addr.String()
	}, nil
}

func trustedAddress(addr netip.Addr, proxies []netip.Prefix) bool {
	for _, proxy := range proxies {
		if proxy.Contains(addr) {
			return true
		}
	}
	return false
}
