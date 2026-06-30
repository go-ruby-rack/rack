// Copyright (c) the go-ruby-rack/rack authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rack

import "strconv"

// Response is a convenient builder for a Rack response — a faithful port of the
// pure-compute parts of Rack::Response. It accumulates a status, a [Headers] and
// a buffered body, and produces the SPEC `[status, headers, body]` tuple via
// [Response.Finish] / [Response.ToA].
type Response struct {
	status   int
	headers  *Headers
	body     []string
	length   int  // byte length of the buffered body
	hasLen   bool // whether length is known (nil in Ruby = !hasLen)
	buffered bool
}

// NewResponse builds a Response with the given body, status and headers, like
// Rack::Response.new(body, status, headers). A nil body yields an empty,
// length-unknown buffered response; a single string body is buffered with its
// byte length recorded. status defaults are the caller's responsibility (pass
// 200 for the Ruby default). headers may be nil.
func NewResponse(body []string, status int, headers *Headers) *Response {
	h := NewHeaders()
	if headers != nil {
		headers.Each(func(k string, v any) bool { h.Set(k, v); return true })
	}
	r := &Response{
		status:   status,
		headers:  h,
		buffered: true,
	}
	if body == nil {
		r.body = []string{}
		r.hasLen = false
	} else {
		r.body = append([]string{}, body...)
		r.length = 0
		for _, part := range body {
			r.length += len(part)
		}
		r.hasLen = true
	}
	return r
}

// NewResponseString is the common case of a single string body.
func NewResponseString(body string, status int, headers *Headers) *Response {
	return NewResponse([]string{body}, status, headers)
}

// ResponseTuple builds a Response from the SPEC `[status, headers, body]`
// argument order, matching Rack::Response.[](status, headers, body).
func ResponseTuple(status int, headers *Headers, body []string) *Response {
	return NewResponse(body, status, headers)
}

// Status returns the response status code.
func (r *Response) Status() int { return r.status }

// SetStatus sets the response status code (Response#status=).
func (r *Response) SetStatus(status int) { r.status = status }

// Headers returns the response Headers.
func (r *Response) Headers() *Headers { return r.headers }

// HasHeader reports whether key is set.
func (r *Response) HasHeader(key string) bool { return r.headers.Has(key) }

// GetHeader returns the value for key.
func (r *Response) GetHeader(key string) any { return r.headers.Get(key) }

// SetHeader sets key to value.
func (r *Response) SetHeader(key string, value any) { r.headers.Set(key, value) }

// DeleteHeader removes key.
func (r *Response) DeleteHeader(key string) { r.headers.Delete(key) }

// AddHeader adds value to key, promoting an existing single value to a list when
// a second value is added, matching Response#add_header. A nil value is a no-op
// (returns the current value).
func (r *Response) AddHeader(key string, value any) any {
	if value == nil {
		return r.headers.Get(key)
	}
	v := toStr(value)
	if existing, ok := r.headers.GetOK(key); ok {
		if arr, isArr := existing.([]any); isArr {
			r.headers.Set(key, append(arr, v))
		} else {
			r.headers.Set(key, []any{existing, v})
		}
	} else {
		r.headers.Set(key, v)
	}
	return r.headers.Get(key)
}

// Write appends a chunk to the buffered body, updating the byte length, matching
// Response#write.
func (r *Response) Write(chunk string) {
	r.body = append(r.body, chunk)
	if r.hasLen {
		r.length += len(chunk)
	} else if r.buffered {
		r.length = len(chunk)
		r.hasLen = true
	}
}

// Body returns the buffered body parts.
func (r *Response) Body() []string { return r.body }

// ContentType returns the content-type header value, or "" if unset.
func (r *Response) ContentType() string {
	if v, ok := r.headers.Get(ContentTypeKey).(string); ok {
		return v
	}
	return ""
}

// SetContentType sets the content-type header (Response#content_type=).
func (r *Response) SetContentType(ct string) { r.headers.Set(ContentTypeKey, ct) }

// MediaType returns the media type of the content-type header.
func (r *Response) MediaType() string { return MediaTypeOf(r.ContentType()) }

// MediaTypeParams returns the content-type parameters.
func (r *Response) MediaTypeParams() *Params { return MediaTypeParams(r.ContentType()) }

// Location returns the location header value, or "" if unset.
func (r *Response) Location() string {
	if v, ok := r.headers.Get("location").(string); ok {
		return v
	}
	return ""
}

