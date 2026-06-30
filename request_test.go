// Copyright (c) the go-ruby-rack/rack authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rack

import (
	"errors"
	"testing"
)

// stringInput is a test [Input] backed by a byte slice.
type stringInput struct {
	data []byte
	pos  int
	err  error
}

func (s *stringInput) Read(n int) ([]byte, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.pos >= len(s.data) {
		return nil, nil
	}
	if n < 0 || s.pos+n > len(s.data) {
		out := s.data[s.pos:]
		s.pos = len(s.data)
		return out, nil
	}
	out := s.data[s.pos : s.pos+n]
	s.pos += n
	return out, nil
}

func TestRequestBasics(t *testing.T) {
	env := Env{
		RequestMethod: "GET",
		PathInfo:      "/hello",
		ScriptName:    "/app",
		QueryString:   "a=1&b=2",
		ServerName:    "example.com",
		ServerPort:    "8080",
		RackURLScheme: "http",
	}
	r := NewRequest(env)
	if r.RequestMethod() != "GET" || !r.IsGet() {
		t.Error("method")
	}
	if r.PathInfo() != "/hello" || r.ScriptName() != "/app" {
		t.Error("paths")
	}
	if r.Path() != "/app/hello" {
		t.Errorf("path = %q", r.Path())
	}
	if r.QueryString() != "a=1&b=2" {
		t.Error("query string")
	}
	if r.Fullpath() != "/app/hello?a=1&b=2" {
		t.Errorf("fullpath = %q", r.Fullpath())
	}
	if r.URL() != "http://example.com:8080/app/hello?a=1&b=2" {
		t.Errorf("url = %q", r.URL())
	}
	if r.Env()[RequestMethod] != "GET" {
		t.Error("Env accessor")
	}
}

func TestRequestMethodPredicates(t *testing.T) {
	methods := map[string]func(*Request) bool{
		"GET": (*Request).IsGet, "POST": (*Request).IsPost, "PUT": (*Request).IsPut,
		"PATCH": (*Request).IsPatch, "DELETE": (*Request).IsDelete, "HEAD": (*Request).IsHead,
		"OPTIONS": (*Request).IsOptions, "TRACE": (*Request).IsTrace, "LINK": (*Request).IsLink,
		"UNLINK": (*Request).IsUnlink,
	}
	for m, pred := range methods {
		r := NewRequest(Env{RequestMethod: m})
		if !pred(r) {
			t.Errorf("predicate for %s false", m)
		}
	}
}

func TestRequestHeaderAccessors(t *testing.T) {
	r := NewRequest(Env{"FOO": "bar"})
	if r.GetHeader("FOO") != "bar" || r.GetHeader("MISSING") != "" {
		t.Error("GetHeader")
	}
	if v, ok := r.GetHeaderRaw("FOO"); !ok || v != "bar" {
		t.Error("GetHeaderRaw")
	}
	if !r.HasHeader("FOO") || r.HasHeader("X") {
		t.Error("HasHeader")
	}
	r.SetHeader("BAZ", "qux")
	if r.GetHeader("BAZ") != "qux" {
		t.Error("SetHeader")
	}
	if r.DeleteHeader("BAZ") != "qux" || r.HasHeader("BAZ") {
		t.Error("DeleteHeader")
	}
	// Non-string header value returns "".
	r.SetHeader("NUM", 5)
	if r.GetHeader("NUM") != "" {
		t.Error("non-string GetHeader should be empty")
	}
}

