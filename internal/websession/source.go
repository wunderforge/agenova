// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package websession

import (
	"net"
	"net/http"
	"net/netip"
)

// loginSource does not use Forwarded or X-Forwarded-For. The optional private
// header is accepted only from the explicitly enabled plaintext loopback proxy,
// which must overwrite it after resolving the client through trusted ingress.
func (h *Handler) loginSource(r *http.Request) (string, error) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "", errSession
	}
	ip, err := netip.ParseAddr(host)
	if err != nil || ip.Zone() != "" {
		return "", errSession
	}
	if h.c.TrustLocalProxyClientIP && r.TLS == nil && ip.IsLoopback() {
		values := r.Header.Values("X-Agenova-Client-IP")
		if len(values) != 1 {
			return "", errSession
		}
		ip, err = netip.ParseAddr(values[0])
		if err != nil || ip.Zone() != "" {
			return "", errSession
		}
	}
	ip = ip.Unmap()
	// IPv6 address rotation within one network must not bypass the source cap.
	if ip.Is6() {
		return netip.PrefixFrom(ip, 64).Masked().String(), nil
	}
	return ip.String(), nil
}
