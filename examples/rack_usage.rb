# frozen_string_literal: true
#
# Pure-Ruby usage of the Rack module, as provided by go-embedded-ruby (rbgo).
# Run it with:  rbgo examples/rack_usage.rb

require "rack"

# Rack::Request: a read-mostly view over a Rack env hash.
env = {
  "REQUEST_METHOD" => "GET",
  "PATH_INFO"      => "/search",
  "QUERY_STRING"   => "q=rack&lang=ruby",
  "SERVER_NAME"    => "example.org",
  "HTTPS"          => "on",
}
req = Rack::Request.new(env)
puts req.request_method                # => GET
puts req.get?                          # => true
puts req.scheme                        # => https
puts req.params.inspect                # => {"q" => "rack", "lang" => "ruby"}

# Rack::Response: buffer a status/headers/body, then emit the SPEC triple.
resp = Rack::Response.new("Hello", 200, { "content-type" => "text/plain" })
resp.write(", Rack!")
status, headers, body = resp.finish
puts status                            # => 200
puts headers["content-type"]           # => text/plain
puts body.inspect                      # => ["Hello", ", Rack!"]

# Rack::Utils: deterministic query/encoding helpers.
puts Rack::Utils.escape("a b&c")       # => a+b%26c
puts Rack::Utils.escape_html("<a>")    # => &lt;a&gt;
puts Rack::Utils.build_query({ "x" => "1", "y" => "2" }) # => x=1&y=2