func TestRequestScheme(t *testing.T) {
	if NewRequest(Env{HTTPS: "on"}).Scheme() != "https" {
		t.Error("HTTPS on")
	}
	if NewRequest(Env{"HTTP_X_FORWARDED_SSL": "on"}).Scheme() != "https" {
		t.Error("forwarded ssl")
	}
	if NewRequest(Env{"HTTP_X_FORWARDED_PROTO": "https"}).Scheme() != "https" {
		t.Error("forwarded proto")
	}
	if NewRequest(Env{"HTTP_X_FORWARDED_SCHEME": "https"}).Scheme() != "https" {
		t.Error("forwarded scheme")
	}
	if NewRequest(Env{RackURLScheme: "http"}).Scheme() != "http" {
		t.Error("url scheme")
	}
	// Forwarded proto with a non-allowed value is ignored.
	if NewRequest(Env{"HTTP_X_FORWARDED_PROTO": "gopher", RackURLScheme: "http"}).Scheme() != "http" {
		t.Error("disallowed proto should fall through")
	}
}

func TestRequestSSL(t *testing.T) {
	if !NewRequest(Env{HTTPS: "on"}).SSL() {
		t.Error("https should be ssl")
	}
	if !NewRequest(Env{RackURLScheme: "wss"}).SSL() {
		t.Error("wss should be ssl")
	}
	if NewRequest(Env{RackURLScheme: "http"}).SSL() {
		t.Error("http not ssl")
	}
}

func TestRequestHostPort(t *testing.T) {
	r := NewRequest(Env{HTTPHost: "example.com:3000", RackURLScheme: "http"})
	if r.Host() != "example.com" || r.Port() != 3000 {
		t.Errorf("host=%q port=%d", r.Host(), r.Port())
	}
	if r.HostWithPort() != "example.com:3000" {
		t.Errorf("hostwithport = %q", r.HostWithPort())
	}
	// Default port omitted from host_with_port.
	r2 := NewRequest(Env{HTTPHost: "example.com:80", RackURLScheme: "http"})
	if r2.HostWithPort() != "example.com" {
		t.Errorf("default port should be omitted, got %q", r2.HostWithPort())
	}
	if r2.Port() != 80 {
		t.Errorf("port = %d", r2.Port())
	}
	// No HTTP_HOST falls back to SERVER_NAME/PORT.
	r3 := NewRequest(Env{ServerName: "srv", ServerPort: "9090", RackURLScheme: "http"})
	if r3.Host() != "srv" || r3.Port() != 9090 {
		t.Errorf("server fallback host=%q port=%d", r3.Host(), r3.Port())
	}
	if r3.Authority() != "srv:9090" {
		t.Errorf("authority = %q", r3.Authority())
	}
}

func TestRequestPortDefaults(t *testing.T) {
	// No authority, no port info: default for scheme.
	r := NewRequest(Env{RackURLScheme: "https", ServerName: "h"})
	if r.Port() != 443 {
		t.Errorf("https default port = %d", r.Port())
	}
	// Unknown scheme falls back to SERVER_PORT.
	r2 := NewRequest(Env{RackURLScheme: "gopher", ServerName: "h", ServerPort: "70"})
	if r2.Port() != 70 {
		t.Errorf("server port fallback = %d", r2.Port())
	}
}

func TestRequestHostIPv6(t *testing.T) {
	r := NewRequest(Env{HTTPHost: "[::1]:8080", RackURLScheme: "http"})
	if r.Host() != "[::1]" {
		t.Errorf("host = %q", r.Host())
	}
	if r.Hostname() != "::1" {
		t.Errorf("hostname = %q", r.Hostname())
	}
	if r.Port() != 8080 {
		t.Errorf("port = %d", r.Port())
	}
}

func TestRequestServerAuthorityEmpty(t *testing.T) {
	r := NewRequest(Env{RackURLScheme: "http"})
	if r.Authority() != "" || r.Host() != "" {
		t.Errorf("empty authority host=%q auth=%q", r.Host(), r.Authority())
	}
	// server name without port.
	r2 := NewRequest(Env{ServerName: "only"})
	if r2.Authority() != "only" {
		t.Errorf("authority = %q", r2.Authority())
	}
}

