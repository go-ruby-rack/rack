// Copyright (c) the go-ruby-rack/rack authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rack

import "testing"

func TestMediaTypeOf(t *testing.T) {
	cases := map[string]string{
		"text/plain;charset=utf-8": "text/plain",
		"TEXT/HTML":                "text/html",
		"text/plain , junk":        "text/plain",
		"application/json":         "application/json",
		"  text/xml  ; x=1":        "  text/xml",
		"":                         "",
	}
	for in, want := range cases {
		if got := MediaTypeOf(in); got != want {
			t.Errorf("MediaTypeOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMediaTypeParams(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"text/plain;charset=utf-8", `{"charset" => "utf-8"}`},
		{"text/plain;charset=", `{"charset" => ""}`},
		{"text/plain;charset", `{"charset" => ""}`},
		{`text/plain; charset="utf-8"`, `{"charset" => "utf-8"}`},
		{"multipart/form-data; boundary=AaB03x", `{"boundary" => "AaB03x"}`},
		{"text/plain", `{}`},
		{"", `{}`},
		{"text/plain; A=1; B=2", `{"a" => "1", "b" => "2"}`},
	}
	for _, c := range cases {
		if got := rubyInspect(MediaTypeParams(c.in)); got != c.want {
			t.Errorf("MediaTypeParams(%q) = %s, want %s", c.in, got, c.want)
		}
	}
}

func TestStripDoubleQuotes(t *testing.T) {
	if stripDoubleQuotes(`"x"`) != "x" {
		t.Error("strip quotes")
	}
	if stripDoubleQuotes("x") != "x" {
		t.Error("no quotes")
	}
	if stripDoubleQuotes("") != "" {
		t.Error("empty")
	}
	if stripDoubleQuotes(`"`) != `"` {
		t.Error("single quote char unchanged")
	}
}
