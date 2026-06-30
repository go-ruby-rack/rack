// Copyright (c) the go-ruby-rack/rack authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rack

import "strings"

// Headers is the Go analogue of Rack::Headers — an insertion-ordered map that
// downcases every string key on the way in, so a Rack-2-style "Content-Type" and
// a Rack-3-style "content-type" address the same slot. A value is typically a
// string or a []any (a header with multiple values, e.g. several set-cookie
// lines). Non-string keys are not used by Rack response headers, so Headers
// keys are always strings.
type Headers struct {
	keys   []string
	values map[string]any
}

// NewHeaders returns an empty Headers.
func NewHeaders() *Headers {
	return &Headers{values: map[string]any{}}
}

// HeadersOf builds a Headers from a plain map, downcasing keys. It mirrors
// constructing Rack::Headers and assigning each pair. Iteration order of a Go
// map is unspecified, so callers needing deterministic order should Set keys
// individually.
func HeadersOf(m map[string]any) *Headers {
	h := NewHeaders()
	for k, v := range m {
		h.Set(k, v)
	}
	return h
}

func downcaseKey(k string) string { return strings.ToLower(k) }

// Len reports the number of header keys.
func (h *Headers) Len() int { return len(h.keys) }

// Keys returns the (already down-cased) keys in insertion order.
func (h *Headers) Keys() []string {
	out := make([]string, len(h.keys))
	copy(out, h.keys)
	return out
}

// Has reports whether the (case-insensitive) key is present.
func (h *Headers) Has(key string) bool {
	_, ok := h.values[downcaseKey(key)]
	return ok
}

// Get returns the value for key, or nil if absent.
func (h *Headers) Get(key string) any {
	return h.values[downcaseKey(key)]
}

// GetOK returns the value for key and whether it was present.
func (h *Headers) GetOK(key string) (any, bool) {
	v, ok := h.values[downcaseKey(key)]
	return v, ok
}

// Set assigns the down-cased key to val.
func (h *Headers) Set(key string, val any) {
	dk := downcaseKey(key)
	if _, ok := h.values[dk]; !ok {
		h.keys = append(h.keys, dk)
	}
	h.values[dk] = val
}

// Delete removes the (case-insensitive) key, returning whether it was present.
func (h *Headers) Delete(key string) bool {
	dk := downcaseKey(key)
	if _, ok := h.values[dk]; !ok {
		return false
	}
	delete(h.values, dk)
	for i, k := range h.keys {
		if k == dk {
			h.keys = append(h.keys[:i], h.keys[i+1:]...)
			break
		}
	}
	return true
}

// Each iterates key/value pairs in insertion order. Returning false from fn
// stops iteration.
func (h *Headers) Each(fn func(key string, val any) bool) {
	for _, k := range h.keys {
		if !fn(k, h.values[k]) {
			return
		}
	}
}

// ToMap returns a plain map snapshot of the headers (losing order).
func (h *Headers) ToMap() map[string]any {
	out := make(map[string]any, len(h.keys))
	for k, v := range h.values {
		out[k] = v
	}
	return out
}