func TestRequestContentType(t *testing.T) {
	r := NewRequest(Env{"CONTENT_TYPE": "text/html;charset=utf-8"})
	if r.ContentType() != "text/html;charset=utf-8" {
		t.Error("content type")
	}
	if r.MediaType() != "text/html" {
		t.Error("media type")
	}
	if r.ContentCharset() != "utf-8" {
		t.Errorf("charset = %q", r.ContentCharset())
	}
	if got := rubyInspect(r.MediaTypeParams()); got != `{"charset" => "utf-8"}` {
		t.Errorf("media type params = %s", got)
	}
	// Empty content type → "".
	if NewRequest(Env{"CONTENT_TYPE": ""}).ContentType() != "" {
		t.Error("empty content type")
	}
	if NewRequest(Env{}).ContentCharset() != "" {
		t.Error("no charset should be empty")
	}
}

func TestRequestXHR(t *testing.T) {
	if !NewRequest(Env{"HTTP_X_REQUESTED_WITH": "XMLHttpRequest"}).XHR() {
		t.Error("xhr true")
	}
	if NewRequest(Env{}).XHR() {
		t.Error("xhr false")
	}
}

func TestRequestGET(t *testing.T) {
	r := NewRequest(Env{QueryString: "a=1&a=2&b=3"})
	g, err := r.GET()
	if err != nil {
		t.Fatal(err)
	}
	if got := rubyInspect(g); got != `{"a" => ["1", "2"], "b" => "3"}` {
		t.Errorf("GET = %s", got)
	}
	// Memoised.
	g2, _ := r.GET()
	if g2 != g {
		t.Error("GET not memoised")
	}
}

func TestRequestGETInvalid(t *testing.T) {
	r := NewRequest(Env{QueryString: "a=%ZZ"})
	if _, err := r.GET(); err == nil {
		t.Error("expected error")
	}
}

func TestRequestPOST(t *testing.T) {
	env := Env{
		RequestMethod:  "POST",
		"CONTENT_TYPE": "application/x-www-form-urlencoded",
		RackInput:      &stringInput{data: []byte("a[b]=1&a[c]=2")},
	}
	r := NewRequest(env)
	p, err := r.POST()
	if err != nil {
		t.Fatal(err)
	}
	if got := rubyInspect(p); got != `{"a" => {"b" => "1", "c" => "2"}}` {
		t.Errorf("POST = %s", got)
	}
	// Memoised.
	p2, _ := r.POST()
	if p2 != p {
		t.Error("POST not memoised")
	}
	if env[RackRequestFormVars] != "a[b]=1&a[c]=2" {
		t.Errorf("form vars = %v", env[RackRequestFormVars])
	}
}

func TestRequestPOSTNoInput(t *testing.T) {
	r := NewRequest(Env{RequestMethod: "POST"})
	p, err := r.POST()
	if err != nil || p.Len() != 0 {
		t.Errorf("no-input POST = %v,%v", rubyInspect(p), err)
	}
}

func TestRequestPOSTNonForm(t *testing.T) {
	// A non-form content type yields empty params (body not read).
	r := NewRequest(Env{
		RequestMethod:  "POST",
		"CONTENT_TYPE": "application/json",
		RackInput:      &stringInput{data: []byte(`{"x":1}`)},
	})
	p, err := r.POST()
	if err != nil || p.Len() != 0 {
		t.Errorf("json POST = %v,%v", rubyInspect(p), err)
	}
}

func TestRequestPOSTPostNoContentType(t *testing.T) {
	// A POST with no content type is treated as form data.
	r := NewRequest(Env{
		RequestMethod: "POST",
		RackInput:     &stringInput{data: []byte("x=1")},
	})
	p, err := r.POST()
	if err != nil {
		t.Fatal(err)
	}
	if got := rubyInspect(p); got != `{"x" => "1"}` {
		t.Errorf("POST = %s", got)
	}
}

func TestRequestPOSTTrailingNUL(t *testing.T) {
	r := NewRequest(Env{
		RequestMethod:  "POST",
		"CONTENT_TYPE": "application/x-www-form-urlencoded",
		RackInput:      &stringInput{data: []byte("x=1\x00")},
	})
	p, err := r.POST()
	if err != nil {
		t.Fatal(err)
	}
	if got := rubyInspect(p); got != `{"x" => "1"}` {
		t.Errorf("POST = %s", got)
	}
}

