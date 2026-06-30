// Copyright (c) the go-ruby-rack/rack authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rack

import (
	"reflect"
	"testing"
)

func TestParamsBasics(t *testing.T) {
	p := NewParams()
	if p.Len() != 0 {
		t.Error("new params not empty")
	}
	p.Set("a", "1")
	p.Set("b", "2")
	p.Set("a", "3") // overwrite, order preserved
	if !reflect.DeepEqual(p.Keys(), []string{"a", "b"}) {
		t.Errorf("keys = %v", p.Keys())
	}
	if v, ok := p.Get("a"); !ok || v != "3" {
		t.Errorf("get a = %v,%v", v, ok)
	}
	if !p.Has("b") {
		t.Error("Has b failed")
	}
	if p.Has("z") {
		t.Error("Has z should be false")
	}
}

func TestParamsDelete(t *testing.T) {
	p := NewParams()
	p.Set("a", "1")
	p.Set("b", "2")
	v, ok := p.Delete("a")
	if !ok || v != "1" {
		t.Errorf("delete a = %v,%v", v, ok)
	}
	if _, ok := p.Delete("a"); ok {
		t.Error("second delete should report absent")
	}
	if !reflect.DeepEqual(p.Keys(), []string{"b"}) {
		t.Errorf("keys = %v", p.Keys())
	}
}

func TestParamsEach(t *testing.T) {
	p := NewParams()
	p.Set("a", "1")
	p.Set("b", "2")
	var ks []string
	p.Each(func(k string, v any) bool { ks = append(ks, k); return true })
	if !reflect.DeepEqual(ks, []string{"a", "b"}) {
		t.Errorf("each order = %v", ks)
	}
	// Early stop.
	ks = nil
	p.Each(func(k string, v any) bool { ks = append(ks, k); return false })
	if len(ks) != 1 {
		t.Errorf("early stop len %d", len(ks))
	}
}

func TestParamsMerge(t *testing.T) {
	a := NewParams()
	a.Set("x", "1")
	a.Set("y", "2")
	b := NewParams()
	b.Set("y", "3")
	b.Set("z", "4")
	m := a.Merge(b)
	if got := rubyInspect(m); got != `{"x" => "1", "y" => "3", "z" => "4"}` {
		t.Errorf("merge = %s", got)
	}
	// Merge nil is a copy.
	m2 := a.Merge(nil)
	if got := rubyInspect(m2); got != `{"x" => "1", "y" => "2"}` {
		t.Errorf("merge nil = %s", got)
	}
}

func TestParamsToMap(t *testing.T) {
	p := NewParams()
	p.Set("a", "1")
	m := p.ToMap()
	if m["a"] != "1" || len(m) != 1 {
		t.Errorf("ToMap = %v", m)
	}
}

func TestParamsGetAbsent(t *testing.T) {
	p := NewParams()
	if _, ok := p.Get("nope"); ok {
		t.Error("absent get ok should be false")
	}
	if v, ok := p.Delete("nope"); ok || v != nil {
		t.Error("absent delete should be nil,false")
	}
}

func TestSortedKeysHelper(t *testing.T) {
	p := NewParams()
	p.Set("b", "1")
	p.Set("a", "2")
	if !reflect.DeepEqual(sortedKeys(p), []string{"a", "b"}) {
		t.Errorf("sortedKeys = %v", sortedKeys(p))
	}
	if itoa(7) != "7" {
		t.Error("itoa")
	}
}
