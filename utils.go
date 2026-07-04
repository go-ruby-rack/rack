// Copyright (c) the go-ruby-rack/rack authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rack

import (
	"crypto/subtle"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// QValue is one entry of an Accept-style header: a media/encoding value and its
// quality weight.
type QValue struct {
	Value   string
	Quality float64
}

// QValues parses a comma-separated quality-value header (e.g.
// "text/html;q=0.8, application/json") into value/quality pairs, matching
// Rack::Utils.q_values. A missing q parameter defaults to quality 1.0.
func QValues(header string) []QValue {
	var out []QValue
	for _, part := range strings.Split(header, ",") {
		segs := strings.SplitN(part, ";", 2)
		value := strings.TrimSpace(segs[0])
		quality := 1.0
		if len(segs) == 2 {
			params := strings.TrimSpace(segs[1])
			if q, ok := parseQParam(params); ok {
				quality = q
			}
		}
		out = append(out, QValue{Value: value, Quality: quality})
	}
	return out
}

// parseQParam matches Ruby's /\Aq=([\d.]+)/ against the parameter string,
// returning the parsed quality if present.
func parseQParam(params string) (float64, bool) {
	if !strings.HasPrefix(params, "q=") {
		return 0, false
	}
	rest := params[2:]
	end := 0
	for end < len(rest) && (rest[end] == '.' || (rest[end] >= '0' && rest[end] <= '9')) {
		end++
	}
	if end == 0 {
		return 0, false
	}
	// Ruby's String#to_f parses the leading numeric run and ignores the rest.
	f, err := strconv.ParseFloat(rest[:end], 64)
	if err != nil {
		// A run like "." alone is not a valid float; Ruby's to_f yields 0.0.
		return 0.0, true
	}
	return f, true
}

// BestQMatch returns the best available mime type for a quality-value Accept
// header, matching Rack::Utils.best_q_match. available is the server's set of
// concrete types; it returns "" when nothing matches.
func BestQMatch(header string, available []string) string {
	values := QValues(header)

	type match struct {
		mime    string
		quality float64
	}
	var matches []match
	for _, qv := range values {
		for _, am := range available {
			if mimeMatch(am, qv.Value) {
				matches = append(matches, match{mime: am, quality: qv.Quality})
				break
			}
		}
	}
	if len(matches) == 0 {
		return ""
	}
	// Ruby sorts ascending by (wildcards*-10 + quality) and takes .last, i.e.
	// the most specific, highest-quality match. A stable sort preserves input
	// order among equal keys, matching MRI's sort_by.
	sort.SliceStable(matches, func(i, j int) bool {
		return mimeSortKey(matches[i].mime, matches[i].quality) <
			mimeSortKey(matches[j].mime, matches[j].quality)
	})
	return matches[len(matches)-1].mime
}

// mimeSortKey reproduces Ruby's sort key (count('*') * -10) + quality.
func mimeSortKey(mime string, quality float64) float64 {
	stars := strings.Count(firstTwo(mime), "*")
	return float64(stars*-10) + quality
}

// firstTwo joins at most the first two '/'-separated segments, matching
// match.split('/', 2) in the Ruby sort_by.
func firstTwo(mime string) string {
	parts := strings.SplitN(mime, "/", 2)
	return strings.Join(parts, "/")
}

// mimeMatch reports whether available type am satisfies the requested type req,
// honouring '*' wildcards in either the type or subtype, matching the relevant
// behaviour of Rack::Mime.match?.
func mimeMatch(am, req string) bool {
	if req == "*/*" || req == "*" {
		return true
	}
	a := strings.SplitN(am, "/", 2)
	r := strings.SplitN(req, "/", 2)
	if len(a) != 2 || len(r) != 2 {
		return strings.EqualFold(am, req)
	}
	return wildEq(a[0], r[0]) && wildEq(a[1], r[1])
}

func wildEq(a, b string) bool {
	return a == "*" || b == "*" || strings.EqualFold(a, b)
}

// ByteRange is a satisfiable byte range [Start, End] (inclusive), the Go
// analogue of the Ruby Range objects get_byte_ranges returns.
type ByteRange struct {
	Start int
	End   int
}

// Size returns the number of bytes the range covers.
func (r ByteRange) Size() int { return r.End - r.Start + 1 }

// GetByteRanges parses a Range header value against a resource of the given
// size, matching Rack::Utils.get_byte_ranges. It returns (nil, false) when the
// header is missing or syntactically invalid, and an empty slice when no range
// is satisfiable. maxRanges caps the number of comma-separated ranges (use 100
// for the Rack default).
func GetByteRanges(httpRange string, size, maxRanges int) ([]ByteRange, bool) {
	if size == 0 {
		return nil, false
	}
	idx := strings.Index(httpRange, "bytes=")
	if httpRange == "" || idx < 0 {
		return nil, false
	}
	spec := httpRange[idx+len("bytes="):]
	if semi := strings.IndexByte(spec, ';'); semi >= 0 {
		spec = spec[:semi]
	}
	if strings.Count(spec, ",") >= maxRanges {
		return nil, false
	}
	ranges := []ByteRange{}
	for _, rangeSpec := range splitRangeSpecs(spec) {
		if !strings.Contains(rangeSpec, "-") {
			return nil, false
		}
		dash := strings.SplitN(rangeSpec, "-", 2)
		r0s, r1s := dash[0], dash[1]
		var r0, r1 int
		if r0s == "" {
			// suffix-byte-range-spec
			if r1s == "" {
				return nil, false
			}
			r0 = size - atoiRuby(r1s)
			if r0 < 0 {
				r0 = 0
			}
			r1 = size - 1
		} else {
			r0 = atoiRuby(r0s)
			if r1s == "" {
				r1 = size - 1
			} else {
				r1 = atoiRuby(r1s)
				if r1 < r0 {
					return nil, false // backwards range is invalid
				}
				if r1 >= size {
					r1 = size - 1
				}
			}
		}
		if r0 <= r1 {
			ranges = append(ranges, ByteRange{Start: r0, End: r1})
		}
	}
	total := 0
	for _, r := range ranges {
		total += r.Size()
	}
	if total > size {
		return []ByteRange{}, true
	}
	return ranges, true
}

// splitRangeSpecs splits on "," optionally followed by spaces/tabs, matching
// Ruby's byte_range.split(/,[ \t]*/).
func splitRangeSpecs(spec string) []string {
	var out []string
	var cur strings.Builder
	for i := 0; i < len(spec); i++ {
		if spec[i] == ',' {
			out = append(out, cur.String())
			cur.Reset()
			for i+1 < len(spec) && (spec[i+1] == ' ' || spec[i+1] == '\t') {
				i++
			}
			continue
		}
		cur.WriteByte(spec[i])
	}
	out = append(out, cur.String())
	return out
}

// atoiRuby parses the leading integer of s the way Ruby's String#to_i does,
// returning 0 for a non-numeric prefix.
func atoiRuby(s string) int {
	s = strings.TrimLeft(s, " \t")
	i, neg := 0, false
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		neg = s[i] == '-'
		i++
	}
	n := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		n = n*10 + int(s[i]-'0')
		i++
	}
	if neg {
		return -n
	}
	return n
}

