// Copyright (c) the go-ruby-rack/rack authors
//
// SPDX-License-Identifier: BSD-3-Clause

// Package rack is a pure-Go (no cgo) reimplementation of the deterministic,
// interpreter-independent core of Ruby's Rack — the SPEC value types and the
// pure-compute utilities — matching MRI's `rack` gem (Rack 3.x) byte-for-byte.
//
// It models a Rack environment as a [Env] (a string-keyed map), and exposes
// [Request] and [Response] over it, plus the [Utils], [MediaType] and header
// helpers. The HTTP server itself (the socket accept loop, Rack::Handler) is
// NOT part of this package: it is the host's job. The body-reading seam — the
// `rack.input` IO — is supplied by the host through the [Input] interface, so
// this package stays free of any Ruby runtime or network dependency.
//
// The package is the Rack backend for go-embedded-ruby, but is a standalone,
// reusable module — a sibling of go-ruby-regexp, go-ruby-erb and go-ruby-yaml.
package rack
