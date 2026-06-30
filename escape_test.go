// Copyright (c) the go-ruby-rack/rack authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rack

import "testing"

func TestEscape(t *testing.T) {
	cases := map[string]string{
		"a b":     "a+b",
		"a+b":     "a%2Bb",
		"100%":    "100%25",
		"foo&bar": "foo%26bar",
		"café":    "caf%C3%A9",
		"a/b?c=d": "a%2Fb%3Fc%3Dd",
		"~-_.":    "%7E-_.",
		"*!()":    "*%21%28%29",
		"[]{}":    "%5B%5D%7B%7D",
		"a=b":     "a%3Db",
		"\n\t":    "%0A%09",
		"":        "",
	}
	for in, want := range cases {
		if got := Escape(in); got != want {
			t.Errorf("Escape(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEscapePath(t *testing.T) {
	cases := map[string]string{
		"a b":         "a%20b",
		"a/b/c":       "a/b/c",
		"foo bar/baz": "foo%20bar/baz",
		"café":        "caf%C3%A9",
		"a%b":         "a%25b",
		"[]":          "[]",
		"":            "",
	}
	for in, want := range cases {
		if got := EscapePath(in); got != want {
			t.Errorf("EscapePath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestUnescape(t *testing.T) {
	cases := map[string]string{
		"a+b":    "a b",
		"a%20b":  "a b",
		"%C3%A9": "é",
		"%2F":    "/",
		"plain":  "plain",
		"a%2Bb":  "a+b",
		"":       "",
		"%41%42": "AB",
	}
	for in, want := range cases {
		got, err := Unescape(in)
		if err != nil {
			t.Errorf("Unescape(%q) unexpected error %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("Unescape(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestUnescapeInvalid(t *testing.T) {
	for _, in := range []string{"bad%ZZ", "trunc%2", "end%", "%G0", "%0G"} {
		if _, err := Unescape(in); err == nil {
			t.Errorf("Unescape(%q) expected error, got nil", in)
		}
	}
}

func TestUnescapePath(t *testing.T) {
	cases := map[string]string{
		"a%20b":  "a b",
		"a+b":    "a+b", // '+' not turned into space
		"%2F":    "/",
		"bad%ZZ": "bad%ZZ", // lenient: invalid escape left verbatim
		"trail%": "trail%",
		"plain":  "plain",
	}
	for in, want := range cases {
		if got := UnescapePath(in); got != want {
			t.Errorf("UnescapePath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEscapeHTML(t *testing.T) {
	in := `<a href="x">& '`
	want := "&lt;a href=&quot;x&quot;&gt;&amp; &#39;"
	if got := EscapeHTML(in); got != want {
		t.Errorf("EscapeHTML(%q) = %q, want %q", in, got, want)
	}
}

func TestUnescapeHTML(t *testing.T) {
	cases := map[string]string{
		"&lt;a&gt;&amp;&quot;&#39;": `<a>&"'`,
		"&apos;":                    "'",
		"&#65;":                     "A",
		"&#x41;":                    "A",
		"plain &amp; text":          "plain & text",
		"bare & amp":                "bare & amp",
		"&unknown;":                 "&unknown;",
		"&toolongentityname;":       "&toolongentityname;",
		"&#xZZ;":                    "&#xZZ;",
		"&#x;":                      "&#x;",
		"&#9A;":                     "&#9A;",
		"no entities":               "no entities",
		"trailing&":                 "trailing&",
	}
	for in, want := range cases {
		if got := UnescapeHTML(in); got != want {
			t.Errorf("UnescapeHTML(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEscapeHTMLRoundTrip(t *testing.T) {
	for _, s := range []string{`<a href="x">& '`, "plain", "&&<<>>"} {
		if got := UnescapeHTML(EscapeHTML(s)); got != s {
			t.Errorf("round-trip(%q) = %q", s, got)
		}
	}
}
