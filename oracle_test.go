// Copyright (c) the go-ruby-rack/rack authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rack

import (
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// rubyBin locates a usable `ruby` with the rack gem once. The oracle tests skip
// themselves when ruby (or the rack gem) is absent — the Windows lane, the qemu
// cross-arch lanes, and any host without the gem — so the deterministic suite
// alone drives the 100% gate there.
func rubyBin(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("ruby")
	if err != nil {
		t.Skip("ruby not on PATH; skipping MRI oracle")
	}
	// Confirm the rack gem is loadable; skip if not installed.
	if err := exec.Command(path, "-rrack", "-e", "1").Run(); err != nil {
		t.Skip("rack gem not installed; skipping MRI oracle")
	}
	return path
}

// rubyEval runs a Ruby script with rack required and returns its stdout. The
// script binmodes $stdout so Windows text-mode never pollutes the bytes (the
// go-ruby-erb lesson), though the oracle skips on Windows anyway.
func rubyEval(t *testing.T, bin, script string) string {
	t.Helper()
	cmd := exec.Command(bin, "-rrack", "-e", "$stdout.binmode\n"+script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ruby error: %v\nscript:\n%s\noutput:\n%s", err, script, out)
	}
	return string(out)
}

// rubyLines runs a script that prints one result per input and splits the
// output into trimmed lines.
func rubyLines(t *testing.T, bin, script string) []string {
	out := rubyEval(t, bin, script)
	return strings.Split(strings.TrimRight(out, "\n"), "\n")
}

func TestOracleEscape(t *testing.T) {
	bin := rubyBin(t)
	inputs := []string{"a b", "a+b", "100%", "foo&bar", "café", "a/b?c=d", "~-_.", "*!()", "[]{}", "a=b", "\n\t"}
	script := `INPUTS.each { |s| puts Rack::Utils.escape(s) }`
	want := rubyLines(t, bin, rubyArrayPreamble(inputs)+script)
	for i, in := range inputs {
		if got := Escape(in); got != want[i] {
			t.Errorf("Escape(%q) = %q, MRI = %q", in, got, want[i])
		}
	}
}

func TestOracleEscapePath(t *testing.T) {
	bin := rubyBin(t)
	inputs := []string{"a b", "a/b/c", "foo bar/baz", "café", "a%b", "[]"}
	want := rubyLines(t, bin, rubyArrayPreamble(inputs)+`INPUTS.each { |s| puts Rack::Utils.escape_path(s) }`)
	for i, in := range inputs {
		if got := EscapePath(in); got != want[i] {
			t.Errorf("EscapePath(%q) = %q, MRI = %q", in, got, want[i])
		}
	}
}

func TestOracleUnescape(t *testing.T) {
	bin := rubyBin(t)
	inputs := []string{"a+b", "a%20b", "%C3%A9", "%2F", "plain", "a%2Bb"}
	want := rubyLines(t, bin, rubyArrayPreamble(inputs)+`INPUTS.each { |s| puts Rack::Utils.unescape(s) }`)
	for i, in := range inputs {
		got, err := Unescape(in)
		if err != nil {
			t.Errorf("Unescape(%q) error %v", in, err)
			continue
		}
		if got != want[i] {
			t.Errorf("Unescape(%q) = %q, MRI = %q", in, got, want[i])
		}
	}
}

func TestOracleEscapeHTML(t *testing.T) {
	bin := rubyBin(t)
	inputs := []string{
		`<a href="x">& '`, "plain", "&&<<>>",
		"&leading", "trailing&", // escape at the two boundaries
		`<p class="lead">Tom &amp; Jerry's "quote" &lt;3</p>`, // realistic mixed markup
		"no entities here at all",                             // fast-path (returns input unchanged)
	}
	want := rubyLines(t, bin, rubyArrayPreamble(inputs)+`INPUTS.each { |s| puts Rack::Utils.escape_html(s) }`)
	for i, in := range inputs {
		if got := EscapeHTML(in); got != want[i] {
			t.Errorf("EscapeHTML(%q) = %q, MRI = %q", in, got, want[i])
		}
	}
}

func TestOracleParseNestedQuery(t *testing.T) {
	bin := rubyBin(t)
	inputs := []string{
		"foo=bar", "foo[]=1&foo[]=2", "foo[bar]=baz", "a[b][c]=d",
		"x[][a]=1&x[][b]=2", "x[][a]=1&x[][a]=2", "a=1&a=2", "key", "foo=",
		"a[b]=c&a[d]=e", "a[][]=1", "a[]b=1", "a[][c][]=1", "",
	}
	want := rubyLines(t, bin, rubyArrayPreamble(inputs)+`INPUTS.each { |s| p Rack::Utils.parse_nested_query(s) }`)
	for i, in := range inputs {
		p, err := ParseNestedQuery(in, "&", DefaultParamDepthLimit)
		if err != nil {
			t.Errorf("ParseNestedQuery(%q) error %v", in, err)
			continue
		}
		if got := rubyInspect(p); got != want[i] {
			t.Errorf("ParseNestedQuery(%q) = %s, MRI = %s", in, got, want[i])
		}
	}
}

func TestOracleParseQuery(t *testing.T) {
	bin := rubyBin(t)
	inputs := []string{"a=1&a=2", "foo=bar&baz=qux", "a", "a=", "=b", "a%20b=c%20d"}
	want := rubyLines(t, bin, rubyArrayPreamble(inputs)+`INPUTS.each { |s| p Rack::Utils.parse_query(s) }`)
	for i, in := range inputs {
		p, err := ParseQuery(in, "&")
		if err != nil {
			t.Errorf("ParseQuery(%q) error %v", in, err)
			continue
		}
		if got := rubyInspect(p); got != want[i] {
			t.Errorf("ParseQuery(%q) = %s, MRI = %s", in, got, want[i])
		}
	}
}

func TestOracleBuildNestedQuery(t *testing.T) {
	bin := rubyBin(t)
	// Each case is a Ruby literal hash and its Go equivalent built inline.
	type pair struct {
		ruby string
		go_  *Params
	}
	mk := func(kv ...any) *Params {
		p := NewParams()
		for i := 0; i < len(kv); i += 2 {
			p.Set(kv[i].(string), kv[i+1])
		}
		return p
	}
	cases := []pair{
		{`{"a"=>"1"}`, mk("a", "1")},
		{`{"a"=>["1","2"]}`, mk("a", []any{"1", "2"})},
		{`{"a"=>{"b"=>"c"}}`, mk("a", mk("b", "c"))},
		{`{"a"=>{"b"=>["1","2"]}}`, mk("a", mk("b", []any{"1", "2"}))},
	}
	var rubies []string
	for _, c := range cases {
		rubies = append(rubies, c.ruby)
	}
	script := "[" + strings.Join(rubies, ", ") + "].each { |h| puts Rack::Utils.build_nested_query(h) }"
	want := rubyLines(t, bin, script)
	for i, c := range cases {
		got, err := BuildNestedQuery(c.go_, "")
		if err != nil {
			t.Errorf("BuildNestedQuery error %v", err)
			continue
		}
		if got != want[i] {
			t.Errorf("BuildNestedQuery(%s) = %q, MRI = %q", c.ruby, got, want[i])
		}
	}
}

func TestOracleStatusCodes(t *testing.T) {
	bin := rubyBin(t)
	out := rubyEval(t, bin, `Rack::Utils::HTTP_STATUS_CODES.each { |k, v| puts "#{k}\t#{v}" }`)
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		parts := strings.SplitN(line, "\t", 2)
		code, _ := strconv.Atoi(parts[0])
		if HTTPStatusCodes[code] != parts[1] {
			t.Errorf("status %d = %q, MRI = %q", code, HTTPStatusCodes[code], parts[1])
		}
	}
	// And the reverse direction (symbols → codes).
	if got, ok := SymbolToStatusCode("not_found"); !ok || got != 404 {
		t.Errorf("not_found = %d", got)
	}
}

