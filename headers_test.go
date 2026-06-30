// Copyright (c) the go-ruby-rack/rack authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rack

import (
	"reflect"
	"testing"
)

func TestHeadersDowncase(t *testing.T) {
	h := NewHeaders()
	h.Set("Content-Type", "text/html")
	if got := h.Get("content-type"); got != "text/html" {
		t.Errorf("case-insensitive get failed: %v", got)
	}
	if got := h.Get("CONTENT-TYPE"); got != "text/html" {
		t.Errorf("upper get failed: %v", got)
	}
	if !h.Has("content-type") {
		t.Error("Has failed")
	}
	if v, ok := h.GetOK("Content-Type"); !ok || v != "text/html" {
		t.Error("GetOK failed")
	}
}

func TestHeadersKeysOrderAndUpdate(t *testing.T) {
	h := NewHeaders()
	h.Set("A", "1")
	h.Set("B", "2")
	h.Set("a", "3") // overwrites, keeps order
	if !reflect.DeepEqual(h.Keys(), []string{"a", "b"}) {
		t.Errorf("keys = %v", h.Keys())
	}
	if h.Get("A") != "3" {
		t.Errorf("overwrite failed: %v", h.Get("A"))
	}
	if h.Len() != 2 {
		t.Errorf("len = %d", h.Len())
	}
}

func TestHeadersDelete(t *testing.T) {
	h := NewHeaders()
	h.Set("X", "1")
	h.Set("Y", "2")
	if !h.Delete("x") {
		t.Error("delete should report present")
	}
	if h.Delete("x") {
		t.Error("second delete should report absent")
	}
	if h.Has("X") {
		t.Error("X still present")
	}
	if !reflect.DeepEqual(h.Keys(), []string{"y"}) {
		t.Errorf("keys after delete = %v", h.Keys())
	}
}

func TestHeadersOf(t *testing.T) {
	h := HeadersOf(map[string]any{"Content-Type": "text/plain"})
	if h.Get("content-type") != "text/plain" {
		t.Error("HeadersOf failed")
	}
}

func TestHeadersEachAndToMap(t *testing.T) {
	h := NewHeaders()
	h.Set("A", "1")
	h.Set("B", "2")
	count := 0
	h.Each(func(k string, v any) bool { count++; return true })
	if count != 2 {
		t.Errorf("each count %d", count)
	}
	// Early stop.
	count = 0
	h.Each(func(k string, v any) bool { count++; return false })
	if count != 1 {
		t.Errorf("early-stop count %d", count)
	}
	m := h.ToMap()
	if m["a"] != "1" || m["b"] != "2" {
		t.Errorf("ToMap = %v", m)
	}
}

func TestHeadersGetAbsent(t *testing.T) {
	h := NewHeaders()
	if h.Get("nope") != nil {
		t.Error("absent should be nil")
	}
	if _, ok := h.GetOK("nope"); ok {
		t.Error("GetOK absent should be false")
	}
}
