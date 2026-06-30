// Copyright (c) the go-ruby-rack/rack authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rack

import "testing"

func TestParseQuery(t *testing.T) {
	cases := map[string]string{
		"a=1&a=2":         `{"a" => ["1", "2"]}`,
		"foo=bar&baz=qux": `{"foo" => "bar", "baz" => "qux"}`,
		"a":               `{"a" => nil}`,
		"a=":              `{"a" => ""}`,
		"=b":              `{"" => "b"}`,
		"a%20b=c%20d":     `{"a b" => "c d"}`,
		"":                `{}`,
		"a=1&a=2&a=3":     `{"a" => ["1", "2", "3"]}`,
	}
	for qs, want := range cases {
		p, err := ParseQuery(qs, "&")
		if err != nil {
			t.Errorf("ParseQuery(%q) error %v", qs, err)
			continue
		}
		if got := rubyInspect(p); got != want {
			t.Errorf("ParseQuery(%q) = %s, want %s", qs, got, want)
		}
	}
}

func TestParseQuerySeparators(t *testing.T) {
	p, err := ParseQuery("a=1;b=2", ";")
	if err != nil {
		t.Fatal(err)
	}
	if got := rubyInspect(p); got != `{"a" => "1", "b" => "2"}` {
		t.Errorf("got %s", got)
	}
	// Default separator when empty string passed.
	p2, _ := ParseQuery("a=1&b=2", "")
	if got := rubyInspect(p2); got != `{"a" => "1", "b" => "2"}` {
		t.Errorf("default sep got %s", got)
	}
}

func TestParseQueryInvalid(t *testing.T) {
	if _, err := ParseQuery("a=%ZZ", "&"); err == nil {
		t.Error("expected error on invalid escape")
	}
	if _, err := ParseQuery("%ZZ=b", "&"); err == nil {
		t.Error("expected error on invalid key escape")
	}
}

func TestParseNestedQuery(t *testing.T) {
	cases := map[string]string{
		"foo=bar":                 `{"foo" => "bar"}`,
		"foo[]=1&foo[]=2":         `{"foo" => ["1", "2"]}`,
		"foo[bar]=baz":            `{"foo" => {"bar" => "baz"}}`,
		"a[b][c]=d":               `{"a" => {"b" => {"c" => "d"}}}`,
		"foo[]=1&foo[]=2&foo[]=3": `{"foo" => ["1", "2", "3"]}`,
		"x[][a]=1&x[][b]=2":       `{"x" => [{"a" => "1", "b" => "2"}]}`,
		"x[][a]=1&x[][a]=2":       `{"x" => [{"a" => "1"}, {"a" => "2"}]}`,
		"a=1&a=2":                 `{"a" => "2"}`,
		"key":                     `{"key" => nil}`,
		"=val":                    `{}`,
		"foo=":                    `{"foo" => ""}`,
		"a[b]=c&a[d]=e":           `{"a" => {"b" => "c", "d" => "e"}}`,
		"":                        `{}`,
	}
	for qs, want := range cases {
		p, err := ParseNestedQuery(qs, "&", DefaultParamDepthLimit)
		if err != nil {
			t.Errorf("ParseNestedQuery(%q) error %v", qs, err)
			continue
		}
		if got := rubyInspect(p); got != want {
			t.Errorf("ParseNestedQuery(%q) = %s, want %s", qs, got, want)
		}
	}
}

