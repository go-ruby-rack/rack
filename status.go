// Copyright (c) the go-ruby-rack/rack authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rack

import (
	"sort"
	"strings"
)

// HTTPStatusCodes maps every standard HTTP status code to its reason phrase,
// matching Rack::Utils::HTTP_STATUS_CODES.
var HTTPStatusCodes = map[int]string{
	100: "Continue",
	101: "Switching Protocols",
	102: "Processing",
	103: "Early Hints",
	200: "OK",
	201: "Created",
	202: "Accepted",
	203: "Non-Authoritative Information",
	204: "No Content",
	205: "Reset Content",
	206: "Partial Content",
	207: "Multi-Status",
	208: "Already Reported",
	226: "IM Used",
	300: "Multiple Choices",
	301: "Moved Permanently",
	302: "Found",
	303: "See Other",
	304: "Not Modified",
	305: "Use Proxy",
	307: "Temporary Redirect",
	308: "Permanent Redirect",
	400: "Bad Request",
	401: "Unauthorized",
	402: "Payment Required",
	403: "Forbidden",
	404: "Not Found",
	405: "Method Not Allowed",
	406: "Not Acceptable",
	407: "Proxy Authentication Required",
	408: "Request Timeout",
	409: "Conflict",
	410: "Gone",
	411: "Length Required",
	412: "Precondition Failed",
	413: "Content Too Large",
	414: "URI Too Long",
	415: "Unsupported Media Type",
	416: "Range Not Satisfiable",
	417: "Expectation Failed",
	421: "Misdirected Request",
	422: "Unprocessable Content",
	423: "Locked",
	424: "Failed Dependency",
	425: "Too Early",
	426: "Upgrade Required",
	428: "Precondition Required",
	429: "Too Many Requests",
	431: "Request Header Fields Too Large",
	451: "Unavailable For Legal Reasons",
	500: "Internal Server Error",
	501: "Not Implemented",
	502: "Bad Gateway",
	503: "Service Unavailable",
	504: "Gateway Timeout",
	505: "HTTP Version Not Supported",
	506: "Variant Also Negotiates",
	507: "Insufficient Storage",
	508: "Loop Detected",
	511: "Network Authentication Required",
}

// symbolToStatusCode is the reverse map: a normalised reason-phrase symbol
// (lower-cased, spaces and hyphens to underscores) to its code, matching
// Rack::Utils::SYMBOL_TO_STATUS_CODE.
var symbolToStatusCode = func() map[string]int {
	m := make(map[string]int, len(HTTPStatusCodes))
	for code, msg := range HTTPStatusCodes {
		m[statusSymbol(msg)] = code
	}
	return m
}()

// obsoleteSymbolToStatusCode carries the deprecated symbol aliases Ruby still
// resolves (with a warning) in Rack::Utils.status_code.
var obsoleteSymbolToStatusCode = map[string]int{
	"payload_too_large":        413,
	"unprocessable_entity":     422,
	"bandwidth_limit_exceeded": 509,
	"not_extended":             510,
}

// statusSymbol normalises a reason phrase to its symbol form: downcase and
// replace each space or hyphen with an underscore.
func statusSymbol(msg string) string {
	var b strings.Builder
	b.Grow(len(msg))
	for i := 0; i < len(msg); i++ {
		c := msg[i]
		switch {
		case c == ' ' || c == '-':
			b.WriteByte('_')
		case c >= 'A' && c <= 'Z':
			b.WriteByte(c + ('a' - 'A'))
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// SymbolToStatusCode returns the HTTP code for a status symbol (e.g.
// "not_found" → 404), resolving the deprecated aliases too. ok is false for an
// unrecognised symbol, mirroring how Rack::Utils.status_code raises
// ArgumentError in that case.
func SymbolToStatusCode(sym string) (code int, ok bool) {
	if c, found := symbolToStatusCode[sym]; found {
		return c, true
	}
	if c, found := obsoleteSymbolToStatusCode[sym]; found {
		return c, true
	}
	return 0, false
}

// StatusWithNoEntityBody reports whether a response with the given status code
// must not carry an entity body — the 1xx range plus 204 and 304 — matching
// Rack::Utils::STATUS_WITH_NO_ENTITY_BODY. This is the predicate form the
// `status_with_no_entity_body?` helper exposes.
func StatusWithNoEntityBody(status int) bool {
	return (status >= 100 && status <= 199) || status == 204 || status == 304
}

// statusCodesSorted returns the status codes in ascending order, used where
// deterministic iteration over HTTPStatusCodes is needed.
func statusCodesSorted() []int {
	codes := make([]int, 0, len(HTTPStatusCodes))
	for c := range HTTPStatusCodes {
		codes = append(codes, c)
	}
	sort.Ints(codes)
	return codes
}
