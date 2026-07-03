// Copyright (c) the go-ruby-rack/rack authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rack

import "strings"

// CookieValue holds the cookie payload and attributes for SetCookieHeader,
// mirroring the Hash form Rack::Utils.set_cookie_header accepts. A zero
// CookieValue with only Value set produces a bare "key=value" cookie. The
// HTTPOnlySet/SecureSet/PartitionedSet booleans gate emission of those valueless
// attributes.
type CookieValue struct {
	Value       string
	Values      []string // multiple values, joined with '&' (Ruby Array form)
	HasValues   bool     // use Values instead of Value
	Domain      string
	Path        string
	MaxAge      string
	Expires     string // pre-formatted httpdate (host supplies it)
	Secure      bool
	HTTPOnly    bool
	Partitioned bool
	SameSite    string // "", "none", "lax" or "strict"
}

// ErrInvalidCookieKey is returned by SetCookieHeader for a key that violates
// RFC6265, matching the ArgumentError Ruby raises.
type ErrInvalidCookieKey struct{ Key string }

func (e *ErrInvalidCookieKey) Error() string {
	return "invalid cookie key: " + quoteInspect(e.Key)
}

// ErrInvalidSameSite is returned for an unrecognised SameSite value.
type ErrInvalidSameSite struct{ Value string }

func (e *ErrInvalidSameSite) Error() string {
	return "Invalid :same_site value: " + quoteInspect(e.Value)
}

func quoteInspect(s string) string { return `"` + s + `"` }

// validCookieKey reports whether key matches Rack's VALID_COOKIE_KEY: one or
// more US-ASCII characters excluding controls, spaces, tabs and separators.
func validCookieKey(key string) bool {
	if key == "" {
		return false
	}
	for i := 0; i < len(key); i++ {
		c := key[i]
		switch {
		case c >= '0' && c <= '9',
			c >= 'a' && c <= 'z',
			c >= 'A' && c <= 'Z':
			continue
		}
		switch c {
		case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '.', '^', '_', '`', '|', '~':
			continue
		}
		return false
	}
	return true
}

// MakeCookieHeader builds the encoded set-cookie string for the given key and
// cookie, matching Rack::Utils.set_cookie_header. The cookie value(s) are
// escaped with Escape and joined with '&'. An invalid key yields an error.
func MakeCookieHeader(key string, c CookieValue) (string, error) {
	if !validCookieKey(key) {
		return "", &ErrInvalidCookieKey{Key: key}
	}
	var values []string
	if c.HasValues {
		values = c.Values
	} else {
		values = []string{c.Value}
	}
	escaped := make([]string, len(values))
	for i, v := range values {
		escaped[i] = Escape(v)
	}

	var b strings.Builder
	b.WriteString(key)
	b.WriteByte('=')
	b.WriteString(strings.Join(escaped, "&"))

	if c.Domain != "" {
		b.WriteString("; domain=" + c.Domain)
	}
	if c.Path != "" {
		b.WriteString("; path=" + c.Path)
	}
	if c.MaxAge != "" {
		b.WriteString("; max-age=" + c.MaxAge)
	}
	if c.Expires != "" {
		b.WriteString("; expires=" + c.Expires)
	}
	if c.Secure {
		b.WriteString("; secure")
	}
	if c.HTTPOnly {
		b.WriteString("; httponly")
	}
	switch strings.ToLower(c.SameSite) {
	case "":
		// omitted
	case "none":
		b.WriteString("; samesite=none")
	case "lax":
		b.WriteString("; samesite=lax")
	case "strict":
		b.WriteString("; samesite=strict")
	default:
		return "", &ErrInvalidSameSite{Value: c.SameSite}
	}
	if c.Partitioned {
		b.WriteString("; partitioned")
	}
	return b.String(), nil
}

// DeleteCookieHeaderValue is the standard expired-cookie payload Rack uses to
// instruct a client to drop a cookie: max-age 0 and an epoch expiry.
const DeleteCookieHeaderValue = "; max-age=0; expires=Thu, 01 Jan 1970 00:00:00 GMT"