// nullByte is the byte ValidPath rejects, matching Rack::Utils::NULL_BYTE.
const nullByte = "\x00"

// ValidPath reports whether path is a valid request path, matching
// Rack::Utils.valid_path?: the bytes must be valid UTF-8 and contain no NUL.
func ValidPath(path string) bool {
	return utf8.ValidString(path) && !strings.Contains(path, nullByte)
}

// CleanPathInfo canonicalises a PATH_INFO the way Rack::Utils.clean_path_info
// does: it splits on '/', drops empty and "." segments, pops the last kept
// segment for each "..", and rejoins with '/'. A leading '/' is restored when
// the input was empty or began with a separator. This is the traversal-safe
// normalisation Rack::Files and the static middleware rely on.
func CleanPathInfo(pathInfo string) string {
	parts := splitPathSeps(pathInfo)
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		switch part {
		case "", ".":
			// skip
		case "..":
			if len(clean) > 0 {
				clean = clean[:len(clean)-1]
			}
		default:
			clean = append(clean, part)
		}
	}
	cleanPath := strings.Join(clean, "/")
	if len(parts) == 0 || parts[0] == "" {
		cleanPath = "/" + cleanPath
	}
	return cleanPath
}

// splitPathSeps splits on single '/' characters with Ruby String#split
// semantics: trailing empty fields are dropped, but leading and interior empty
// fields (from "//" or a leading "/") are preserved. Rack's PATH_SEPS is the
// bare "/" separator on POSIX hosts.
func splitPathSeps(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, "/")
	// Ruby's split drops trailing empty strings.
	end := len(parts)
	for end > 0 && parts[end-1] == "" {
		end--
	}
	return parts[:end]
}

