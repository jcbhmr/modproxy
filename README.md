# Module proxy for Go

⏩ Basic Go module proxy interface & multiplexer

## Installation

```sh
go get github.com/jcbhmr/modproxy
```

## Usage

This module is intended to augment the existing [golang.org/x/mod](https://pkg.go.dev/golang.org/x/mod) module. Imagine it as though it were golang.org/x/mod/modproxy (even though it's actually github.com/jcbhmr/modproxy). It's supposed to be like a counterpart to the [golang.org/x/mod/sumdb](https://pkg.go.dev/golang.org/x/mod/sumdb) module, but for module proxies.

## Development

```sh
go test -v ./...
```