// MakeDeleteCookieHeader builds an encoded set-cookie string that deletes the
// named cookie, matching Rack::Utils.delete_set_cookie_header: an empty value,
// max-age 0 and an epoch expiry, plus any supplied attributes.
func MakeDeleteCookieHeader(key string, c CookieValue) (string, error) {
	c.Value = ""
	c.HasValues = false
	c.MaxAge = "0"
	c.Expires = "Thu, 01 Jan 1970 00:00:00 GMT"
	return MakeCookieHeader(key, c)
}

// ParseCookies extracts and parses the Cookie header (env["HTTP_COOKIE"]) from a
// Rack environment, matching Rack::Utils.parse_cookies. A missing or non-string
// HTTP_COOKIE parses as an empty header.
func ParseCookies(env Env) *Params {
	str, _ := env[HTTPCookie].(string)
	return ParseCookiesHeader(str)
}

// ParseCookiesHeader parses a Cookie header value into an ordered map of cookie
// key to value, matching Rack::Utils.parse_cookies_header (RFC6265, splitting on
// ';'). The first occurrence of a key wins. A cookie with no '=' maps to a nil
// value; a value is unescaped, falling back to the raw bytes if it cannot be.
func ParseCookiesHeader(value string) *Params {
	out := NewParams()
	if value == "" {
		return out
	}
	for _, cookie := range splitCookies(value) {
		if cookie == "" {
			continue
		}
		eq := strings.IndexByte(cookie, '=')
		var k string
		var v any
		if eq < 0 {
			k = cookie
			v = nil
		} else {
			k = cookie[:eq]
			raw := cookie[eq+1:]
			if dec, err := Unescape(raw); err == nil {
				v = dec
			} else {
				v = raw
			}
		}
		if !out.Has(k) {
			out.Set(k, v)
		}
	}
	return out
}

// splitCookies splits a Cookie header on ';' optionally followed by spaces,
// matching Ruby's value.split(/; */n).
func splitCookies(value string) []string {
	var out []string
	var cur strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] == ';' {
			out = append(out, cur.String())
			cur.Reset()
			for i+1 < len(value) && value[i+1] == ' ' {
				i++
			}
			continue
		}
		cur.WriteByte(value[i])
	}
	out = append(out, cur.String())
	return out
}

// SetCookieHeaderInto appends a set-cookie value to a Headers, converting an
// existing single value to a []any list, matching Rack::Utils.set_cookie_header!.
func SetCookieHeaderInto(h *Headers, key string, c CookieValue) error {
	enc, err := MakeCookieHeader(key, c)
	if err != nil {
		return err
	}
	appendSetCookie(h, enc)
	return nil
}

// DeleteCookieHeaderInto sets an expired cookie in the Headers, matching
// Rack::Utils.delete_cookie_header!.
func DeleteCookieHeaderInto(h *Headers, key string, c CookieValue) error {
	enc, err := MakeDeleteCookieHeader(key, c)
	if err != nil {
		return err
	}
	if existing, ok := h.GetOK(SetCookie); ok {
		h.Set(SetCookie, append(toAnyList(existing), enc))
	} else {
		h.Set(SetCookie, enc)
	}
	return nil
}

func appendSetCookie(h *Headers, enc string) {
	if existing, ok := h.GetOK(SetCookie); ok {
		if arr, isArr := existing.([]any); isArr {
			h.Set(SetCookie, append(arr, enc))
		} else {
			h.Set(SetCookie, []any{existing, enc})
		}
	} else {
		h.Set(SetCookie, enc)
	}
}

// toAnyList coerces a header value (string or []any) into a []any, matching
// Ruby's Array(header) in delete_set_cookie_header!.
func toAnyList(v any) []any {
	switch x := v.(type) {
	case []any:
		return x
	case nil:
		return nil
	default:
		return []any{x}
	}
}
