// Copyright (c) the go-ruby-rack/rack authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rack

import (
	"reflect"
	"testing"
)

func TestResponseFinishString(t *testing.T) {
	r := NewResponseString("Hello", 200, nil)
	status, headers, body := r.Finish()
	if status != 200 {
		t.Errorf("status = %d", status)
	}
	if headers.Get(ContentLengthKey) != "5" {
		t.Errorf("content-length = %v", headers.Get(ContentLengthKey))
	}
	if !reflect.DeepEqual(body, []string{"Hello"}) {
		t.Errorf("body = %v", body)
	}
}

func TestResponseFinishMultibyteLength(t *testing.T) {
	// content-length is the byte length, not the char length.
	r := NewResponseString("café", 200, nil)
	_, headers, _ := r.Finish()
	if headers.Get(ContentLengthKey) != "5" {
		t.Errorf("content-length = %v, want 5", headers.Get(ContentLengthKey))
	}
}

func TestResponseNilBody(t *testing.T) {
	r := NewResponse(nil, 200, nil)
	_, headers, body := r.Finish()
	// length unknown → no content-length set.
	if _, ok := headers.GetOK(ContentLengthKey); ok {
		t.Error("nil body should not set content-length")
	}
	if len(body) != 0 {
		t.Errorf("body = %v", body)
	}
}

func TestResponseWrite(t *testing.T) {
	r := NewResponse(nil, 200, nil)
	r.Write("ab")
	r.Write("cde")
	_, headers, body := r.Finish()
	if !reflect.DeepEqual(body, []string{"ab", "cde"}) {
		t.Errorf("body = %v", body)
	}
	if headers.Get(ContentLengthKey) != "5" {
		t.Errorf("content-length = %v", headers.Get(ContentLengthKey))
	}
}

func TestResponseWriteAppendsToKnownLength(t *testing.T) {
	r := NewResponseString("ab", 200, nil)
	r.Write("cd")
	_, headers, _ := r.Finish()
	if headers.Get(ContentLengthKey) != "4" {
		t.Errorf("content-length = %v", headers.Get(ContentLengthKey))
	}
}

func TestResponseNoEntityBody(t *testing.T) {
	r := NewResponseString("ignored", 204, nil)
	r.SetContentType("text/plain")
	status, headers, body := r.Finish()
	if status != 204 {
		t.Error("status")
	}
	if headers.Has(ContentTypeKey) || headers.Has(ContentLengthKey) {
		t.Error("204 should strip content-type and content-length")
	}
	if len(body) != 0 {
		t.Errorf("204 body = %v", body)
	}
}

func TestResponseChunked(t *testing.T) {
	r := NewResponseString("data", 200, nil)
	r.SetHeader(TransferEncoding, "chunked")
	_, headers, _ := r.Finish()
	// chunked → no content-length.
	if headers.Has(ContentLengthKey) {
		t.Error("chunked should not set content-length")
	}
}

func TestResponseToA(t *testing.T) {
	r := NewResponseString("x", 201, nil)
	s, _, _ := r.ToA()
	if s != 201 {
		t.Error("ToA status")
	}
}

func TestResponseTupleConstructors(t *testing.T) {
	h := NewHeaders()
	h.Set("X", "1")
	r := ResponseTuple(202, h, []string{"body"})
	s, headers, body := r.Finish()
	if s != 202 || headers.Get("x") != "1" || !reflect.DeepEqual(body, []string{"body"}) {
		t.Errorf("tuple = %d %v %v", s, headers.Get("x"), body)
	}
	// Headers are copied, not aliased.
	h.Set("Y", "2")
	if headers.Has("y") {
		t.Error("headers should be copied")
	}
}

func TestResponseHeaderHelpers(t *testing.T) {
	r := NewResponse(nil, 200, nil)
	r.SetHeader("X-A", "1")
	if !r.HasHeader("x-a") || r.GetHeader("X-A") != "1" {
		t.Error("set/get/has header")
	}
	r.DeleteHeader("X-A")
	if r.HasHeader("X-A") {
		t.Error("delete header")
	}
}

func TestResponseAddHeader(t *testing.T) {
	r := NewResponse(nil, 200, nil)
	r.AddHeader("Vary", "accept-encoding")
	if r.GetHeader("vary") != "accept-encoding" {
		t.Errorf("first = %v", r.GetHeader("vary"))
	}
	r.AddHeader("Vary", "cookie")
	if !reflect.DeepEqual(r.GetHeader("vary"), []any{"accept-encoding", "cookie"}) {
		t.Errorf("second = %v", r.GetHeader("vary"))
	}
	r.AddHeader("Vary", "x")
	if !reflect.DeepEqual(r.GetHeader("vary"), []any{"accept-encoding", "cookie", "x"}) {
		t.Errorf("third = %v", r.GetHeader("vary"))
	}
	// nil value is a no-op returning current.
	if got := r.AddHeader("Vary", nil); !reflect.DeepEqual(got, []any{"accept-encoding", "cookie", "x"}) {
		t.Errorf("nil add = %v", got)
	}
}

