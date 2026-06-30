// Copyright (c) the go-ruby-rack/rack authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rack

import "strings"

// Env is a Rack environment: a string-keyed map of CGI-style variables and
// rack.* entries. It is the substrate Request reads from and writes to. Values
// are usually strings; rack.input is an [Input].
type Env map[string]any

// Input is the body-reading seam (env["rack.input"]). The host supplies an
// implementation backed by whatever IO it has; this package never reads the
// network itself. It mirrors the subset of the Ruby IO contract Rack relies on.
type Input interface {
	// Read returns up to n bytes, or all remaining bytes when n < 0. It returns
	// nil at EOF with no bytes read, matching IO#read's nil-at-EOF behaviour.
	Read(n int) ([]byte, error)
}

// Request provides a convenient, stateless interface over a Rack [Env]. It is a
// faithful port of the pure-compute parts of Rack::Request; the env passed in is
// referenced directly and may be mutated by the setters and the GET/POST
// memoisation, exactly like the Ruby class.
type Request struct {
	env Env
}

// NewRequest wraps env in a Request. env must be non-nil.
func NewRequest(env Env) *Request {
	return &Request{env: env}
}

// Env returns the underlying environment.
func (r *Request) Env() Env { return r.env }

// GetHeader returns the env value for name (string-typed), or "" if absent or
// non-string. Use GetHeaderRaw for the untyped value.
func (r *Request) GetHeader(name string) string {
	if v, ok := r.env[name].(string); ok {
		return v
	}
	return ""
}

// GetHeaderRaw returns the raw env value for name and whether it was present.
func (r *Request) GetHeaderRaw(name string) (any, bool) {
	v, ok := r.env[name]
	return v, ok
}

// HasHeader reports whether name is set in the env.
func (r *Request) HasHeader(name string) bool {
	_, ok := r.env[name]
	return ok
}

// SetHeader sets name to v in the env.
func (r *Request) SetHeader(name string, v any) { r.env[name] = v }

// DeleteHeader removes name from the env, returning its prior value.
func (r *Request) DeleteHeader(name string) any {
	v := r.env[name]
	delete(r.env, name)
	return v
}

// RequestMethod returns the REQUEST_METHOD (e.g. "GET").
func (r *Request) RequestMethod() string { return r.GetHeader(RequestMethod) }

// ScriptName returns SCRIPT_NAME, defaulting to "".
func (r *Request) ScriptName() string { return r.GetHeader(ScriptName) }

// PathInfo returns PATH_INFO, defaulting to "".
func (r *Request) PathInfo() string { return r.GetHeader(PathInfo) }

// QueryString returns QUERY_STRING, defaulting to "".
func (r *Request) QueryString() string { return r.GetHeader(QueryString) }

// ServerName returns SERVER_NAME.
func (r *Request) ServerName() string { return r.GetHeader(ServerName) }

// ServerPort returns SERVER_PORT.
func (r *Request) ServerPort() string { return r.GetHeader(ServerPort) }

// Body returns the rack.input Input, or nil if unset.
func (r *Request) Body() Input {
	if in, ok := r.env[RackInput].(Input); ok {
		return in
	}
	return nil
}

// Method predicates.
func (r *Request) IsGet() bool     { return r.RequestMethod() == MethodGet }
func (r *Request) IsPost() bool    { return r.RequestMethod() == MethodPost }
func (r *Request) IsPut() bool     { return r.RequestMethod() == MethodPut }
func (r *Request) IsPatch() bool   { return r.RequestMethod() == MethodPatch }
func (r *Request) IsDelete() bool  { return r.RequestMethod() == MethodDelete }
func (r *Request) IsHead() bool    { return r.RequestMethod() == MethodHead }
func (r *Request) IsOptions() bool { return r.RequestMethod() == MethodOptions }
func (r *Request) IsTrace() bool   { return r.RequestMethod() == MethodTrace }
func (r *Request) IsLink() bool    { return r.RequestMethod() == MethodLink }
func (r *Request) IsUnlink() bool  { return r.RequestMethod() == MethodUnlink }

// XHR reports whether HTTP_X_REQUESTED_WITH is "XMLHttpRequest", matching
// Request#xhr?.
func (r *Request) XHR() bool {
	return r.GetHeader("HTTP_X_REQUESTED_WITH") == "XMLHttpRequest"
}

// ContentType returns CONTENT_TYPE, or "" if absent or empty (Request#content_type
// returns nil there; we use "").
func (r *Request) ContentType() string {
	ct := r.GetHeader("CONTENT_TYPE")
	if ct == "" {
		return ""
	}
	return ct
}

// MediaType returns the media type of CONTENT_TYPE, matching Request#media_type.
func (r *Request) MediaType() string { return MediaTypeOf(r.ContentType()) }

// MediaTypeParams returns the CONTENT_TYPE parameters, matching
// Request#media_type_params.
func (r *Request) MediaTypeParams() *Params { return MediaTypeParams(r.ContentType()) }

