// Copyright (c) the go-ruby-rack/rack authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rack

import (
	"strings"
)

// hexUpper formats one byte as two upper-case hex digits, like Ruby's URI escaper.
const hexUpper = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

func appendPercent(b *strings.Builder, c byte) {
	b.WriteByte('%')
	b.WriteByte(hexUpper[c>>4])
	b.WriteByte(hexUpper[c&0x0f])
}

// wwwFormUnreserved reports whether c is left untouched by Ruby's
// URI.encode_www_form_component (Rack::Utils.escape). The unreserved set is the
// ASCII alphanumerics plus '*', '-', '.' and '_'. Space is handled separately
// (encoded as '+'); every other byte becomes %XX.
func wwwFormUnreserved(c byte) bool {
	switch {
	case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		return true
	case c == '*' || c == '-' || c == '.' || c == '_':
		return true
	}
	return false
}

// Escape escapes a string the way Rack::Utils.escape does — i.e.
// URI.encode_www_form_component: CGI form encoding where space becomes '+', the
// unreserved set (alphanumerics and *-._) is preserved, and every other byte is
// percent-encoded with upper-case hex.
func Escape(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == ' ':
			b.WriteByte('+')
		case wwwFormUnreserved(c):
			b.WriteByte(c)
		default:
			appendPercent(&b, c)
		}
	}
	return b.String()
}

// escapePathSafe reports whether c is left untouched by Ruby's RFC2396 path
// escaper (Rack::Utils.escape_path). The safe set is the bytes Ruby's
// URI::RFC2396_PARSER.escape leaves alone for a path component.
func escapePathSafe(c byte) bool {
	switch {
	case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		return true
	}
	switch c {
	case '!', '$', '&', '\'', '(', ')', '*', '+', ',', '-', '.', '/',
		':', ';', '=', '?', '@', '[', ']', '_', '~':
		return true
	}
	return false
}

// EscapePath escapes a string like Rack::Utils.escape_path — RFC2396 URI path
// escaping, where (unlike Escape) space becomes "%20" rather than '+' and the
// path-safe punctuation set is preserved.
func EscapePath(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if escapePathSafe(c) {
			b.WriteByte(c)
		} else {
			appendPercent(&b, c)
		}
	}
	return b.String()
}

// hexVal decodes one hex digit, returning the value and whether it was valid.
func hexVal(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

// Unescape reverses Rack::Utils.escape (URI.decode_www_form_component): '+'
// becomes a space and %XX is decoded. An invalid percent-escape (a '%' not
// followed by two hex digits) makes the whole operation fail the way MRI raises
// ArgumentError, which this surfaces as a non-nil error.
func Unescape(s string) (string, error) {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '+':
			b.WriteByte(' ')
		case '%':
			if i+2 >= len(s) {
				return "", &InvalidParameterError{msg: "invalid %-encoding (" + s + ")"}
			}
			hi, ok1 := hexVal(s[i+1])
			lo, ok2 := hexVal(s[i+2])
			if !ok1 || !ok2 {
				return "", &InvalidParameterError{msg: "invalid %-encoding (" + s + ")"}
			}
			b.WriteByte(hi<<4 | lo)
			i += 2
		default:
			b.WriteByte(c)
		}
	}
	return b.String(), nil
}

// UnescapePath reverses EscapePath, decoding %XX escapes. Unlike Unescape it
// does not turn '+' into a space (mirroring URI::RFC2396_PARSER.unescape, which
// only undoes percent-encoding). An invalid escape is left verbatim, matching
// Ruby's lenient path unescaper.
func UnescapePath(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '%' && i+2 < len(s) {
			hi, ok1 := hexVal(s[i+1])
			lo, ok2 := hexVal(s[i+2])
			if ok1 && ok2 {
				b.WriteByte(hi<<4 | lo)
				i += 2
				continue
			}
		}
		b.WriteByte(c)
	}
	return b.String()
}

// htmlEscapes maps the five characters CGI.escapeHTML (and thus
// Rack::Utils.escape_html) replaces with entities. Note the apostrophe uses the
// numeric reference &#39;, exactly as MRI emits.
var htmlEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	`"`, "&quot;",
	"'", "&#39;",
)

// EscapeHTML escapes ampersands, angle brackets and quotes to their HTML/XML
// entities, matching Rack::Utils.escape_html (CGI.escapeHTML).
func EscapeHTML(s string) string {
	return htmlEscaper.Replace(s)
}

// UnescapeHTML reverses EscapeHTML, decoding the named and numeric character
// references CGI.unescapeHTML understands for the bytes EscapeHTML emits, plus
// the common decimal/hex numeric forms.
func UnescapeHTML(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if s[i] != '&' {
			b.WriteByte(s[i])
			i++
			continue
		}
		// Find the terminating ';' within a small window.
		semi := strings.IndexByte(s[i:], ';')
		if semi < 0 || semi > 10 {
			b.WriteByte('&')
			i++
			continue
		}
		entity := s[i+1 : i+semi]
		if rep, ok := unescapeEntity(entity); ok {
			b.WriteString(rep)
			i += semi + 1
		} else {
			b.WriteByte('&')
			i++
		}
	}
	return b.String()
}

// unescapeEntity decodes a single entity body (the text between '&' and ';').
func unescapeEntity(e string) (string, bool) {
	switch e {
	case "amp":
		return "&", true
	case "lt":
		return "<", true
	case "gt":
		return ">", true
	case "quot":
		return `"`, true
	case "apos":
		return "'", true
	}
	if len(e) >= 2 && e[0] == '#' {
		var code int
		if e[1] == 'x' || e[1] == 'X' {
			for _, c := range []byte(e[2:]) {
				v, ok := hexVal(c)
				if !ok {
					return "", false
				}
				code = code*16 + int(v)
			}
			if len(e) == 2 {
				return "", false
			}
		} else {
			for _, c := range []byte(e[1:]) {
				if c < '0' || c > '9' {
					return "", false
				}
				code = code*10 + int(c-'0')
			}
		}
		return string(rune(code)), true
	}
	return "", false
}
