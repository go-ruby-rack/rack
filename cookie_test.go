// Copyright (c) the go-ruby-rack/rack authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rack

import (
	"reflect"
	"testing"
)

func TestMakeCookieHeader(t *testing.T) {
	got, err := MakeCookieHeader("foo", CookieValue{Value: "bar"})
	if err != nil || got != "foo=bar" {
		t.Errorf("simple = %q,%v", got, err)
	}

	got, err = MakeCookieHeader("foo", CookieValue{
		Value: "bar", Path: "/", MaxAge: "10", HTTPOnly: true, Secure: true, SameSite: "lax",
	})
	want := "foo=bar; path=/; max-age=10; secure; httponly; samesite=lax"
	if err != nil || got != want {
		t.Errorf("full = %q,%v want %q", got, err, want)
	}

	got, _ = MakeCookieHeader("foo", CookieValue{HasValues: true, Values: []string{"a", "b"}})
	if got != "foo=a&b" {
		t.Errorf("array = %q", got)
	}

	got, _ = MakeCookieHeader("foo", CookieValue{Value: "x y"})
	if got != "foo=x+y" {
		t.Errorf("escaped value = %q", got)
	}
}

func TestMakeCookieHeaderSameSiteVariants(t *testing.T) {
	for ss, want := range map[string]string{
		"none":   "foo=; samesite=none",
		"strict": "foo=; samesite=strict",
		"":       "foo=",
	} {
		got, err := MakeCookieHeader("foo", CookieValue{SameSite: ss})
		if err != nil || got != want {
			t.Errorf("samesite %q = %q,%v want %q", ss, got, err, want)
		}
	}
}

func TestMakeCookieHeaderDomainPartitioned(t *testing.T) {
	got, _ := MakeCookieHeader("foo", CookieValue{Value: "v", Domain: "example.com", Partitioned: true})
	if got != "foo=v; domain=example.com; partitioned" {
		t.Errorf("domain/partitioned = %q", got)
	}
	got, _ = MakeCookieHeader("foo", CookieValue{Value: "v", Expires: "Thu, 01 Jan 1970 00:00:00 GMT"})
	if got != "foo=v; expires=Thu, 01 Jan 1970 00:00:00 GMT" {
		t.Errorf("expires = %q", got)
	}
}

func TestMakeCookieHeaderInvalidKey(t *testing.T) {
	if _, err := MakeCookieHeader("bad key", CookieValue{Value: "v"}); err == nil {
		t.Error("expected invalid key error")
	} else if _, ok := err.(*ErrInvalidCookieKey); !ok {
		t.Errorf("wrong type %T", err)
	}
	if _, err := MakeCookieHeader("", CookieValue{}); err == nil {
		t.Error("empty key should be invalid")
	}
	_ = (&ErrInvalidCookieKey{Key: "k"}).Error()
}

func TestMakeCookieHeaderInvalidSameSite(t *testing.T) {
	if _, err := MakeCookieHeader("foo", CookieValue{SameSite: "weird"}); err == nil {
		t.Error("expected invalid same_site error")
	} else if _, ok := err.(*ErrInvalidSameSite); !ok {
		t.Errorf("wrong type %T", err)
	}
	_ = (&ErrInvalidSameSite{Value: "v"}).Error()
}

func TestValidCookieKey(t *testing.T) {
	for _, k := range []string{"foo", "a_b-c.d", "!#$%&'*+^`|~"} {
		if !validCookieKey(k) {
			t.Errorf("%q should be valid", k)
		}
	}
	for _, k := range []string{"", "a b", "a;b", "a=b", "a\tb"} {
		if validCookieKey(k) {
			t.Errorf("%q should be invalid", k)
		}
	}
}

func TestMakeDeleteCookieHeader(t *testing.T) {
	got, err := MakeDeleteCookieHeader("foo", CookieValue{})
	want := "foo=" + DeleteCookieHeaderValue
	if err != nil || got != want {
		t.Errorf("= %q,%v want %q", got, err, want)
	}
	// Invalid key surfaces the error.
	if _, err := MakeDeleteCookieHeader("bad key", CookieValue{}); err == nil {
		t.Error("expected error")
	}
}

func TestParseCookiesHeader(t *testing.T) {
	cases := map[string]string{
		"a=1; b=2":           `{"a" => "1", "b" => "2"}`,
		"x=%20space":         `{"x" => " space"}`,
		"dup=1; dup=2":       `{"dup" => "1"}`,
		"":                   `{}`,
		"novalue":            `{"novalue" => nil}`,
		"max-age=0; foo=bar": `{"max-age" => "0", "foo" => "bar"}`,
	}
	for in, want := range cases {
		if got := rubyInspect(ParseCookiesHeader(in)); got != want {
			t.Errorf("ParseCookiesHeader(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestParseCookiesHeaderBadEscapeFallback(t *testing.T) {
	// An undecodable value falls back to the raw bytes.
	if got := rubyInspect(ParseCookiesHeader("x=%ZZ")); got != `{"x" => "%ZZ"}` {
		t.Errorf("got %s", got)
	}
	// Empty segment between separators is skipped.
	if got := rubyInspect(ParseCookiesHeader("a=1; ; b=2")); got != `{"a" => "1", "b" => "2"}` {
		t.Errorf("got %s", got)
	}
}

func TestSetCookieHeaderInto(t *testing.T) {
	h := NewHeaders()
	if err := SetCookieHeaderInto(h, "a", CookieValue{Value: "1"}); err != nil {
		t.Fatal(err)
	}
	if h.Get(SetCookie) != "a=1" {
		t.Errorf("first = %v", h.Get(SetCookie))
	}
	if err := SetCookieHeaderInto(h, "b", CookieValue{Value: "2"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(h.Get(SetCookie), []any{"a=1", "b=2"}) {
		t.Errorf("second = %v", h.Get(SetCookie))
	}
	if err := SetCookieHeaderInto(h, "c", CookieValue{Value: "3"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(h.Get(SetCookie), []any{"a=1", "b=2", "c=3"}) {
		t.Errorf("third = %v", h.Get(SetCookie))
	}
	// Error path.
	if err := SetCookieHeaderInto(h, "bad key", CookieValue{}); err == nil {
		t.Error("expected error")
	}
}

func TestDeleteCookieHeaderInto(t *testing.T) {
	h := NewHeaders()
	if err := DeleteCookieHeaderInto(h, "a", CookieValue{}); err != nil {
		t.Fatal(err)
	}
	if h.Get(SetCookie) != "a="+DeleteCookieHeaderValue {
		t.Errorf("first = %v", h.Get(SetCookie))
	}
	if err := DeleteCookieHeaderInto(h, "b", CookieValue{}); err != nil {
		t.Fatal(err)
	}
	arr, ok := h.Get(SetCookie).([]any)
	if !ok || len(arr) != 2 {
		t.Errorf("second = %v", h.Get(SetCookie))
	}
	if err := DeleteCookieHeaderInto(h, "bad key", CookieValue{}); err == nil {
		t.Error("expected error")
	}
}

func TestToAnyList(t *testing.T) {
	if toAnyList(nil) != nil {
		t.Error("nil")
	}
	if !reflect.DeepEqual(toAnyList("x"), []any{"x"}) {
		t.Error("string")
	}
	if !reflect.DeepEqual(toAnyList([]any{"x"}), []any{"x"}) {
		t.Error("list")
	}
}