func TestOracleResponseFinish(t *testing.T) {
	bin := rubyBin(t)
	out := rubyEval(t, bin, `
r = Rack::Response.new("Hello", 201, {"content-type" => "text/plain"})
status, headers, body = r.finish
puts status
puts headers["content-length"]
puts headers["content-type"]
puts body.join
`)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	r := NewResponseString("Hello", 201, HeadersOf(map[string]any{"content-type": "text/plain"}))
	status, headers, body := r.Finish()
	if strconv.Itoa(status) != lines[0] {
		t.Errorf("status %d vs %s", status, lines[0])
	}
	if headers.Get(ContentLengthKey) != lines[1] {
		t.Errorf("content-length %v vs %s", headers.Get(ContentLengthKey), lines[1])
	}
	if headers.Get(ContentTypeKey) != lines[2] {
		t.Errorf("content-type %v vs %s", headers.Get(ContentTypeKey), lines[2])
	}
	if strings.Join(body, "") != lines[3] {
		t.Errorf("body %q vs %s", body, lines[3])
	}
}

func TestOracleCookies(t *testing.T) {
	bin := rubyBin(t)
	out := rubyEval(t, bin, `
puts Rack::Utils.set_cookie_header("foo", {value: "bar", path: "/", max_age: 10, http_only: true, secure: true, same_site: :lax})
puts Rack::Utils.delete_set_cookie_header("foo")
p Rack::Utils.parse_cookies_header("a=1; b=2; a=3")
`)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")

	got, _ := MakeCookieHeader("foo", CookieValue{Value: "bar", Path: "/", MaxAge: "10", HTTPOnly: true, Secure: true, SameSite: "lax"})
	if got != lines[0] {
		t.Errorf("set_cookie = %q, MRI = %q", got, lines[0])
	}
	del, _ := MakeDeleteCookieHeader("foo", CookieValue{})
	if del != lines[1] {
		t.Errorf("delete_cookie = %q, MRI = %q", del, lines[1])
	}
	if c := rubyInspect(ParseCookiesHeader("a=1; b=2; a=3")); c != lines[2] {
		t.Errorf("parse_cookies = %s, MRI = %s", c, lines[2])
	}
}

