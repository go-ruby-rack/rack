// Copyright (c) the go-ruby-rack/rack authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rack

import "strings"

// MediaTypeOf returns the media type (type/subtype) portion of a CONTENT_TYPE
// string without parameters — e.g. "text/plain;charset=utf-8" → "text/plain" —
// matching Rack::MediaType.type. An empty content type yields "".
//
// Ruby returns nil here; in Go we use the empty string to mean "no media type".
func MediaTypeOf(contentType string) string {
	if contentType == "" {
		return ""
	}
	t := splitMediaType(contentType, 2)[0]
	t = strings.TrimRight(t, " \t\r\n\v\f")
	return strings.ToLower(t)
}

// MediaTypeParams parses the parameters of a CONTENT_TYPE string into an
// ordered map — e.g. "text/plain;charset=utf-8" → {"charset" => "utf-8"} —
// matching Rack::MediaType.params. Parameter names are down-cased; a parameter
// with no value (or an empty value) maps to "". Surrounding double quotes on a
// value are stripped.
func MediaTypeParams(contentType string) *Params {
	out := NewParams()
	if contentType == "" {
		return out
	}
	parts := splitMediaType(contentType, -1)
	for _, s := range parts[1:] {
		s = strings.TrimSpace(s)
		eq := strings.IndexByte(s, '=')
		var k, v string
		if eq < 0 {
			k = s
			v = ""
		} else {
			k, v = s[:eq], s[eq+1:]
		}
		k = strings.ToLower(k)
		out.Set(k, stripDoubleQuotes(v))
	}
	return out
}

// splitMediaType splits on ';' or ',' (Rack::MediaType::SPLIT_PATTERN). limit
// of -1 means unlimited; limit of 2 caps to two fields (the type and the rest).
func splitMediaType(s string, limit int) []string {
	var out []string
	var cur strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c == ';' || c == ',') && (limit < 0 || len(out) < limit-1) {
			out = append(out, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteByte(c)
	}
	out = append(out, cur.String())
	return out
}

// stripDoubleQuotes drops a surrounding pair of double quotes, returning "" for
// an empty/absent value, matching Rack::MediaType#strip_doublequotes.
func stripDoubleQuotes(str string) string {
	if len(str) >= 2 && strings.HasPrefix(str, `"`) && strings.HasSuffix(str, `"`) {
		return str[1 : len(str)-1]
	}
	return str
}