// ContentCharset returns the charset media-type parameter, or "" if none.
func (r *Request) ContentCharset() string {
	if v, ok := r.MediaTypeParams().Get("charset"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// Scheme returns the request scheme, honouring HTTPS, X-Forwarded-SSL and
// rack.url_scheme, matching Request#scheme (the forwarded-Proto header path is
// covered by ForwardedScheme below).
func (r *Request) Scheme() string {
	switch {
	case r.GetHeader(HTTPS) == "on":
		return "https"
	case r.GetHeader("HTTP_X_FORWARDED_SSL") == "on":
		return "https"
	default:
		if fs := r.forwardedScheme(); fs != "" {
			return fs
		}
		return r.GetHeader(RackURLScheme)
	}
}

var allowedSchemes = map[string]bool{"https": true, "http": true, "wss": true, "ws": true}

// forwardedScheme checks the X-Forwarded-Proto then X-Forwarded-Scheme headers
// (the default x_forwarded_proto_priority), returning the last allowed scheme.
func (r *Request) forwardedScheme() string {
	for _, hdr := range []string{"HTTP_X_FORWARDED_PROTO", "HTTP_X_FORWARDED_SCHEME"} {
		vals := splitHeader(r.GetHeader(hdr))
		for i := len(vals) - 1; i >= 0; i-- {
			if allowedSchemes[vals[i]] {
				return vals[i]
			}
		}
	}
	return ""
}

// SSL reports whether the scheme is https or wss, matching Request#ssl?.
func (r *Request) SSL() bool {
	s := r.Scheme()
	return s == "https" || s == "wss"
}

// Authority returns the request authority (host[:port]), preferring HTTP_HOST
// then SERVER_NAME/SERVER_PORT, matching Request#authority (without the
// Forwarded header, which the host can layer on).
func (r *Request) Authority() string {
	if h := r.GetHeader(HTTPHost); h != "" {
		return h
	}
	return r.serverAuthority()
}

func (r *Request) serverAuthority() string {
	host := r.ServerName()
	port := r.ServerPort()
	if host == "" {
		return ""
	}
	if port != "" {
		return host + ":" + port
	}
	return host
}

// Host returns the host portion of the authority, matching Request#host.
func (r *Request) Host() string {
	host, _, _ := splitAuthority(r.Authority())
	return host
}

// Hostname returns the address portion (IPv6 brackets removed), matching
// Request#hostname.
func (r *Request) Hostname() string {
	_, addr, _ := splitAuthority(r.Authority())
	return addr
}

var defaultPorts = map[string]int{"http": 80, "https": 443, "coffee": 80}

// Port returns the request port, falling back through the authority, the
// scheme's default port and SERVER_PORT, matching Request#port.
func (r *Request) Port() int {
	if auth := r.Authority(); auth != "" {
		if _, _, port := splitAuthority(auth); port > 0 {
			return port
		}
	}
	if dp, ok := defaultPorts[r.Scheme()]; ok {
		return dp
	}
	return atoiRuby(r.ServerPort())
}

// HostWithPort returns the host, including the port only when it differs from
// the scheme's default, matching Request#host_with_port.
func (r *Request) HostWithPort() string {
	auth := r.Authority()
	host, _, port := splitAuthority(auth)
	if port == defaultPorts[r.Scheme()] {
		return host
	}
	return auth
}

// BaseURL returns scheme://host[:port], matching Request#base_url.
func (r *Request) BaseURL() string {
	return r.Scheme() + "://" + r.HostWithPort()
}

// Path returns SCRIPT_NAME + PATH_INFO, matching Request#path.
func (r *Request) Path() string { return r.ScriptName() + r.PathInfo() }

// Fullpath returns the path plus the query string when present, matching
// Request#fullpath.
func (r *Request) Fullpath() string {
	qs := r.QueryString()
	if qs == "" {
		return r.Path()
	}
	return r.Path() + "?" + qs
}

// URL returns the reconstructed request URL, matching Request#url.
func (r *Request) URL() string { return r.BaseURL() + r.Fullpath() }

// IP returns the originating client IP, walking REMOTE_ADDR and the
// X-Forwarded-For chain past trusted proxies, matching Request#ip.
func (r *Request) IP() string {
	remote := splitHeader(r.GetHeader("REMOTE_ADDR"))
	for i := len(remote) - 1; i >= 0; i-- {
		if !TrustedProxy(remote[i]) {
			return remote[i]
		}
	}
	if ffor := r.forwardedFor(); len(ffor) > 0 {
		for i := len(ffor) - 1; i >= 0; i-- {
			if !TrustedProxy(ffor[i]) {
				return ffor[i]
			}
		}
		return ffor[0]
	}
	if len(remote) > 0 {
		return remote[0]
	}
	return ""
}

func (r *Request) forwardedFor() []string {
	value := r.GetHeader("HTTP_X_FORWARDED_FOR")
	if value == "" {
		return nil
	}
	var out []string
	for _, a := range splitHeader(value) {
		_, addr, _ := splitAuthority(wrapIPv6(a))
		out = append(out, addr)
	}
	return out
}

// GET returns the parsed query-string parameters, memoised into
// rack.request.query_hash, matching Request#GET.
func (r *Request) GET() (*Params, error) {
	if cached, ok := r.env[RackRequestQueryHash].(*Params); ok {
		return cached, nil
	}
	p, err := ParseQuery(r.QueryString(), "&")
	if err != nil {
		return nil, err
	}
	r.env[RackRequestQueryHash] = p
	return p, nil
}

// POST parses the request body as form data when the content type is a
// form-data media type, memoising into rack.request.form_hash. It returns an
// empty Params when there is no rack.input or the body is not form data,
// matching Request#POST. The body is read through the [Input] seam.
func (r *Request) POST() (*Params, error) {
	if cached, ok := r.env[RackRequestFormHash].(*Params); ok {
		return cached, nil
	}
	in := r.Body()
	var result *Params
	switch {
	case in == nil:
		result = NewParams()
	case r.isFormData() || r.isParseableData():
		data, err := in.Read(-1)
		if err != nil {
			return nil, err
		}
		body := string(data)
		// Strip a trailing NUL (the Safari Ajax workaround).
		if strings.HasSuffix(body, "\x00") {
			body = body[:len(body)-1]
		}
		r.env[RackRequestFormVars] = body
		p, err := ParseNestedQuery(body, "&", DefaultParamDepthLimit)
		if err != nil {
			return nil, err
		}
		result = p
	default:
		result = NewParams()
	}
	r.env[RackRequestFormHash] = result
	return result, nil
}

// Params returns the union of GET and POST, with POST winning on collisions,
// matching Request#params.
func (r *Request) Params() (*Params, error) {
	get, err := r.GET()
	if err != nil {
		return nil, err
	}
	post, err := r.POST()
	if err != nil {
		return nil, err
	}
	return get.Merge(post), nil
}

var formDataMediaTypes = map[string]bool{
	"application/x-www-form-urlencoded": true,
	"multipart/form-data":               true,
}

var parseableDataMediaTypes = map[string]bool{
	"multipart/related": true,
	"multipart/mixed":   true,
}

// isFormData mirrors Request#form_data?: a POST with no content type, or a
// recognised form-data media type.
func (r *Request) isFormData() bool {
	mt := r.MediaType()
	meth := r.GetHeader(RackMethodOverrideOrigMethod)
	if meth == "" {
		meth = r.RequestMethod()
	}
	return (meth == MethodPost && mt == "") || formDataMediaTypes[mt]
}

func (r *Request) isParseableData() bool {
	return parseableDataMediaTypes[r.MediaType()]
}

// Cookies returns the parsed Cookie header, memoised into
// rack.request.cookie_hash, matching Request#cookies.
func (r *Request) Cookies() *Params {
	str := r.GetHeader(HTTPCookie)
	if cached, ok := r.env[RackRequestCookieHash].(*Params); ok {
		if prev, _ := r.env[RackRequestCookieString].(string); prev == str {
			return cached
		}
	}
	parsed := ParseCookiesHeader(str)
	r.env[RackRequestCookieHash] = parsed
	r.env[RackRequestCookieString] = str
	return parsed
}

// splitHeader splits on commas, spaces and tabs, dropping empties, matching
// Request#split_header.
func splitHeader(value string) []string {
	if value == "" {
		return nil
	}
	value = strings.TrimSpace(value)
	var out []string
	for _, f := range strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t'
	}) {
		out = append(out, f)
	}
	return out
}

