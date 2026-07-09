# Ruby examples

Pure-Ruby examples for the `rack` library as provided by
[go-embedded-ruby](https://github.com/go-embedded-ruby/ruby) (rbgo). Run them
with the `rbgo` interpreter:

```sh
rbgo examples/rack_usage.rb
```

| File | Shows |
| --- | --- |
| [`rack_usage.rb`](rack_usage.rb) | `Rack::Request` accessors, `Rack::Response` finish triple, and `Rack::Utils` encoding/query helpers. |

Each example is executed as-is under rbgo (`require "rack"`).
