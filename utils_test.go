// Copyright (c) the go-ruby-rack/rack authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rack

import (
	"reflect"
	"testing"
)

func TestQValues(t *testing.T) {
	cases := []struct {
		in   string
		want []QValue
	}{
		{"text/html;q=0.8, application/json", []QValue{{"text/html", 0.8}, {"application/json", 1.0}}},
		{"*/*", []QValue{{"*/*", 1.0}}},
		{"text/*;q=0.5,text/html", []QValue{{"text/*", 0.5}, {"text/html", 1.0}}},
		{"audio/*; q=0.2, audio/basic", []QValue{{"audio/*", 0.2}, {"audio/basic", 1.0}}},
	}
	for _, c := range cases {
		if got := QValues(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("QValues(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestQValuesMalformedQuality(t *testing.T) {
	// A "q=" with no number, and a non-q parameter, both default to 1.0.
	got := QValues("text/html;q=, application/json;level=1")
	want := []QValue{{"text/html", 1.0}, {"application/json", 1.0}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
	// A lone "." parses to 0.0 via to_f.
	got = QValues("text/html;q=.")
	if got[0].Quality != 0.0 {
		t.Errorf("q=. quality = %v", got[0].Quality)
	}
}

func TestBestQMatch(t *testing.T) {
	if got := BestQMatch("text/html;q=0.5,application/json;q=0.9",
		[]string{"text/html", "application/json"}); got != "application/json" {
		t.Errorf("got %q", got)
	}
	if got := BestQMatch("*/*", []string{"text/html", "application/json"}); got != "text/html" {
		t.Errorf("wildcard got %q", got)
	}
	if got := BestQMatch("image/png", []string{"text/html"}); got != "" {
		t.Errorf("no match should be empty, got %q", got)
	}
	if got := BestQMatch("text/*", []string{"text/html"}); got != "text/html" {
		t.Errorf("partial wildcard got %q", got)
	}
}

func TestMimeMatch(t *testing.T) {
	if !mimeMatch("text/html", "*") || !mimeMatch("text/html", "*/*") {
		t.Error("wildcard match")
	}
	if !mimeMatch("text/html", "text/*") {
		t.Error("subtype wildcard")
	}
	if mimeMatch("text/html", "image/png") {
		t.Error("non-match")
	}
	// Non-slash forms compare case-insensitively.
	if !mimeMatch("gzip", "GZIP") {
		t.Error("non-slash eqfold")
	}
	if mimeMatch("gzip", "deflate") {
		t.Error("non-slash mismatch")
	}
}

func TestGetByteRanges(t *testing.T) {
	cases := []struct {
		spec string
		size int
		want []ByteRange
		ok   bool
	}{
		{"bytes=0-499", 1000, []ByteRange{{0, 499}}, true},
		{"bytes=500-", 1000, []ByteRange{{500, 999}}, true},
		{"bytes=-200", 1000, []ByteRange{{800, 999}}, true},
		{"bytes=0-0,-1", 1000, []ByteRange{{0, 0}, {999, 999}}, true},
		{"bytes=500-499", 1000, nil, false},
		{"", 1000, nil, false},
		{"bytes=0-1999", 1000, []ByteRange{{0, 999}}, true}, // clamp to size
	}
	for _, c := range cases {
		got, ok := GetByteRanges(c.spec, c.size, 100)
		if ok != c.ok {
			t.Errorf("GetByteRanges(%q) ok = %v, want %v", c.spec, ok, c.ok)
			continue
		}
		if ok && !reflect.DeepEqual(got, c.want) {
			t.Errorf("GetByteRanges(%q) = %v, want %v", c.spec, got, c.want)
		}
	}
}

func TestGetByteRangesEdgeCases(t *testing.T) {
	// Zero size returns nil,false.
	if _, ok := GetByteRanges("bytes=0-1", 0, 100); ok {
		t.Error("zero size should be false")
	}
	// No "bytes=" prefix.
	if _, ok := GetByteRanges("items=0-1", 100, 100); ok {
		t.Error("non-bytes unit should be false")
	}
	// Too many ranges (>= maxRanges commas).
	if _, ok := GetByteRanges("bytes=0-1,2-3,4-5", 100, 2); ok {
		t.Error("over maxRanges should be false")
	}
	// Range spec without a dash.
	if _, ok := GetByteRanges("bytes=5", 100, 100); ok {
		t.Error("missing dash should be false")
	}
	// Suffix with empty number "bytes=-".
	if _, ok := GetByteRanges("bytes=-", 100, 100); ok {
		t.Error("empty suffix should be false")
	}
	// Suffix larger than size clamps start to 0.
	got, ok := GetByteRanges("bytes=-2000", 1000, 100)
	if !ok || !reflect.DeepEqual(got, []ByteRange{{0, 999}}) {
		t.Errorf("big suffix = %v,%v", got, ok)
	}
	// Trailing ';' is trimmed.
	got, ok = GetByteRanges("bytes=0-9;junk", 1000, 100)
	if !ok || !reflect.DeepEqual(got, []ByteRange{{0, 9}}) {
		t.Errorf("semicolon = %v,%v", got, ok)
	}
	// Total ranges exceeding size yields empty slice (true).
	got, ok = GetByteRanges("bytes=0-9,0-9", 10, 100)
	if !ok || len(got) != 0 {
		t.Errorf("oversize total = %v,%v", got, ok)
	}
	// Comma + tab/space delimiter.
	got, ok = GetByteRanges("bytes=0-0,\t2-2", 100, 100)
	if !ok || !reflect.DeepEqual(got, []ByteRange{{0, 0}, {2, 2}}) {
		t.Errorf("ws delimiter = %v,%v", got, ok)
	}
}

func TestByteRangeSize(t *testing.T) {
	if (ByteRange{Start: 0, End: 499}).Size() != 500 {
		t.Error("Size")
	}
}

func TestAtoiRuby(t *testing.T) {
	cases := map[string]int{
		"123":   123,
		"  42":  42,
		"-7":    -7,
		"+9":    9,
		"12abc": 12,
		"abc":   0,
		"":      0,
	}
	for in, want := range cases {
		if got := atoiRuby(in); got != want {
			t.Errorf("atoiRuby(%q) = %d, want %d", in, got, want)
		}
	}
}