func TestOracleMediaType(t *testing.T) {
	bin := rubyBin(t)
	inputs := []string{"text/plain;charset=utf-8", "TEXT/HTML", "multipart/form-data; boundary=AaB03x", "text/plain"}
	want := rubyLines(t, bin, rubyArrayPreamble(inputs)+`INPUTS.each { |s| puts Rack::MediaType.type(s).to_s; p Rack::MediaType.params(s) }`)
	for i, in := range inputs {
		if got := MediaTypeOf(in); got != want[i*2] {
			t.Errorf("MediaType.type(%q) = %q, MRI = %q", in, got, want[i*2])
		}
		if got := rubyInspect(MediaTypeParams(in)); got != want[i*2+1] {
			t.Errorf("MediaType.params(%q) = %s, MRI = %s", in, got, want[i*2+1])
		}
	}
}

func TestOracleByteRanges(t *testing.T) {
	bin := rubyBin(t)
	out := rubyEval(t, bin, `
[["bytes=0-499",1000],["bytes=500-",1000],["bytes=-200",1000],["bytes=0-0,-1",1000]].each do |r, s|
  puts Rack::Utils.get_byte_ranges(r, s).map { |x| "#{x.begin}-#{x.end}" }.join(",")
end
`)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	specs := []struct {
		spec string
		size int
	}{
		{"bytes=0-499", 1000}, {"bytes=500-", 1000}, {"bytes=-200", 1000}, {"bytes=0-0,-1", 1000},
	}
	for i, s := range specs {
		ranges, ok := GetByteRanges(s.spec, s.size, 100)
		if !ok {
			t.Errorf("GetByteRanges(%q) not ok", s.spec)
			continue
		}
		var parts []string
		for _, r := range ranges {
			parts = append(parts, strconv.Itoa(r.Start)+"-"+strconv.Itoa(r.End))
		}
		if got := strings.Join(parts, ","); got != lines[i] {
			t.Errorf("GetByteRanges(%q) = %q, MRI = %q", s.spec, got, lines[i])
		}
	}
}

func TestOracleQValues(t *testing.T) {
	bin := rubyBin(t)
	inputs := []string{"text/html;q=0.8, application/json", "*/*", "text/*;q=0.5,text/html"}
	out := rubyEval(t, bin, rubyArrayPreamble(inputs)+`INPUTS.each { |h| puts Rack::Utils.q_values(h).map { |v, q| "#{v}=#{q}" }.join(",") }`)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	for i, in := range inputs {
		var parts []string
		for _, qv := range QValues(in) {
			parts = append(parts, qv.Value+"="+rubyFloat(qv.Quality))
		}
		if got := strings.Join(parts, ","); got != lines[i] {
			t.Errorf("QValues(%q) = %q, MRI = %q", in, got, lines[i])
		}
	}
}

// rubyFloat renders a float the way Ruby's Float#to_s does for the simple
// quality values used here — always with at least one fractional digit (1.0,
// 0.8) so the oracle string comparison lines up with MRI.
func rubyFloat(f float64) string {
	s := strconv.FormatFloat(f, 'g', -1, 64)
	if !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	return s
}

// rubyArrayPreamble emits an INPUTS array literal of the given strings so the
// oracle scripts can iterate them. Each string is rendered as a Ruby literal.
func rubyArrayPreamble(inputs []string) string {
	parts := make([]string, len(inputs))
	for i, s := range inputs {
		parts[i] = rubyStr(s)
	}
	return "INPUTS = [" + strings.Join(parts, ", ") + "]\n"
}
