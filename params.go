// Copyright (c) the go-ruby-rack/rack authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rack

// Params is an insertion-ordered string-keyed map, the Go analogue of Ruby's
// Hash (which preserves insertion order). parse_query / parse_nested_query and
// the Request param accessors return values built from Params, []any, string
// and nil — the same small value model the rack gem produces. A nested value is
// one of:
//
//   - nil          (a bare key with no '=')
//   - string       (a scalar value)
//   - []any        (an array, from foo[] nesting or repeated parse_query keys)
//   - *Params      (a sub-hash, from foo[bar] nesting)
type Params struct {
	keys   []string
	values map[string]any
}

// NewParams returns an empty ordered map.
func NewParams() *Params {
	return &Params{values: map[string]any{}}
}

// Len reports the number of keys.
func (p *Params) Len() int { return len(p.keys) }

// Keys returns the keys in insertion order. The slice is a copy and safe to
// mutate.
func (p *Params) Keys() []string {
	out := make([]string, len(p.keys))
	copy(out, p.keys)
	return out
}

// Has reports whether key is present.
func (p *Params) Has(key string) bool {
	_, ok := p.values[key]
	return ok
}

// Get returns the value for key and whether it was present.
func (p *Params) Get(key string) (any, bool) {
	v, ok := p.values[key]
	return v, ok
}

// Set assigns key to val, appending it to the key order if new.
func (p *Params) Set(key string, val any) {
	if _, ok := p.values[key]; !ok {
		p.keys = append(p.keys, key)
	}
	p.values[key] = val
}

// Delete removes key, returning its value (or nil) and whether it was present.
func (p *Params) Delete(key string) (any, bool) {
	v, ok := p.values[key]
	if !ok {
		return nil, false
	}
	delete(p.values, key)
	for i, k := range p.keys {
		if k == key {
			p.keys = append(p.keys[:i], p.keys[i+1:]...)
			break
		}
	}
	return v, true
}

// Each iterates key/value pairs in insertion order. If fn returns false the
// iteration stops.
func (p *Params) Each(fn func(key string, val any) bool) {
	for _, k := range p.keys {
		if !fn(k, p.values[k]) {
			return
		}
	}
}

// Merge returns a new *Params containing this map's pairs overlaid with other's
// (other wins on a key collision), mirroring Hash#merge used by Request#params.
func (p *Params) Merge(other *Params) *Params {
	out := NewParams()
	p.Each(func(k string, v any) bool { out.Set(k, v); return true })
	if other != nil {
		other.Each(func(k string, v any) bool { out.Set(k, v); return true })
	}
	return out
}

// ToMap returns a plain Go map snapshot (losing key order), convenient for
// callers that do not care about ordering.
func (p *Params) ToMap() map[string]any {
	out := make(map[string]any, len(p.keys))
	for k, v := range p.values {
		out[k] = v
	}
	return out
}
