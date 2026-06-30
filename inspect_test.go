// Copyright (c) the go-ruby-rack/rack authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rack

import (
	"sort"
	"strconv"
	"strings"
)

// rubyInspect renders a parsed value as the Ruby `inspect` / `p` string MRI
// prints, so the oracle tests can compare byte-for-byte against `ruby`. Hash
// keys are emitted in insertion order to match Ruby's ordered Hash. It supports
// the value model parse_query / parse_nested_query produce: *Params, []any,
// string and nil.
func rubyInspect(v any) string {
	switch x := v.(type) {
	case nil:
		return "nil"
	case string:
		return rubyStr(x)
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			parts[i] = rubyInspect(e)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case *Params:
		var parts []string
		x.Each(func(k string, val any) bool {
			parts = append(parts, rubyStr(k)+" => "+rubyInspect(val))
			return true
		})
		return "{" + strings.Join(parts, ", ") + "}"
	}
	return "nil"
}

// rubyStr renders a Go string as a Ruby double-quoted string literal, escaping
// the characters MRI's String#inspect escapes that appear in our test corpus.
func rubyStr(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// sortedKeys returns a Params' keys sorted, used by tests that need a stable
// order independent of insertion (e.g. comparing a plain Go map build).
func sortedKeys(p *Params) []string {
	ks := p.Keys()
	sort.Strings(ks)
	return ks
}

// itoa is a tiny helper used in table-driven tests.
func itoa(i int) string { return strconv.Itoa(i) }