// wrapIPv6 wraps an unbracketed IPv6 address in square brackets, matching
// Request#wrap_ipv6.
func wrapIPv6(host string) string {
	if !strings.HasPrefix(host, "[") && strings.Count(host, ":") > 1 {
		return "[" + host + "]"
	}
	return host
}

// splitAuthority parses an authority into (host, address, port), where host
// retains IPv6 brackets and address strips them, matching Request#split_authority.
// port is 0 when absent.
func splitAuthority(authority string) (host, address string, port int) {
	if authority == "" {
		return "", "", 0
	}
	rest := authority
	if strings.HasPrefix(rest, "[") {
		end := strings.IndexByte(rest, ']')
		if end < 0 {
			return "", "", 0
		}
		address = rest[1:end]
		host = rest[:end+1]
		rest = rest[end+1:]
		if rest == "" {
			return host, address, 0
		}
		if rest[0] != ':' {
			return "", "", 0
		}
		p := atoiRubyStrict(rest[1:])
		if p < 0 {
			return "", "", 0
		}
		return host, address, p
	}
	if colon := strings.LastIndexByte(rest, ':'); colon >= 0 {
		portStr := rest[colon+1:]
		p := atoiRubyStrict(portStr)
		if p >= 0 {
			host = rest[:colon]
			return host, host, p
		}
		// Not a numeric port: whole thing is the host.
		return rest, rest, 0
	}
	return rest, rest, 0
}

// atoiRubyStrict parses an all-digit string, returning -1 if it is empty or
// contains a non-digit (so a non-numeric ":port" is rejected).
func atoiRubyStrict(s string) int {
	if s == "" {
		return -1
	}
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return -1
		}
		n = n*10 + int(s[i]-'0')
	}
	return n
}
