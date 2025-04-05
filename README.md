# xmod 

🆕 Extensions to the [golang.org/x/mod](https://pkg.go.dev/golang.org/x/mod) module

<table align=center><td>

```go
server := modproxy.NewServer(&myServerOps{})
http.ListenAndServe(":8080", server)
```

<tr><td>

```go
client := modproxy.NewClient(&myClientOps{})
repo, err := client.Lookup("golang.org/x/mod")
latest, err := repo.Latest()
goMod, err := repo.GoMod(latest.Version)
buffer := &bytes.Buffer{}
err = repo.Zip(buffer, latest.Version)
```

<tr><td>

```go

```

</table>

## Installation

```sh
go get github.com/jcbhmr/xmod
```

## Usage

This module is intended to augment the existing [golang.org/x/mod](https://pkg.go.dev/golang.org/x/mod) module. There are two main components:

1\. **github.com/jcbhmr/xmod/modproxy:** A client & server implementation for a Go module proxy. The client is similar to [cmd/go/internal/modfetch](https://pkg.go.dev/cmd/go/internal/modfetch) mixed with [golang.org/x/mod/sumdb.Client](https://pkg.go.dev/golang.org/x/mod/sumdb#Client).

```go
type httpClientOps struct {
    BaseURL string
}
func (h *httpClientOps) ReadRemote(path string) ([]byte, error) {
    resp, err := http.Get(path.Join(h.BaseURL, path))
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("%s %d %s", resp.Request.URL, resp.StatusCode, resp.Status)
    }
    return io.ReadAll(resp.Body)
}

client := modproxy.NewClient(&httpClientOps{
    BaseURL: "https://proxy.golang.org",
})
client.SetGONOPROXY("example.com")

var repo *modproxy.Repo
repo, err = client.Lookup("golang.org/x/mod")

var latest *modproxy.RevInfo
latest, err = repo.Latest()

var goMod []byte
goMod, err = repo.GoMod(latest.Version)

buffer := &bytes.Buffer{}
err = repo.Zip(buffer, latest.Version)
```

2\. **github.com/jcbhmr/xmod/zip:** Adds more FS-agnostic zip file handling to the existing [golang.org/x/mod/zip](https://pkg.go.dev/golang.org/x/mod/zip) package.

```go

```

## Development

```sh
go test -v ./...
```