func TestRequestPOSTReadError(t *testing.T) {
	r := NewRequest(Env{
		RequestMethod:  "POST",
		"CONTENT_TYPE": "application/x-www-form-urlencoded",
		RackInput:      &stringInput{err: errors.New("boom")},
	})
	if _, err := r.POST(); err == nil {
		t.Error("expected read error")
	}
}

func TestRequestPOSTParseError(t *testing.T) {
	r := NewRequest(Env{
		RequestMethod:  "POST",
		"CONTENT_TYPE": "application/x-www-form-urlencoded",
		RackInput:      &stringInput{data: []byte("a=%ZZ")},
	})
	if _, err := r.POST(); err == nil {
		t.Error("expected parse error")
	}
}

func TestRequestPOSTParseable(t *testing.T) {
	// multipart/related is parseable_data? but not form_data?.
	r := NewRequest(Env{
		RequestMethod:  "POST",
		"CONTENT_TYPE": "multipart/related",
		RackInput:      &stringInput{data: []byte("x=1")},
	})
	p, err := r.POST()
	if err != nil {
		t.Fatal(err)
	}
	if got := rubyInspect(p); got != `{"x" => "1"}` {
		t.Errorf("POST = %s", got)
	}
}

func TestRequestParams(t *testing.T) {
	r := NewRequest(Env{
		QueryString:    "a=1&shared=q",
		RequestMethod:  "POST",
		"CONTENT_TYPE": "application/x-www-form-urlencoded",
		RackInput:      &stringInput{data: []byte("b=2&shared=p")},
	})
	p, err := r.Params()
	if err != nil {
		t.Fatal(err)
	}
	// POST wins on "shared".
	if got := rubyInspect(p); got != `{"a" => "1", "shared" => "p", "b" => "2"}` {
		t.Errorf("params = %s", got)
	}
}

func TestRequestParamsGETError(t *testing.T) {
	r := NewRequest(Env{QueryString: "a=%ZZ"})
	if _, err := r.Params(); err == nil {
		t.Error("expected GET error to surface")
	}
}

func TestRequestParamsPOSTError(t *testing.T) {
	r := NewRequest(Env{
		QueryString:    "a=1",
		RequestMethod:  "POST",
		"CONTENT_TYPE": "application/x-www-form-urlencoded",
		RackInput:      &stringInput{err: errors.New("boom")},
	})
	if _, err := r.Params(); err == nil {
		t.Error("expected POST error to surface")
	}
}

func TestRequestFormDataMethodOverride(t *testing.T) {
	// The override original-method drives form_data? detection.
	r := NewRequest(Env{
		RackMethodOverrideOrigMethod: "POST",
		RequestMethod:                "PUT",
		RackInput:                    &stringInput{data: []byte("x=1")},
	})
	p, err := r.POST()
	if err != nil {
		t.Fatal(err)
	}
	if got := rubyInspect(p); got != `{"x" => "1"}` {
		t.Errorf("override POST = %s", got)
	}
}

func TestRequestCookies(t *testing.T) {
	r := NewRequest(Env{HTTPCookie: "a=1; b=2"})
	c := r.Cookies()
	if got := rubyInspect(c); got != `{"a" => "1", "b" => "2"}` {
		t.Errorf("cookies = %s", got)
	}
	// Memoised while the cookie string is unchanged.
	if r.Cookies() != c {
		t.Error("cookies not memoised")
	}
	// Changing the cookie header re-parses.
	r.SetHeader(HTTPCookie, "c=3")
	if got := rubyInspect(r.Cookies()); got != `{"c" => "3"}` {
		t.Errorf("re-parsed = %s", got)
	}
}