func TestParseNestedQueryDefaultDepth(t *testing.T) {
	// depthLimit <= 0 falls back to DefaultParamDepthLimit.
	p, err := ParseNestedQuery("a[b]=c", "&", 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := rubyInspect(p); got != `{"a" => {"b" => "c"}}` {
		t.Errorf("got %s", got)
	}
}

func TestParseNestedQueryTrailingBracket(t *testing.T) {
	// The "after == '['" branch: a name ending in a lone '['.
	p, err := ParseNestedQuery("a[=1", "&", DefaultParamDepthLimit)
	if err != nil {
		t.Fatal(err)
	}
	if got := rubyInspect(p); got != `{"a[" => "1"}` {
		t.Errorf("got %s", got)
	}
}

func TestParseNestedQueryArrayOfHashNonHashLast(t *testing.T) {
	// x[][a]=1&x[]=2: second element is a scalar appended to the array, then
	// x[][b] starts a new hash because the last element is not a hash.
	p, err := ParseNestedQuery("x[][a]=1&x[][a]=2&x[][b]=3", "&", DefaultParamDepthLimit)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"x" => [{"a" => "1"}, {"a" => "2", "b" => "3"}]}`
	if got := rubyInspect(p); got != want {
		t.Errorf("got %s want %s", got, want)
	}
}

func TestParseNestedQueryTypeConflict(t *testing.T) {
	// foo=bar then foo[]=baz: a scalar key reused as an array.
	if _, err := ParseNestedQuery("foo=bar&foo[]=baz", "&", DefaultParamDepthLimit); err == nil {
		t.Error("expected ParameterTypeError (array)")
	} else if _, ok := err.(*ParameterTypeError); !ok {
		t.Errorf("expected *ParameterTypeError, got %T", err)
	}
	// foo=bar then foo[x]=baz: a scalar key reused as a hash.
	if _, err := ParseNestedQuery("foo=bar&foo[x]=baz", "&", DefaultParamDepthLimit); err == nil {
		t.Error("expected ParameterTypeError (hash)")
	}
	// foo=bar then foo[][x]=baz: a scalar key reused as array-of-hash.
	if _, err := ParseNestedQuery("foo=bar&foo[][x]=baz", "&", DefaultParamDepthLimit); err == nil {
		t.Error("expected ParameterTypeError (array of hash)")
	}
}

func TestParseNestedQueryTooDeep(t *testing.T) {
	if _, err := ParseNestedQuery("a[b][c]=d", "&", 2); err == nil {
		t.Error("expected ParamsTooDeepError")
	} else if _, ok := err.(*ParamsTooDeepError); !ok {
		t.Errorf("expected *ParamsTooDeepError, got %T", err)
	}
}

func TestParseNestedQueryInvalidEscape(t *testing.T) {
	if _, err := ParseNestedQuery("a=%ZZ", "&", DefaultParamDepthLimit); err == nil {
		t.Error("expected error")
	}
}

func TestParseNestedQueryNestedArrayInArray(t *testing.T) {
	// x[][][y] style: childKey keeps the bracket form (the inner if-else's
	// "else" path where childKey = after[2:]).
	p, err := ParseNestedQuery("a[][b][]=1", "&", DefaultParamDepthLimit)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"a" => [{"b" => ["1"]}]}`
	if got := rubyInspect(p); got != want {
		t.Errorf("got %s want %s", got, want)
	}
}

func TestBuildQuery(t *testing.T) {
	mk := func(pairs ...any) *Params {
		p := NewParams()
		for i := 0; i < len(pairs); i += 2 {
			p.Set(pairs[i].(string), pairs[i+1])
		}
		return p
	}
	cases := []struct {
		in   *Params
		want string
	}{
		{mk("a", "1", "b", "2"), "a=1&b=2"},
		{mk("a", []any{"1", "2"}), "a=1&a=2"},
		{mk("q", "a b", "x", nil), "q=a+b&x"},
		{mk("k", "v&w"), "k=v%26w"},
		{NewParams(), ""},
	}
	for _, c := range cases {
		if got := BuildQuery(c.in); got != c.want {
			t.Errorf("BuildQuery = %q, want %q", got, c.want)
		}
	}
}

func TestBuildNestedQuery(t *testing.T) {
	mk := func(pairs ...any) *Params {
		p := NewParams()
		for i := 0; i < len(pairs); i += 2 {
			p.Set(pairs[i].(string), pairs[i+1])
		}
		return p
	}
	cases := []struct {
		in   any
		want string
	}{
		{mk("a", "1"), "a=1"},
		{mk("a", []any{"1", "2"}), "a%5B%5D=1&a%5B%5D=2"},
		{mk("a", mk("b", "c")), "a%5Bb%5D=c"},
		{mk("a", mk("b", []any{"1", "2"})), "a%5Bb%5D%5B%5D=1&a%5Bb%5D%5B%5D=2"},
		{mk("a", nil), "a"},
	}
	for _, c := range cases {
		got, err := BuildNestedQuery(c.in, "")
		if err != nil {
			t.Errorf("BuildNestedQuery error %v", err)
			continue
		}
		if got != c.want {
			t.Errorf("BuildNestedQuery = %q, want %q", got, c.want)
		}
	}
}

func TestBuildNestedQueryScalarNoPrefix(t *testing.T) {
	// A bare scalar with no prefix is an error (value must be a Hash).
	if _, err := BuildNestedQuery("scalar", ""); err == nil {
		t.Error("expected error for scalar at top level")
	} else if _, ok := err.(*ParameterTypeError); !ok {
		t.Errorf("expected *ParameterTypeError, got %T", err)
	}
}

func TestBuildNestedQueryNonStringScalar(t *testing.T) {
	p := NewParams()
	p.Set("n", 42)
	got, err := BuildNestedQuery(p, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "n=42" {
		t.Errorf("got %q", got)
	}
}
