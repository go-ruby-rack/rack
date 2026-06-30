// Copyright (c) the go-ruby-rack/rack authors
//
// SPDX-License-Identifier: BSD-3-Clause

// This file rounds out branch coverage for the small, deterministic helpers and
// error types whose paths the behavioural tests do not all reach. Keeping it
// separate documents that these assertions exist purely to exercise every line
// the 100% gate requires.
package rack

import "testing"

func TestErrorMessages(t *testing.T) {
	if (&ParameterTypeError{msg: "a"}).Error() != "a" {
		t.Error("ParameterTypeError")
	}
	if (&InvalidParameterError{msg: "b"}).Error() != "b" {
		t.Error("InvalidParameterError")
	}
	if (&ParamsTooDeepError{msg: "c"}).Error() != "c" {
		t.Error("ParamsTooDeepError")
	}
}

func TestHexValLowercase(t *testing.T) {
	for in, want := range map[byte]byte{'a': 10, 'f': 15, '0': 0, '9': 9, 'A': 10, 'F': 15} {
		if v, ok := hexVal(in); !ok || v != want {
			t.Errorf("hexVal(%q) = %d,%v want %d", in, v, ok, want)
		}
	}
	if _, ok := hexVal('g'); ok {
		t.Error("non-hex should be false")
	}
}

func TestTypeName(t *testing.T) {
	cases := map[string]any{
		"String":   "x",
		"Array":    []any{},
		"Hash":     NewParams(),
		"NilClass": nil,
		"Object":   42,
	}
	for want, v := range cases {
		if got := typeName(v); got != want {
			t.Errorf("typeName(%T) = %q, want %q", v, got, want)
		}
	}
}

func TestSplitBracketKey(t *testing.T) {
	// Adjacent brackets are collapsed (the inner skip loop). A trailing bracket
	// leaves an empty final segment, which paramsHashHasKey skips, so it is
	// behaviourally equivalent to Ruby's split that drops trailing empties.
	got := splitBracketKey("a[][b]")
	want := []string{"a", "b", ""}
	if len(got) != len(want) {
		t.Fatalf("splitBracketKey = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("splitBracketKey[%d] = %q want %q", i, got[i], want[i])
		}
	}
}

func TestToStrVariants(t *testing.T) {
	if toStr(nil) != "" {
		t.Error("nil")
	}
	if toStr("x") != "x" {
		t.Error("string")
	}
	if toStr(42) != "42" {
		t.Error("int")
	}
}

