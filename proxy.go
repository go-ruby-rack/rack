// Copyright (c) the go-ruby-rack/rack authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rack

import (
	"regexp"
	"strings"
)

// trustedProxyRE is the union of patterns Rack uses to recognise a trusted
// proxy / loopback / private address (Rack::Request.ip_filter): loopback IPv4
// 127.x, IPv6 ::1, the fc00::/fd00:: ULA range, the RFC1918 private IPv4 ranges,
// and the "localhost" hostname / unix socket forms.
var trustedProxyRE = regexp.MustCompile(`(?i)` + strings.Join([]string{
	`\A127(\.(25[0-5]|2[0-4][0-9]|[01]?[0-9]?[0-9])){3}\z`,
	`\A::1\z`,
	`\Af[cd][0-9a-f]{2}(?::[0-9a-f]{0,4}){0,7}\z`,
	`\A10(\.(25[0-5]|2[0-4][0-9]|[01]?[0-9]?[0-9])){3}\z`,
	`\A172\.(1[6-9]|2[0-9]|3[01])(\.(25[0-5]|2[0-4][0-9]|[01]?[0-9]?[0-9])){2}\z`,
	`\A192\.168(\.(25[0-5]|2[0-4][0-9]|[01]?[0-9]?[0-9])){2}\z`,
	`\Alocalhost\z|\Aunix(\z|:)`,
}, "|"))

// TrustedProxy reports whether ip is a trusted proxy / loopback / private
// address, matching Rack::Request.ip_filter. Hosts that terminate TLS behind a
// reverse proxy can rely on this when deciding which forwarded address to trust.
func TrustedProxy(ip string) bool {
	return trustedProxyRE.MatchString(ip)
}