// SecureCompare reports whether a and b are equal using a constant-time
// comparison, matching Rack::Utils.secure_compare. It returns false immediately
// when the lengths differ (as MRI does before the fixed-length compare), so it
// leaks length but not content, and is safe for comparing secrets such as CSRF
// tokens.
func SecureCompare(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// SelectBestEncoding picks the best available content-coding for an
// Accept-Encoding preference list, matching Rack::Utils.select_best_encoding.
// available is the server's ordered coding list (its order breaks quality ties);
// accept is the parsed Accept-Encoding as value/quality pairs (typically from
// [QValues]). Only the first 16 accept entries are considered. A "*" expands to
// every available coding not named explicitly; "identity" is always an implicit
// candidate unless disqualified by a q=0. It returns ("", false) when nothing is
// acceptable (the Ruby nil).
func SelectBestEncoding(available []string, accept []QValue) (string, bool) {
	if len(accept) > 16 {
		accept = accept[:16]
	}
	type cand struct {
		enc  string
		q    float64
		pref int
	}
	prefOf := func(enc string) int {
		for i, a := range available {
			if a == enc {
				return i
			}
		}
		return len(available)
	}
	named := make(map[string]bool, len(accept))
	for _, qv := range accept {
		named[qv.Value] = true
	}
	var expanded []cand
	wildcardSeen := false
	for _, qv := range accept {
		pref := prefOf(qv.Value)
		if qv.Value == "*" {
			if !wildcardSeen {
				for _, m2 := range available {
					if !named[m2] {
						expanded = append(expanded, cand{m2, qv.Quality, pref})
					}
				}
				wildcardSeen = true
			}
			continue
		}
		expanded = append(expanded, cand{qv.Value, qv.Quality, pref})
	}
	sorted := make([]cand, len(expanded))
	copy(sorted, expanded)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].q != sorted[j].q {
			return sorted[i].q > sorted[j].q // higher quality first
		}
		return sorted[i].pref < sorted[j].pref // then server preference order
	})
	candidates := make([]string, 0, len(sorted)+1)
	for _, c := range sorted {
		candidates = append(candidates, c.enc)
	}
	if !containsStr(candidates, "identity") {
		candidates = append(candidates, "identity")
	}
	// A q=0 disqualifies a coding entirely (delete every occurrence).
	for _, c := range expanded {
		if c.q == 0.0 {
			candidates = deleteStr(candidates, c.enc)
		}
	}
	availSet := make(map[string]bool, len(available))
	for _, a := range available {
		availSet[a] = true
	}
	for _, c := range candidates {
		if availSet[c] {
			return c, true
		}
	}
	return "", false
}

func containsStr(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func deleteStr(xs []string, s string) []string {
	out := xs[:0]
	for _, x := range xs {
		if x != s {
			out = append(out, x)
		}
	}
	return out
}

// allowedForwardedParams is the set of Forwarded header parameters Rack accepts,
// matching Rack::Utils::ALLOWED_FORWARED_PARAMS. Any other parameter aborts the
// whole parse.
var allowedForwardedParams = map[string]bool{"by": true, "for": true, "host": true, "proto": true}

const (
	forwardedMaxParams  = 1024
	forwardedMaxEscapes = 1024
)

// ForwardedValues parses an RFC 7239 Forwarded header into its parameter lists,
// matching Rack::Utils.forwarded_values. It returns (nil, false) when the header
// is absent, contains a disallowed parameter, or exceeds the DoS guards (more
// than 1024 parameters or 1024 quoted escapes); otherwise it returns a map from
// the lower-cased parameter name ("by", "for", "host", "proto") to the ordered
// list of its values. present is false exactly when the Ruby method returns nil.
func ForwardedValues(forwardedHeader string, hasHeader bool) (map[string][]string, bool) {
	if !hasHeader {
		return nil, false
	}
	header := strings.ReplaceAll(forwardedHeader, "\n", ";")
	header = trimLeftSet(header, " \t;,")
	params := map[string][]string{}
	numParams, numEscapes := 0, 0
	for {
		eq := strings.IndexByte(header, '=')
		if eq < 0 {
			break
		}
		numParams++
		if numParams > forwardedMaxParams {
			return nil, false
		}
		param := header[:eq]
		header = header[eq+1:]
		param = strings.TrimSpace(param)
		param = strings.ToLower(param)
		if !allowedForwardedParams[param] {
			return nil, false
		}
		var value string
		if len(header) > 0 && header[0] == '"' {
			header = header[1:]
			var b strings.Builder
			for {
				i := strings.IndexAny(header, "\"\\")
				if i < 0 {
					// Unterminated quote: MRI's `while i = header.index(...)`
					// loop simply stops when no closing quote or escape
					// remains. The accumulated value is kept as-is and the
					// unconsumed remainder stays in header — it is NOT folded
					// into the value. Leaving it in header reproduces MRI's
					// behaviour where the leftover either yields nothing more
					// (no further '=') or forms a bogus parameter name that
					// aborts the whole parse to nil.
					break
				}
				c := header[i]
				b.WriteString(header[:i])
				header = header[i+1:]
				if c == '"' {
					break
				}
				numEscapes++
				if numEscapes > forwardedMaxEscapes {
					return nil, false
				}
				if len(header) > 0 {
					b.WriteByte(header[0])
					header = header[1:]
				}
			}
			value = b.String()
		} else if i := strings.IndexAny(header, ";,"); i >= 0 {
			value = header[:i]
			value = trimRightSet(value, " \t;,")
			header = header[i:]
			value = strings.TrimLeft(value, " \t")
		} else {
			header = strings.TrimSpace(header)
			value = header
			header = ""
			value = strings.TrimLeft(value, " \t")
		}
		params[param] = append(params[param], value)
		if header != "" {
			header = trimLeftSet(header, " \t;,")
		}
	}
	return params, true
}

// trimLeftSet trims any leading bytes contained in set.
func trimLeftSet(s, set string) string {
	i := 0
	for i < len(s) && strings.IndexByte(set, s[i]) >= 0 {
		i++
	}
	return s[i:]
}

// trimRightSet trims any trailing bytes contained in set.
func trimRightSet(s, set string) string {
	j := len(s)
	for j > 0 && strings.IndexByte(set, s[j-1]) >= 0 {
		j--
	}
	return s[:j]
}
