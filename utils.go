// Copyright (c) the go-ruby-rack/rack authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rack

import (
	"sort"
	"strconv"
	"strings"
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