func TestResponseContentTypeAndMedia(t *testing.T) {
	r := NewResponse(nil, 200, nil)
	r.SetContentType("text/html;charset=utf-8")
	if r.ContentType() != "text/html;charset=utf-8" {
		t.Error("content type")
	}
	if r.MediaType() != "text/html" {
		t.Error("media type")
	}
	if got := rubyInspect(r.MediaTypeParams()); got != `{"charset" => "utf-8"}` {
		t.Errorf("media params = %s", got)
	}
	// Unset content type returns "".
	if NewResponse(nil, 200, nil).ContentType() != "" {
		t.Error("unset content type")
	}
}

func TestResponseLocationRedirect(t *testing.T) {
	r := NewResponse(nil, 200, nil)
	r.Redirect("/elsewhere", 0) // default 302
	if r.Status() != 302 || r.Location() != "/elsewhere" {
		t.Errorf("redirect default = %d %q", r.Status(), r.Location())
	}
	r.Redirect("/perm", 301)
	if r.Status() != 301 {
		t.Error("redirect explicit status")
	}
	if !r.IsRedirect() {
		t.Error("should be redirect")
	}
	// Unset location.
	if NewResponse(nil, 200, nil).Location() != "" {
		t.Error("unset location")
	}
}

func TestResponseStatusSetters(t *testing.T) {
	r := NewResponse(nil, 200, nil)
	r.SetStatus(404)
	if r.Status() != 404 {
		t.Error("set status")
	}
}

func TestResponseSetCookie(t *testing.T) {
	r := NewResponse(nil, 200, nil)
	if err := r.SetCookie("foo", CookieValue{Value: "bar"}); err != nil {
		t.Fatal(err)
	}
	if r.GetHeader(SetCookie) != "foo=bar" {
		t.Errorf("set cookie = %v", r.GetHeader(SetCookie))
	}
	if err := r.SetCookie("baz", CookieValue{Value: "qux"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.GetHeader(SetCookie), []any{"foo=bar", "baz=qux"}) {
		t.Errorf("two cookies = %v", r.GetHeader(SetCookie))
	}
	// Error path.
	if err := r.SetCookie("bad key", CookieValue{}); err == nil {
		t.Error("expected invalid key error")
	}
}

func TestResponseDeleteCookie(t *testing.T) {
	r := NewResponse(nil, 200, nil)
	if err := r.DeleteCookie("foo", CookieValue{}); err != nil {
		t.Fatal(err)
	}
	if r.GetHeader(SetCookie) != "foo="+DeleteCookieHeaderValue {
		t.Errorf("delete cookie = %v", r.GetHeader(SetCookie))
	}
	if err := r.DeleteCookie("bar", CookieValue{}); err != nil {
		t.Fatal(err)
	}
	if arr, ok := r.GetHeader(SetCookie).([]any); !ok || len(arr) != 2 {
		t.Errorf("two deletes = %v", r.GetHeader(SetCookie))
	}
	if err := r.DeleteCookie("bad key", CookieValue{}); err == nil {
		t.Error("expected error")
	}
}

func TestResponseEmptyAndContentLength(t *testing.T) {
	r := NewResponse(nil, 200, nil)
	if !r.Empty() {
		t.Error("nil body should be empty")
	}
	r.Write("x")
	if r.Empty() {
		t.Error("after write not empty")
	}
	// ContentLength reads the header after Finish.
	r.Finish()
	if r.ContentLength() != 1 {
		t.Errorf("content length = %d", r.ContentLength())
	}
	if NewResponse(nil, 200, nil).ContentLength() != -1 {
		t.Error("unset content length should be -1")
	}
}

func TestResponseBody(t *testing.T) {
	r := NewResponse([]string{"a", "b"}, 200, nil)
	if !reflect.DeepEqual(r.Body(), []string{"a", "b"}) {
		t.Errorf("body = %v", r.Body())
	}
}

func TestResponsePredicates(t *testing.T) {
	type pred struct {
		status int
		fn     func(*Response) bool
		want   bool
	}
	preds := []pred{
		{99, (*Response).Invalid, true},
		{200, (*Response).Invalid, false},
		{600, (*Response).Invalid, true},
		{100, (*Response).Informational, true},
		{200, (*Response).Successful, true},
		{301, (*Response).Redirection, true},
		{404, (*Response).ClientError, true},
		{500, (*Response).ServerError, true},
		{200, (*Response).OK, true},
		{201, (*Response).Created, true},
		{202, (*Response).Accepted, true},
		{204, (*Response).NoContent, true},
		{301, (*Response).MovedPermanently, true},
		{400, (*Response).BadRequest, true},
		{401, (*Response).Unauthorized, true},
		{403, (*Response).Forbidden, true},
		{404, (*Response).NotFound, true},
		{405, (*Response).MethodNotAllowed, true},
		{406, (*Response).NotAcceptable, true},
		{408, (*Response).RequestTimeout, true},
		{412, (*Response).PreconditionFailed, true},
		{422, (*Response).Unprocessable, true},
		{200, (*Response).IsRedirect, false},
		{303, (*Response).IsRedirect, true},
	}
	for _, p := range preds {
		r := NewResponse(nil, p.status, nil)
		if got := p.fn(r); got != p.want {
			t.Errorf("predicate at status %d = %v, want %v", p.status, got, p.want)
		}
	}
}