func TestRequestBody(t *testing.T) {
	in := &stringInput{data: []byte("hi")}
	r := NewRequest(Env{RackInput: in})
	if r.Body() != in {
		t.Error("Body")
	}
	if NewRequest(Env{}).Body() != nil {
		t.Error("nil body")
	}
}

func TestRequestIP(t *testing.T) {
	// Direct remote address, untrusted.
	r := NewRequest(Env{"REMOTE_ADDR": "8.8.8.8"})
	if r.IP() != "8.8.8.8" {
		t.Errorf("ip = %q", r.IP())
	}
	// Trusted proxy + forwarded-for client.
	r2 := NewRequest(Env{
		"REMOTE_ADDR":          "127.0.0.1",
		"HTTP_X_FORWARDED_FOR": "203.0.113.1, 10.0.0.1",
	})
	if r2.IP() != "203.0.113.1" {
		t.Errorf("forwarded ip = %q", r2.IP())
	}
	// All trusted: first remote returned.
	r3 := NewRequest(Env{"REMOTE_ADDR": "10.0.0.1, 10.0.0.2"})
	if r3.IP() != "10.0.0.1" {
		t.Errorf("all-trusted ip = %q", r3.IP())
	}
	// Empty.
	if NewRequest(Env{}).IP() != "" {
		t.Error("empty ip")
	}
}

func TestRequestIPForwardedAllTrusted(t *testing.T) {
	// Remote trusted, all forwarded-for also trusted: returns first forwarded.
	r := NewRequest(Env{
		"REMOTE_ADDR":          "127.0.0.1",
		"HTTP_X_FORWARDED_FOR": "10.0.0.1, 10.0.0.2",
	})
	if r.IP() != "10.0.0.1" {
		t.Errorf("ip = %q", r.IP())
	}
}

func TestServerAccessors(t *testing.T) {
	r := NewRequest(Env{ServerName: "s", ServerPort: "1"})
	if r.ServerName() != "s" || r.ServerPort() != "1" {
		t.Error("server accessors")
	}
}

func TestSplitAuthorityEdges(t *testing.T) {
	// Bare IPv6 without brackets but with a port-like trailing: lastIndex colon.
	host, addr, port := splitAuthority("h:notaport")
	if host != "h:notaport" || addr != "h:notaport" || port != 0 {
		t.Errorf("non-numeric port: %q %q %d", host, addr, port)
	}
	// Unterminated IPv6 bracket.
	if h, a, p := splitAuthority("[::1"); h != "" || a != "" || p != 0 {
		t.Errorf("unterminated: %q %q %d", h, a, p)
	}
	// IPv6 with junk after bracket (not a colon).
	if h, _, _ := splitAuthority("[::1]x"); h != "" {
		t.Errorf("junk after bracket should fail: %q", h)
	}
	// IPv6 bracket with non-numeric port.
	if h, _, _ := splitAuthority("[::1]:xx"); h != "" {
		t.Errorf("bad v6 port should fail: %q", h)
	}
	// Empty.
	if h, a, p := splitAuthority(""); h != "" || a != "" || p != 0 {
		t.Error("empty authority")
	}
}

func TestWrapIPv6(t *testing.T) {
	if wrapIPv6("::1") != "[::1]" {
		t.Error("should wrap")
	}
	if wrapIPv6("[::1]") != "[::1]" {
		t.Error("already wrapped")
	}
	if wrapIPv6("1.2.3.4") != "1.2.3.4" {
		t.Error("ipv4 unchanged")
	}
}

func TestSplitHeader(t *testing.T) {
	if got := splitHeader("a, b\tc d"); len(got) != 4 {
		t.Errorf("split = %v", got)
	}
	if splitHeader("") != nil {
		t.Error("empty split should be nil")
	}
}

func TestAtoiRubyStrict(t *testing.T) {
	if atoiRubyStrict("123") != 123 {
		t.Error("digits")
	}
	if atoiRubyStrict("") != -1 || atoiRubyStrict("12a") != -1 {
		t.Error("invalid should be -1")
	}
}
