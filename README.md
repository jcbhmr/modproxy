# xmod 

🆕 Extensions to the [golang.org/x/mod](https://pkg.go.dev/golang.org/x/mod) module

- **gomodzip:** Create & read Go module zip files CLI
- **gomodproxy:** Basic Go module proxy server CLI
- **proxy:** Go module proxy client & server interfaces
- **zip:** Additional io/fs.FS interface support for the existing x/mod/zip package

## Installation

```sh
go install github.com/jcbhmr/xmod/gomodzip@latest
go install github.com/jcbhmr/xmod/gomodproxy@latest
```

```sh
go get github.com/jcbhmr/xmod
```

## Development

```sh
go test -v ./...
```
