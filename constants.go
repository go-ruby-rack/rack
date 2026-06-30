// Copyright (c) the go-ruby-rack/rack authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rack

// Request env keys (Rack::* constants from constants.rb).
const (
	HTTPHost       = "HTTP_HOST"
	HTTPPort       = "HTTP_PORT"
	HTTPS          = "HTTPS"
	PathInfo       = "PATH_INFO"
	RequestMethod  = "REQUEST_METHOD"
	RequestPath    = "REQUEST_PATH"
	ScriptName     = "SCRIPT_NAME"
	QueryString    = "QUERY_STRING"
	ServerProtocol = "SERVER_PROTOCOL"
	ServerName     = "SERVER_NAME"
	ServerPort     = "SERVER_PORT"
	HTTPCookie     = "HTTP_COOKIE"

	// Response header keys (already lower-case, per the Rack 3 SPEC).
	CacheControl     = "cache-control"
	ContentLengthKey = "content-length"
	ContentTypeKey   = "content-type"
	ETagKey          = "etag"
	Expires          = "expires"
	SetCookie        = "set-cookie"
	TransferEncoding = "transfer-encoding"

	// HTTP method verbs.
	MethodGet     = "GET"
	MethodPost    = "POST"
	MethodPut     = "PUT"
	MethodPatch   = "PATCH"
	MethodDelete  = "DELETE"
	MethodHead    = "HEAD"
	MethodOptions = "OPTIONS"
	MethodConnect = "CONNECT"
	MethodLink    = "LINK"
	MethodUnlink  = "UNLINK"
	MethodTrace   = "TRACE"

	// Rack environment variables.
	RackURLScheme                = "rack.url_scheme"
	RackInput                    = "rack.input"
	RackErrors                   = "rack.errors"
	RackSession                  = "rack.session"
	RackRequestQueryHash         = "rack.request.query_hash"
	RackRequestQueryString       = "rack.request.query_string"
	RackRequestCookieHash        = "rack.request.cookie_hash"
	RackRequestCookieString      = "rack.request.cookie_string"
	RackRequestFormHash          = "rack.request.form_hash"
	RackRequestFormInput         = "rack.request.form_input"
	RackRequestFormVars          = "rack.request.form_vars"
	RackMethodOverrideOrigMethod = "rack.methodoverride.original_method"
)

// chunked is the transfer-encoding value Response treats specially in finish.
const chunked = "chunked"