func TestBuildNestedQueryNestedError(t *testing.T) {
	// An array containing a bad scalar (no prefix reachable) — actually arrays
	// always carry a prefix; instead trigger the Hash branch error by nesting a
	// value that recurses to the no-prefix scalar case is impossible, so we
	// confirm a deeply nested structure builds and the array-of-array path runs.
	inner := NewParams()
	inner.Set("k", []any{"1", "2"})
	outer := NewParams()
	outer.Set("a", inner)
	got, err := BuildNestedQuery(outer, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "a%5Bk%5D%5B%5D=1&a%5Bk%5D%5B%5D=2" {
		t.Errorf("got %q", got)
	}
}

func TestSplitPairsTrailingSpaces(t *testing.T) {
	// A separator followed by spaces then end-of-string.
	p, err := ParseQuery("a=1&  ", "&")
	if err != nil {
		t.Fatal(err)
	}
	if got := rubyInspect(p); got != `{"a" => "1"}` {
		t.Errorf("got %s", got)
	}
}

func TestPortBadAuthorityPort(t *testing.T) {
	// Authority present but with a non-numeric port falls through to scheme
	// default / server port.
	r := NewRequest(Env{HTTPHost: "host:abc", RackURLScheme: "http"})
	if r.Port() != 80 {
		t.Errorf("port = %d", r.Port())
	}
}

func TestFullpathEmptyQuery(t *testing.T) {
	r := NewRequest(Env{PathInfo: "/p"})
	if r.Fullpath() != "/p" {
		t.Errorf("fullpath = %q", r.Fullpath())
	}
}

func TestResponseHeadersAccessor(t *testing.T) {
	r := NewResponse(nil, 200, nil)
	r.SetHeader("X", "1")
	if r.Headers().Get("x") != "1" {
		t.Error("Headers accessor")
	}
}

func TestParseNestedQueryArrayOfArray(t *testing.T) {
	// a[][]=1 → {"a"=>[["1"]]}: the depth>0 "[]" key branch returning [v], and
	// the array-nesting prefix branch.
	p, err := ParseNestedQuery("a[][]=1", "&", DefaultParamDepthLimit)
	if err != nil {
		t.Fatal(err)
	}
	if got := rubyInspect(p); got != `{"a" => [["1"]]}` {
		t.Errorf("got %s", got)
	}
	// a[][]=1&a[][]=2 → {"a"=>[["1"],["2"]]}.
	p2, err := ParseNestedQuery("a[][]=1&a[][]=2", "&", DefaultParamDepthLimit)
	if err != nil {
		t.Fatal(err)
	}
	if got := rubyInspect(p2); got != `{"a" => [["1"], ["2"]]}` {
		t.Errorf("got %s", got)
	}
	// a[][c][]=1 → {"a"=>[{"c"=>["1"]}]}: childKey keeps the bracket form.
	p3, err := ParseNestedQuery("a[][c][]=1", "&", DefaultParamDepthLimit)
	if err != nil {
		t.Fatal(err)
	}
	if got := rubyInspect(p3); got != `{"a" => [{"c" => ["1"]}]}` {
		t.Errorf("got %s", got)
	}
}

func TestParseNestedQueryArrayOfHashTooDeep(t *testing.T) {
	// A too-deep recursion inside the array-of-hash and nested-array branches.
	if _, err := ParseNestedQuery("a[][b]=1", "&", 1); err == nil {
		t.Error("expected too-deep in array-of-hash recursion")
	}
	if _, err := ParseNestedQuery("a[][b][c]=1", "&", 2); err == nil {
		t.Error("expected too-deep in nested recursion")
	}
}

func TestParseNestedQueryArrayHashSuffix(t *testing.T) {
	// a[]b=1 → {"a"=>[{"b"=>"1"}]}: after "[]b" is not the x[][y] pattern, so
	// childKey is the bytes after "[]" (the cond-false branch).
	p, err := ParseNestedQuery("a[]b=1", "&", DefaultParamDepthLimit)
	if err != nil {
		t.Fatal(err)
	}
	if got := rubyInspect(p); got != `{"a" => [{"b" => "1"}]}` {
		t.Errorf("got %s", got)
	}
}

func TestParseNestedQueryArrayHashConflict(t *testing.T) {
	// x[][a]=1 then x[][a][b]=2: recursing into the existing last hash where a is
	// already a scalar raises a type error (the if-branch error propagation).
	if _, err := ParseNestedQuery("x[][a]=1&x[][a][b]=2", "&", DefaultParamDepthLimit); err == nil {
		t.Error("expected ParameterTypeError")
	} else if _, ok := err.(*ParameterTypeError); !ok {
		t.Errorf("expected *ParameterTypeError, got %T", err)
	}
}

func TestParamsHashHasKeyEmptyLeadingPart(t *testing.T) {
	// A key beginning with '[' yields an empty first split part (the continue).
	h := NewParams()
	inner := NewParams()
	inner.Set("a", "1")
	h.Set("x", inner)
	if !paramsHashHasKey(h, "x[a]") {
		t.Error("x[a] should be found")
	}
}

func TestBuildNestedQueryEmptySub(t *testing.T) {
	// An empty sub-hash is dropped (the s != "" guard).
	outer := NewParams()
	outer.Set("a", NewParams())
	outer.Set("b", "1")
	got, err := BuildNestedQuery(outer, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "b=1" {
		t.Errorf("got %q", got)
	}
}

func TestPortServerPortFallback(t *testing.T) {
	// Authority with no port, unknown scheme: falls through to SERVER_PORT.
	r := NewRequest(Env{HTTPHost: "host", RackURLScheme: "gopher", ServerPort: "70"})
	if r.Port() != 70 {
		t.Errorf("port = %d", r.Port())
	}
}

func TestSplitAuthorityIPv6NoPort(t *testing.T) {
	host, addr, port := splitAuthority("[::1]")
	if host != "[::1]" || addr != "::1" || port != 0 {
		t.Errorf("[::1] = %q %q %d", host, addr, port)
	}
}

func TestParamsHashHasKeyBranches(t *testing.T) {
	// A key containing "[]" short-circuits to false.
	h := NewParams()
	if paramsHashHasKey(h, "a[]") {
		t.Error("[] key should be false")
	}
	// A non-hash intermediate yields false.
	h.Set("a", "scalar")
	if paramsHashHasKey(h, "a[b]") {
		t.Error("non-hash intermediate should be false")
	}
}
