// Copyright (c) the go-ruby-rack/rack authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rack

import "testing"

func TestHTTPStatusCodes(t *testing.T) {
	if HTTPStatusCodes[200] != "OK" || HTTPStatusCodes[404] != "Not Found" {
		t.Error("status code table wrong")
	}
	if HTTPStatusCodes[418] != "" {
		t.Error("418 should be unassigned (absent) in this table")
	}
}

func TestSymbolToStatusCode(t *testing.T) {
	cases := map[string]int{
		"ok":                    200,
		"not_found":             404,
		"internal_server_error": 500,
		"unprocessable_content": 422,
		// obsolete aliases
		"payload_too_large":    413,
		"unprocessable_entity": 422,
		"not_extended":         510,
	}
	for sym, want := range cases {
		got, ok := SymbolToStatusCode(sym)
		if !ok || got != want {
			t.Errorf("SymbolToStatusCode(%q) = %d,%v want %d", sym, got, ok, want)
		}
	}
	if _, ok := SymbolToStatusCode("nonsense"); ok {
		t.Error("expected nonsense to be unrecognised")
	}
}

func TestStatusSymbol(t *testing.T) {
	if statusSymbol("Not Found") != "not_found" {
		t.Error("Not Found")
	}
	if statusSymbol("Non-Authoritative Information") != "non_authoritative_information" {
		t.Error("hyphen and space")
	}
}

func TestStatusWithNoEntityBody(t *testing.T) {
	for _, s := range []int{100, 150, 199, 204, 304} {
		if !StatusWithNoEntityBody(s) {
			t.Errorf("StatusWithNoEntityBody(%d) = false, want true", s)
		}
	}
	for _, s := range []int{200, 201, 404, 500, 99, 305} {
		if StatusWithNoEntityBody(s) {
			t.Errorf("StatusWithNoEntityBody(%d) = true, want false", s)
		}
	}
}

func TestStatusCodesSorted(t *testing.T) {
	codes := statusCodesSorted()
	if len(codes) != len(HTTPStatusCodes) {
		t.Fatalf("len %d != %d", len(codes), len(HTTPStatusCodes))
	}
	for i := 1; i < len(codes); i++ {
		if codes[i-1] >= codes[i] {
			t.Fatalf("not sorted at %d: %d >= %d", i, codes[i-1], codes[i])
		}
	}
	if codes[0] != 100 {
		t.Errorf("first code %d want 100", codes[0])
	}
}
