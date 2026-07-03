// Copyright (c) the go-ruby-rack/rack authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rack

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestCleanPathInfo(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/foo/../bar", "/bar"},
		{"foo/./bar/", "foo/bar"},
		{"/../../etc/passwd", "/etc/passwd"},
		{"", "/"},
		{"a//b", "a/b"},
		{"/a/b/..", "/a"},
		{"..", ""},           // sole ".." on relative path pops nothing, no leading slash
		{"/", "/"},           // bare separator
		{"foo", "foo"},       // no separators, relative
		{"/foo", "/foo"},     // absolute, single segment
		{"a/../..", ""},      // pops past root on a relative path
		{"/a/../../b", "/b"}, // pop-past-root then descend
	}
	for _, c := range cases {
		if got := CleanPathInfo(c.in); got != c.want {
			t.Errorf("CleanPathInfo(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestValidPath(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"/normal/path", true},
		{"café", true},
		{"", true},
		{"/with\x00null", false},
		{"\xff\xfe", false}, // invalid UTF-8
	}
	for _, c := range cases {
		if got := ValidPath(c.in); got != c.want {
			t.Errorf("ValidPath(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestSecureCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"abc", "abc", true},
		{"abc", "abd", false},
		{"abc", "ab", false},
		{"", "", true},
		{"secret-token", "secret-token", true},
	}
	for _, c := range cases {
		if got := SecureCompare(c.a, c.b); got != c.want {
			t.Errorf("SecureCompare(%q,%q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestSelectBestEncoding(t *testing.T) {
	q := func(pairs ...any) []QValue {
		var out []QValue
		for i := 0; i < len(pairs); i += 2 {
			out = append(out, QValue{pairs[i].(string), pairs[i+1].(float64)})
		}
		return out
	}
	cases := []struct {
		name      string
		available []string
		accept    []QValue
		want      string
		ok        bool
	}{
		{"quality-order", []string{"gzip", "deflate", "identity"}, q("gzip", 1.0, "deflate", 0.5), "gzip", true},
		{"wildcard", []string{"gzip", "identity"}, q("*", 1.0), "gzip", true},
		{"wildcard-twice", []string{"gzip", "identity"}, q("*", 1.0, "*", 0.5), "gzip", true},
		{"zero-quality", []string{"gzip", "identity"}, q("gzip", 0.0), "identity", true},
		{"none", []string{"gzip"}, q("deflate", 1.0), "", false},
		{"pref-tie", []string{"b", "a"}, q("a", 1.0, "b", 1.0), "b", true},
		{"implicit-identity", []string{"identity", "gzip"}, q("br", 0.9), "identity", true},
		{"identity-explicit", []string{"gzip", "identity"}, q("identity", 1.0, "gzip", 0.5), "identity", true},
	}
	for _, c := range cases {
		got, ok := SelectBestEncoding(c.available, c.accept)
		if got != c.want || ok != c.ok {
			t.Errorf("%s: SelectBestEncoding(%v,%v) = (%q,%v), want (%q,%v)",
				c.name, c.available, c.accept, got, ok, c.want, c.ok)
		}
	}
	// More than 16 accept entries: only the first 16 are considered, so a 17th
	// naming a better coding is ignored.
	long := make([]QValue, 0, 20)
	for i := 0; i < 16; i++ {
		long = append(long, QValue{"identity", 1.0})
	}
	long = append(long, QValue{"gzip", 1.0}) // 17th, dropped
	if got, ok := SelectBestEncoding([]string{"gzip", "identity"}, long); got != "identity" || !ok {
		t.Errorf("truncation: got (%q,%v), want (identity,true)", got, ok)
	}
}

func TestForwardedValues(t *testing.T) {
	cases := []struct {
		name   string
		header string
		has    bool
		want   map[string][]string
		ok     bool
	}{
		{"simple", "for=1.2.3.4;proto=https", true, map[string][]string{"for": {"1.2.3.4"}, "proto": {"https"}}, true},
		{"quoted", `for="[2001:db8::1]:8080";host=example.com`, true, map[string][]string{"for": {"[2001:db8::1]:8080"}, "host": {"example.com"}}, true},
		{"repeat", "for=a, for=b", true, map[string][]string{"for": {"a", "b"}}, true},
		{"absent", "", false, nil, false},
		{"disallowed", "bogus=x", true, nil, false},
		{"quoted-escape", `for="a\"b"`, true, map[string][]string{"for": {`a"b`}}, true},
		{"unterminated-quote", `for="abc`, true, map[string][]string{"for": {"abc"}}, true},
		{"rest-of-line", "for=abc", true, map[string][]string{"for": {"abc"}}, true},
		{"newline", "for=a\nproto=b", true, map[string][]string{"for": {"a"}, "proto": {"b"}}, true},
		{"case-insensitive", "For=UP;PROTO=HTTPS", true, map[string][]string{"for": {"UP"}, "proto": {"HTTPS"}}, true},
		{"by-param", "by=203.0.113.43", true, map[string][]string{"by": {"203.0.113.43"}}, true},
		{"trailing-space", "for=a ;proto=b", true, map[string][]string{"for": {"a"}, "proto": {"b"}}, true},
	}
	for _, c := range cases {
		got, ok := ForwardedValues(c.header, c.has)
		if ok != c.ok {
			t.Errorf("%s: ok = %v, want %v", c.name, ok, c.ok)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: ForwardedValues(%q) = %v, want %v", c.name, c.header, got, c.want)
		}
	}
	// DoS guards: exceeding the parameter or escape limits aborts to nil.
	if _, ok := ForwardedValues(strings.Repeat("for=a;", forwardedMaxParams+1), true); ok {
		t.Errorf("param-limit: expected abort")
	}
	if _, ok := ForwardedValues(`for="`+strings.Repeat(`\a`, forwardedMaxEscapes+1)+`"`, true); ok {
		t.Errorf("escape-limit: expected abort")
	}
}

func TestParseCookies(t *testing.T) {
	env := Env{HTTPCookie: "a=1; b=2; a=3"}
	got := ParseCookies(env)
	if v, _ := got.Get("a"); v != "1" {
		t.Errorf("a = %v, want 1 (first wins)", v)
	}
	if v, _ := got.Get("b"); v != "2" {
		t.Errorf("b = %v, want 2", v)
	}
	// Missing / non-string HTTP_COOKIE yields an empty map.
	if ParseCookies(Env{}).Len() != 0 {
		t.Errorf("empty env should parse to empty cookies")
	}
	if ParseCookies(Env{HTTPCookie: 42}).Len() != 0 {
		t.Errorf("non-string HTTP_COOKIE should parse to empty cookies")
	}
}

// --- MRI oracle tests for the new utilities -----------------------------------

func TestOracleCleanPathInfo(t *testing.T) {
	bin := rubyBin(t)
	inputs := []string{"/foo/../bar", "foo/./bar/", "/../../etc/passwd", "", "a//b", "/a/b/..", "/", "..", "a/../.."}
	// Wrap in sentinels so an empty result survives the trailing-newline trim.
	want := rubyLines(t, bin, rubyArrayPreamble(inputs)+`INPUTS.each { |s| puts "<#{Rack::Utils.clean_path_info(s)}>" }`)
	for i, in := range inputs {
		if got := "<" + CleanPathInfo(in) + ">"; got != want[i] {
			t.Errorf("CleanPathInfo(%q) = %q, MRI = %q", in, got, want[i])
		}
	}
}

func TestOracleValidPath(t *testing.T) {
	bin := rubyBin(t)
	// The NUL byte cannot travel through exec argv, so the inputs are built as
	// Ruby literals (with \x00 escaped) inside the script itself.
	inputs := []string{"/normal", "café", "a\x00b"}
	want := rubyLines(t, bin, `[ "/normal", "café", "a\x00b" ].each { |s| puts Rack::Utils.valid_path?(s) }`)
	for i, in := range inputs {
		got := "false"
		if ValidPath(in) {
			got = "true"
		}
		if got != want[i] {
			t.Errorf("ValidPath(%q) = %s, MRI = %s", in, got, want[i])
		}
	}
}

func TestOracleSecureCompare(t *testing.T) {
	bin := rubyBin(t)
	type pair struct{ a, b string }
	pairs := []pair{{"abc", "abc"}, {"abc", "abd"}, {"abc", "ab"}, {"", ""}}
	var rb []string
	for _, p := range pairs {
		rb = append(rb, "Rack::Utils.secure_compare("+rubyStr(p.a)+", "+rubyStr(p.b)+")")
	}
	want := rubyLines(t, bin, "["+strings.Join(rb, ", ")+"].each { |v| puts v }")
	for i, p := range pairs {
		got := "false"
		if SecureCompare(p.a, p.b) {
			got = "true"
		}
		if got != want[i] {
			t.Errorf("SecureCompare(%q,%q) = %s, MRI = %s", p.a, p.b, got, want[i])
		}
	}
}

func TestOracleSelectBestEncoding(t *testing.T) {
	bin := rubyBin(t)
	// Each case: available list + Ruby accept literal (array of [enc, q]).
	type c struct {
		avail  []string
		accept string
		qv     []QValue
	}
	cases := []c{
		{[]string{"gzip", "deflate", "identity"}, `[["gzip",1.0],["deflate",0.5]]`, []QValue{{"gzip", 1.0}, {"deflate", 0.5}}},
		{[]string{"gzip", "identity"}, `[["*",1.0]]`, []QValue{{"*", 1.0}}},
		{[]string{"gzip", "identity"}, `[["gzip",0.0]]`, []QValue{{"gzip", 0.0}}},
		{[]string{"gzip"}, `[["deflate",1.0]]`, []QValue{{"deflate", 1.0}}},
		{[]string{"b", "a"}, `[["a",1.0],["b",1.0]]`, []QValue{{"a", 1.0}, {"b", 1.0}}},
	}
	var scripts []string
	for _, cc := range cases {
		av := "%w[" + strings.Join(cc.avail, " ") + "]"
		scripts = append(scripts, "Rack::Utils.select_best_encoding("+av+", "+cc.accept+").inspect")
	}
	want := rubyLines(t, bin, "["+strings.Join(scripts, ", ")+"].each { |v| puts v }")
	for i, cc := range cases {
		got, ok := SelectBestEncoding(cc.avail, cc.qv)
		g := "nil"
		if ok {
			g = rubyStr(got)
		}
		if g != want[i] {
			t.Errorf("SelectBestEncoding(%v) = %s, MRI = %s", cc.avail, g, want[i])
		}
	}
}

func TestOracleForwardedValues(t *testing.T) {
	bin := rubyBin(t)
	inputs := []string{
		"for=1.2.3.4;proto=https",
		`for="[2001:db8::1]:8080";host=example.com`,
		"for=a, for=b",
		"bogus=x",
		"For=UP;PROTO=HTTPS",
		"by=203.0.113.43",
	}
	// Canonicalise the Ruby hash to "param=v1|v2;param2=..." with params sorted.
	script := rubyArrayPreamble(inputs) + `INPUTS.each do |s|
  h = Rack::Utils.forwarded_values(s)
  if h.nil?
    puts "nil"
  else
    puts h.sort_by { |k, _| k.to_s }.map { |k, v| "#{k}=#{v.join('|')}" }.join(";")
  end
end`
	want := rubyLines(t, bin, script)
	for i, in := range inputs {
		got, ok := ForwardedValues(in, true)
		var line string
		if !ok {
			line = "nil"
		} else {
			keys := make([]string, 0, len(got))
			for k := range got {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			var segs []string
			for _, k := range keys {
				segs = append(segs, k+"="+strings.Join(got[k], "|"))
			}
			line = strings.Join(segs, ";")
		}
		if line != want[i] {
			t.Errorf("ForwardedValues(%q) = %q, MRI = %q", in, line, want[i])
		}
	}
}

func TestOracleParseCookies(t *testing.T) {
	bin := rubyBin(t)
	out := rubyEval(t, bin, `p Rack::Utils.parse_cookies({"HTTP_COOKIE" => "a=1; b=2; a=3"})`)
	want := strings.TrimRight(out, "\n")
	got := rubyInspect(ParseCookies(Env{HTTPCookie: "a=1; b=2; a=3"}))
	if got != want {
		t.Errorf("ParseCookies = %s, MRI = %s", got, want)
	}
}
