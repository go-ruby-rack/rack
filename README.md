<p align="center"><img src="https://raw.githubusercontent.com/go-ruby-rack/brand/main/social/go-ruby-rack-rack.png" alt="go-ruby-rack/rack" width="720"></p>

# rack — go-ruby-rack

[![Docs](https://img.shields.io/badge/docs-mkdocs--material-DC2626)](https://go-ruby-rack.github.io/docs/)
[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26.4%2B-00ADD8)](https://go.dev/dl/)
[![Coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)](#tests--coverage)

**A pure-Go (no cgo) reimplementation of the deterministic core of Ruby's
[Rack](https://github.com/rack/rack)** — the SPEC value types and the
pure-compute utilities of `Rack::Utils`, `Rack::Request`, `Rack::Response`,
`Rack::MediaType` and the header machinery — matching the MRI `rack` gem
(Rack 3.x) **byte-for-byte**. It shapes a Rack environment, parses and builds
query strings and cookies, escapes and unescapes URI/HTML, maps HTTP status
codes, and produces the `[status, headers, body]` SPEC tuple — **without any
Ruby runtime**.

It is the Rack backend for
[go-embedded-ruby](https://github.com/go-embedded-ruby/ruby), but is a
**standalone, reusable** module — a sibling of
[go-ruby-regexp](https://github.com/go-ruby-regexp/regexp) (the Onigmo engine),
[go-ruby-erb](https://github.com/go-ruby-erb/erb) (the ERB compiler) and
[go-ruby-yaml](https://github.com/go-ruby-yaml/yaml) (the Psych port).

> **What it is — and isn't.** Shaping the env hash, parsing parameters and
> cookies, escaping, status-code mapping and assembling the response tuple are
> all fully deterministic and need **no interpreter**, so they live here as pure
> Go. The HTTP server — `Rack::Handler`, the socket accept loop, TLS — is the
> **host's** job and is out of scope. Reading the request body is a single,
> explicit seam: the host supplies an [`Input`](#the-bodyinput-seam) backed by
> whatever IO it has, so this library never touches the network.

## Features

A faithful port of Rack 3.x's pure-compute surface, validated against the `rack`
gem on every supported platform:

- **`Utils` query parsing** — `ParseQuery` / `ParseNestedQuery` expand
  `foo[bar]` / `foo[]` / `x[][a]` bracket nesting exactly like the gem (including
  the array-of-hash vs nested-array subtleties and the `ParameterTypeError` /
  `ParamsTooDeepError` conflicts), and `BuildQuery` / `BuildNestedQuery` invert
  them.
- **Escaping** — `Escape` / `Unescape` (`encode/decode_www_form_component`,
  space ↔ `+`), `EscapePath` / `UnescapePath` (RFC2396), and `EscapeHTML` /
  `UnescapeHTML`, byte-identical to MRI down to the unreserved set and upper-case
  hex.
- **Status codes** — the full `HTTPStatusCodes` table, the symbol → code reverse
  map (with the deprecated aliases), and `StatusWithNoEntityBody`.
- **Headers** — `Headers`, the insertion-ordered, key-down-casing map mirroring
  `Rack::Headers`.
- **Content negotiation** — `QValues`, `BestQMatch`, `SelectBestEncoding`
  (Accept-Encoding), and `GetByteRanges` / byte-range parsing.
- **Path & security helpers** — `CleanPathInfo` (traversal-safe path
  normalisation), `ValidPath`, `SecureCompare` (constant-time), and
  `ForwardedValues` (RFC 7239 `Forwarded` parsing).
- **Cookies** — `ParseCookiesHeader` / `ParseCookies` (from an `Env`),
  `MakeCookieHeader` / `MakeDeleteCookieHeader`, and the `…Into(*Headers, …)`
  mutators.
- **`Request`** over an `Env` — method predicates, `PathInfo` / `QueryString` /
  `GET` / `POST` / `Params` / `Cookies`, `ContentType` / `MediaType`, `Host` /
  `Port` / `Scheme` / `SSL` / `BaseURL` / `URL` / `Fullpath`, `XHR`, `IP` (with
  the trusted-proxy filter) and the `…Header` accessors.
- **`Response`** — `NewResponse(body, status, headers)`, `Write`, `Finish` /
  `ToA` (the `[status, headers, body]` tuple), `SetStatus`, header get/set,
  `SetCookie` / `DeleteCookie`, `Redirect`, and the status-class predicates.
- **`MediaType`** — `MediaTypeOf` and `MediaTypeParams`.

CGO-free, dependency-free, **100% test coverage**, `gofmt` + `go vet` clean, and
green across the six 64-bit Go targets (amd64, arm64, riscv64, loong64, ppc64le,
s390x) and three OSes (Linux, macOS, Windows).

## Install

```sh
go get github.com/go-ruby-rack/rack
```

## Usage

```go
package main

import (
	"fmt"

	"github.com/go-ruby-rack/rack"
)

func main() {
	// Parse a nested query string into structural types.
	p, _ := rack.ParseNestedQuery("user[name]=ada&user[langs][]=go", "&",
		rack.DefaultParamDepthLimit)
	fmt.Println(p.Get("user")) // *rack.Params {"name"=>"ada", "langs"=>["go"]}

	// Shape a request over a Rack env.
	req := rack.NewRequest(rack.Env{
		rack.RequestMethod: "GET",
		rack.PathInfo:      "/search",
		rack.QueryString:   "q=hello+world",
		rack.HTTPHost:      "example.com",
		rack.RackURLScheme: "https",
	})
	fmt.Println(req.URL()) // https://example.com/search?q=hello+world
	get, _ := req.GET()
	fmt.Println(get.Get("q")) // "hello world"

	// Build a response and emit the SPEC tuple.
	res := rack.NewResponseString("Hello", 200, nil)
	res.SetContentType("text/plain")
	status, headers, body := res.Finish()
	fmt.Println(status, headers.Get("content-length"), body)
	// 200 5 [Hello]
}
```

## The body/input seam

Reading the request body is the one place this library defers to the host. A
`Request` reads `env["rack.input"]` through the small `Input` interface:

```go
type Input interface {
	// Read returns up to n bytes, or all remaining bytes when n < 0, and
	// nil at EOF — the subset of Ruby's IO contract Rack relies on.
	Read(n int) ([]byte, error)
}
```

`Request.POST` / `Request.Params` read it only for form-data content types,
parse the body with `ParseNestedQuery`, and memoise into the env exactly like
the gem. The host (e.g. `rbgo`) supplies the `Input` over whatever socket or
buffer it has, so the package stays free of any network or runtime dependency.

## Value model

Parsed parameters and cookies are built from a small, fixed set of Go types — the
analogue of the Ruby `Hash`/`Array`/`String`/`nil` graph the gem returns:

| Ruby             | Go                       |
| ---------------- | ------------------------ |
| `Hash` (ordered) | `*rack.Params`           |
| `Array`          | `[]any`                  |
| `String`         | `string`                 |
| `nil`            | `nil`                    |
| response headers | `*rack.Headers` (ordered, down-cased keys) |

`*Params` preserves insertion order (like Ruby's `Hash`), so key order
round-trips through `BuildQuery` / `BuildNestedQuery`.

## Tests & coverage

The suite pairs deterministic, ruby-free tests (which alone hold coverage at
100%, so the qemu cross-arch and Windows lanes pass the gate) with a
**differential MRI oracle**: every escaping, query-parse, status, cookie,
media-type, byte-range and `Response#finish` case is also run through the system
`ruby` with the `rack` gem and compared byte-for-byte. The oracle scripts
`$stdout.binmode` so Windows text-mode never pollutes the bytes, and skip
themselves where `ruby` or the gem is absent.

```sh
COVERPKG=$(go list ./... | paste -sd, -)
go test -race -coverpkg="$COVERPKG" -coverprofile=cover.out ./...
go tool cover -func=cover.out | tail -1   # 100.0%
```

## License

BSD-3-Clause — see [LICENSE](LICENSE). Copyright the go-ruby-rack/rack authors.
