// Copyright (c) the go-ruby-rack/rack authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rack

import (
	"fmt"
	"strings"
)

// DefaultParamDepthLimit is the default nesting depth allowed by
// parse_nested_query, matching Rack::Utils.default_query_parser (32). It guards
// against a rogue client triggering a stack overflow.
const DefaultParamDepthLimit = 32

// ParameterTypeError is raised when nested parameters parsed by
// ParseNestedQuery contain conflicting structural types (e.g. a key used both
// as a scalar and as an array/hash). It corresponds to
// Rack::QueryParser::ParameterTypeError.
type ParameterTypeError struct{ msg string }

func (e *ParameterTypeError) Error() string { return e.msg }

// InvalidParameterError is raised when a parameter has an invalid byte sequence
// or %-encoding, corresponding to Rack::QueryParser::InvalidParameterError.
type InvalidParameterError struct{ msg string }

func (e *InvalidParameterError) Error() string { return e.msg }

// ParamsTooDeepError is raised when nested parameters exceed the configured
// depth limit, corresponding to Rack::QueryParser::ParamsTooDeepError.
type ParamsTooDeepError struct{ msg string }

func (e *ParamsTooDeepError) Error() string { return e.msg }

// splitPairs splits a query string into raw "k=v" segments on the separator
// characters (default "&"), dropping empty segments. Like Ruby's
// QueryParser#each_query_pair, the separator string is treated as a set of
// delimiter characters, each optionally followed by spaces.
func splitPairs(qs, sep string) []string {
	if qs == "" {
		return nil
	}
	if sep == "" {
		sep = "&"
	}
	delim := make(map[byte]bool, len(sep))
	for i := 0; i < len(sep); i++ {
		delim[sep[i]] = true
	}
	var out []string
	var cur strings.Builder
	flush := func() {
		out = append(out, cur.String())
		cur.Reset()
	}
	for i := 0; i < len(qs); i++ {
		c := qs[i]
		if delim[c] {
			flush()
			// Skip the run of spaces that Ruby's `<sep> */n` pattern consumes.
			for i+1 < len(qs) && qs[i+1] == ' ' {
				i++
			}
			continue
		}
		cur.WriteByte(c)
	}
	flush()
	// Drop empties (matching Ruby's `next if p.empty?`).
	filtered := out[:0]
	for _, p := range out {
		if p != "" {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// queryPair holds one decoded key/value from a query string. present reports
// whether the segment carried an '=' (so the value is meaningful) — a bare "k"
// yields a nil value in the resulting map.
type queryPair struct {
	key    string
	val    string
	hasVal bool
}

// eachQueryPair decodes each "k=v" segment, unescaping both halves. It returns
// an error if any half has an invalid %-encoding (mirroring the rescue in
// QueryParser#each_query_pair that re-raises as InvalidParameterError).
func eachQueryPair(qs, sep string) ([]queryPair, error) {
	segs := splitPairs(qs, sep)
	pairs := make([]queryPair, 0, len(segs))
	for _, p := range segs {
		eq := strings.IndexByte(p, '=')
		var rawK, rawV string
		hasVal := false
		if eq < 0 {
			rawK = p
		} else {
			rawK, rawV = p[:eq], p[eq+1:]
			hasVal = true
		}
		k, err := Unescape(rawK)
		if err != nil {
			return nil, err
		}
		v := ""
		if hasVal {
			v, err = Unescape(rawV)
			if err != nil {
				return nil, err
			}
		}
		pairs = append(pairs, queryPair{key: k, val: v, hasVal: hasVal})
	}
	return pairs, nil
}

// ParseQuery parses a query string into a *Params, collapsing repeated keys
// into a []any (in order) the way Rack::Utils.parse_query does. A bare key (no
// '=') maps to nil; an empty value maps to "". The sep argument is the delimiter
// set, defaulting to "&" when empty.
func ParseQuery(qs, sep string) (*Params, error) {
	pairs, err := eachQueryPair(qs, sep)
	if err != nil {
		return nil, err
	}
	params := NewParams()
	for _, p := range pairs {
		var v any
		if p.hasVal {
			v = p.val
		} else {
			v = nil
		}
		if cur, ok := params.Get(p.key); ok {
			if arr, isArr := cur.([]any); isArr {
				params.Set(p.key, append(arr, v))
			} else {
				params.Set(p.key, []any{cur, v})
			}
		} else {
			params.Set(p.key, v)
		}
	}
	return params, nil
}

// ParseNestedQuery parses a query string into structural types — *Params,
// []any and string — expanding foo[bar] and foo[] bracket nesting exactly like
// Rack::Utils.parse_nested_query. depthLimit caps nesting (use
// DefaultParamDepthLimit). Conflicting types raise ParameterTypeError and
// over-deep nesting raises ParamsTooDeepError.
func ParseNestedQuery(qs, sep string, depthLimit int) (*Params, error) {
	if depthLimit <= 0 {
		depthLimit = DefaultParamDepthLimit
	}
	pairs, err := eachQueryPair(qs, sep)
	if err != nil {
		return nil, err
	}
	params := NewParams()
	for _, p := range pairs {
		var v any
		if p.hasVal {
			v = p.val
		}
		if _, err := normalizeParams(params, p.key, v, 0, depthLimit); err != nil {
			return nil, err
		}
	}
	return params, nil
}

// normalizeParams is the faithful port of Rack::QueryParser#_normalize_params.
// It returns the (possibly newly created) container so the array-of-hash branch
// can collect sub-results.
func normalizeParams(params *Params, name string, v any, depth, limit int) (any, error) {
	if depth >= limit {
		return nil, &ParamsTooDeepError{msg: "exceeded available parameter key space"}
	}

	var k, after string
	switch {
	case name == "":
		// nil/empty name, treat as empty string.
		k, after = "", ""
	case depth == 0:
		// Don't treat '[' or '[]' at the very start specially.
		if start := indexFrom(name, '[', 1); start >= 0 {
			k = name[:start]
			after = name[start:]
		} else {
			k = name
			after = ""
		}
	case strings.HasPrefix(name, "[]"):
		k = "[]"
		after = name[2:]
	case strings.HasPrefix(name, "[") && indexFrom(name, ']', 1) >= 0:
		start := indexFrom(name, ']', 1)
		k = name[1:start]
		after = name[start+1:]
	default:
		// Malformed: nested but not starting with '['.
		k = name
		after = ""
	}

	if k == "" {
		return params, nil
	}

	switch {
	case after == "":
		if k == "[]" && depth != 0 {
			return []any{v}, nil
		}
		params.Set(k, v)
	case after == "[":
		params.Set(name, v)
	case after == "[]":
		cur, _ := params.Get(k)
		arr, ok := cur.([]any)
		if cur == nil {
			arr = []any{}
		} else if !ok {
			return nil, &ParameterTypeError{msg: "expected Array (got " + typeName(cur) + ") for param `" + k + "'"}
		}
		params.Set(k, append(arr, v))
	case strings.HasPrefix(after, "[]"):
		// Recognise x[][y] (hash inside array) parameters. By default the child
		// key is everything after the "[]"; only the clean "[]" + "[key]" shape
		// (a single bracketed segment, no inner brackets) uses the inner key.
		childKey := after[2:]
		if len(after) >= 4 && after[2] == '[' && strings.HasSuffix(after, "]") {
			if ck := after[3 : len(after)-1]; ck != "" && !strings.ContainsAny(ck, "[]") {
				childKey = ck
			}
		}
		cur, _ := params.Get(k)
		arr, ok := cur.([]any)
		if cur == nil {
			arr = []any{}
		} else if !ok {
			return nil, &ParameterTypeError{msg: "expected Array (got " + typeName(cur) + ") for param `" + k + "'"}
		}
		if last, isHash := lastHash(arr); isHash && !paramsHashHasKey(last, childKey) {
			if _, err := normalizeParams(last, childKey, v, depth+1, limit); err != nil {
				return nil, err
			}
		} else {
			// Ruby appends the *return value* of the recursion, not the fresh
			// make_params: when childKey is "[]" the recursion returns [v] (an
			// array literal) rather than the hash, producing a nested array.
			res, err := normalizeParams(NewParams(), childKey, v, depth+1, limit)
			if err != nil {
				return nil, err
			}
			arr = append(arr, res)
		}
		params.Set(k, arr)
	default:
		cur, ok := params.Get(k)
		var sub *Params
		if !ok || cur == nil {
			sub = NewParams()
		} else if hp, isHash := cur.(*Params); isHash {
			sub = hp
		} else {
			return nil, &ParameterTypeError{msg: "expected Hash (got " + typeName(cur) + ") for param `" + k + "'"}
		}
		res, err := normalizeParams(sub, after, v, depth+1, limit)
		if err != nil {
			return nil, err
		}
		params.Set(k, res)
	}

	return params, nil
}

// indexFrom returns the index of byte c in s at or after position from, or -1.
func indexFrom(s string, c byte, from int) int {
	if from >= len(s) {
		return -1
	}
	i := strings.IndexByte(s[from:], c)
	if i < 0 {
		return -1
	}
	return from + i
}

// lastHash returns the trailing element of arr if it is a *Params.
func lastHash(arr []any) (*Params, bool) {
	if len(arr) == 0 {
		return nil, false
	}
	h, ok := arr[len(arr)-1].(*Params)
	return h, ok
}

// paramsHashHasKey ports QueryParser#params_hash_has_key?: it reports whether
// the nested hash already contains the bracket-path key, so x[][a] knows
// whether to start a new array element.
func paramsHashHasKey(h *Params, key string) bool {
	if strings.Contains(key, "[]") {
		return false
	}
	cur := any(h)
	for _, part := range splitBracketKey(key) {
		if part == "" {
			continue
		}
		hp, ok := cur.(*Params)
		if !ok || !hp.Has(part) {
			return false
		}
		cur, _ = hp.Get(part)
	}
	return true
}

// splitBracketKey splits on runs of '[' and ']' (Ruby's key.split(/[\[\]]+/)).
func splitBracketKey(key string) []string {
	var out []string
	var cur strings.Builder
	for i := 0; i < len(key); i++ {
		if key[i] == '[' || key[i] == ']' {
			out = append(out, cur.String())
			cur.Reset()
			for i+1 < len(key) && (key[i+1] == '[' || key[i+1] == ']') {
				i++
			}
		} else {
			cur.WriteByte(key[i])
		}
	}
	out = append(out, cur.String())
	return out
}

// typeName renders the Ruby class name of a parsed value for error messages.
func typeName(v any) string {
	switch v.(type) {
	case string:
		return "String"
	case []any:
		return "Array"
	case *Params:
		return "Hash"
	case nil:
		return "NilClass"
	}
	return "Object"
}

// BuildQuery builds a flat query string from an ordered map of values, matching
// Rack::Utils.build_query: a []any value repeats the key, a nil value emits a
// bare key, and keys/values are escaped with Escape. Keys are emitted in p's
// insertion order.
func BuildQuery(p *Params) string {
	var parts []string
	p.Each(func(k string, v any) bool {
		if arr, ok := v.([]any); ok {
			// Ruby maps each element to [k, x] and recurses build_query, which
			// for an array of scalars yields "k=x" segments.
			var segs []string
			for _, x := range arr {
				segs = append(segs, buildQueryScalar(k, x))
			}
			parts = append(parts, strings.Join(segs, "&"))
		} else {
			parts = append(parts, buildQueryScalar(k, v))
		}
		return true
	})
	return strings.Join(parts, "&")
}

func buildQueryScalar(k string, v any) string {
	if v == nil {
		return Escape(k)
	}
	return Escape(k) + "=" + Escape(toStr(v))
}

// BuildNestedQuery builds a query string from nested structural types, matching
// Rack::Utils.build_nested_query. value may be a *Params (Hash), []any (Array),
// a string scalar, or nil. prefix is the key prefix ("" at the top level).
func BuildNestedQuery(value any, prefix string) (string, error) {
	// Ruby raises only at the top level, where a bare scalar (or nil giving a
	// prefix-less escape) has no key. Once nested, every recursion carries a
	// non-empty prefix, so the inner walk cannot fail — hence buildNested below
	// returns no error.
	if prefix == "" {
		switch value.(type) {
		case []any, *Params:
			// structural — fine
		default:
			return "", &ParameterTypeError{msg: "value must be a Hash"}
		}
	}
	return buildNested(value, prefix), nil
}

// buildNested is the non-erroring recursive core of BuildNestedQuery. It always
// runs with a non-empty prefix on scalar leaves, mirroring Ruby's
// build_nested_query once past the top-level guard.
func buildNested(value any, prefix string) string {
	switch val := value.(type) {
	case []any:
		segs := make([]string, 0, len(val))
		for _, v := range val {
			segs = append(segs, buildNested(v, prefix+"[]"))
		}
		return strings.Join(segs, "&")
	case *Params:
		var segs []string
		val.Each(func(k string, v any) bool {
			var p string
			if prefix != "" {
				p = prefix + "[" + k + "]"
			} else {
				p = k
			}
			if s := buildNested(v, p); s != "" {
				segs = append(segs, s)
			}
			return true
		})
		return strings.Join(segs, "&")
	case nil:
		return Escape(prefix)
	default:
		return Escape(prefix) + "=" + Escape(toStr(value))
	}
}

// toStr renders a scalar parameter value as a string. Parsed query values are
// already strings; this also accepts the handful of Go scalar types a host may
// pass to BuildQuery / BuildNestedQuery.
func toStr(v any) string {
	switch s := v.(type) {
	case string:
		return s
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}