// SetLocation sets the location header.
func (r *Response) SetLocation(loc string) { r.headers.Set("location", loc) }

// Redirect sets the status (default 302) and location header, matching
// Response#redirect.
func (r *Response) Redirect(target string, status int) {
	if status == 0 {
		status = 302
	}
	r.status = status
	r.SetLocation(target)
}

// SetCookie appends a set-cookie header for key/cookie, matching
// Response#set_cookie.
func (r *Response) SetCookie(key string, cookie CookieValue) error {
	enc, err := MakeCookieHeader(key, cookie)
	if err != nil {
		return err
	}
	r.AddHeader(SetCookie, enc)
	return nil
}

// DeleteCookie sets an expiring set-cookie header for key, matching
// Response#delete_cookie.
func (r *Response) DeleteCookie(key string, cookie CookieValue) error {
	enc, err := MakeDeleteCookieHeader(key, cookie)
	if err != nil {
		return err
	}
	if existing, ok := r.headers.GetOK(SetCookie); ok {
		r.headers.Set(SetCookie, append(toAnyList(existing), enc))
	} else {
		r.headers.Set(SetCookie, enc)
	}
	return nil
}

// chunkedResp reports whether the transfer-encoding header is "chunked".
func (r *Response) chunkedResp() bool {
	v, _ := r.headers.Get(TransferEncoding).(string)
	return v == chunked
}

// noEntityBody reports whether this response must not carry an entity body,
// matching Response#no_entity_body? (the body is enumerable here by construction).
func (r *Response) noEntityBody() bool {
	return StatusWithNoEntityBody(r.status)
}

// Finish produces the SPEC `[status, headers, body]` tuple, matching
// Response#finish. For a no-entity-body status it strips content-type and
// content-length and returns an empty body; otherwise it sets content-length
// from the buffered length (unless chunked).
func (r *Response) Finish() (status int, headers *Headers, body []string) {
	if r.noEntityBody() {
		r.headers.Delete(ContentTypeKey)
		r.headers.Delete(ContentLengthKey)
		return r.status, r.headers, []string{}
	}
	if r.hasLen && !r.chunkedResp() {
		r.headers.Set(ContentLengthKey, strconv.Itoa(r.length))
	}
	return r.status, r.headers, r.body
}

// ToA is an alias for Finish (Response#to_a).
func (r *Response) ToA() (int, *Headers, []string) { return r.Finish() }

// Empty reports whether the buffered body has no parts, matching Response#empty?.
func (r *Response) Empty() bool { return len(r.body) == 0 }

// ContentLength returns the content-length header as an int, or -1 if unset.
func (r *Response) ContentLength() int {
	v, ok := r.headers.Get(ContentLengthKey).(string)
	if !ok {
		return -1
	}
	return atoiRuby(v)
}

// Status-class predicates (Response::Helpers).
func (r *Response) Invalid() bool       { return r.status < 100 || r.status >= 600 }
func (r *Response) Informational() bool { return r.status >= 100 && r.status < 200 }
func (r *Response) Successful() bool    { return r.status >= 200 && r.status < 300 }
func (r *Response) Redirection() bool   { return r.status >= 300 && r.status < 400 }
func (r *Response) ClientError() bool   { return r.status >= 400 && r.status < 500 }
func (r *Response) ServerError() bool   { return r.status >= 500 && r.status < 600 }

func (r *Response) OK() bool                 { return r.status == 200 }
func (r *Response) Created() bool            { return r.status == 201 }
func (r *Response) Accepted() bool           { return r.status == 202 }
func (r *Response) NoContent() bool          { return r.status == 204 }
func (r *Response) MovedPermanently() bool   { return r.status == 301 }
func (r *Response) BadRequest() bool         { return r.status == 400 }
func (r *Response) Unauthorized() bool       { return r.status == 401 }
func (r *Response) Forbidden() bool          { return r.status == 403 }
func (r *Response) NotFound() bool           { return r.status == 404 }
func (r *Response) MethodNotAllowed() bool   { return r.status == 405 }
func (r *Response) NotAcceptable() bool      { return r.status == 406 }
func (r *Response) RequestTimeout() bool     { return r.status == 408 }
func (r *Response) PreconditionFailed() bool { return r.status == 412 }
func (r *Response) Unprocessable() bool      { return r.status == 422 }

// IsRedirect reports whether the status is a redirect code, matching
// Response#redirect?.
func (r *Response) IsRedirect() bool {
	switch r.status {
	case 301, 302, 303, 307, 308:
		return true
	}
	return false
}
