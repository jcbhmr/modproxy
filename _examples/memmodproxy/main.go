package main

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"io/fs"
	"net/http"

	"github.com/jcbhmr/modproxy"
	"github.com/psanford/memfs"
	"golang.org/x/mod/module"
)

type MemModProxy struct {
	Latests map[string]string
	Mods    map[module.Version][]byte
	Zips    map[module.Version][]byte
}

func (m *MemModProxy) List(ctx context.Context, path string) ([]string, error) {
	versions := []string{}
	for v := range m.Mods {
		if v.Path == path {
			versions = append(versions, v.Version)
		}
	}
	if len(versions) == 0 {
		return nil, fs.ErrNotExist
	}
	return versions, nil
}

func (m *MemModProxy) Info(ctx context.Context, version module.Version) (modproxy.Info, error) {
	if _, ok := m.Mods[version]; ok {
		return modproxy.Info{Version: version.Version}, nil
	}
	return modproxy.Info{}, fs.ErrNotExist
}

func (m *MemModProxy) Mod(ctx context.Context, version module.Version) (io.ReadCloser, error) {
	if mod, ok := m.Mods[version]; ok {
		return io.NopCloser(bytes.NewReader(mod)), nil
	}
	return nil, fs.ErrNotExist
}

func (m *MemModProxy) Zip(ctx context.Context, version module.Version) (io.ReadCloser, error) {
	if zipData, ok := m.Zips[version]; ok {
		return io.NopCloser(bytes.NewReader(zipData)), nil
	}
	return nil, fs.ErrNotExist
}

func (m *MemModProxy) Latest(ctx context.Context, path string) (modproxy.Info, error) {
	if v, ok := m.Latests[path]; ok {
		return modproxy.Info{Version: v}, nil
	}
	return modproxy.Info{}, fs.ErrNotExist
}

func main() {
	proxier := &MemModProxy{
		Latests: map[string]string{
			"octocat.github.io/awesome": "v1.0.0",
		},
		Mods: map[module.Version][]byte{
			{"octocat.github.io/awesome", "v1.0.0"}: []byte("module octocat.github.io/awesome\n"),
		},
		Zips: map[module.Version][]byte{
			{"octocat.github.io/awesome", "v1.0.0"}: func() []byte {
				var err error
				f := memfs.New()
				err = f.MkdirAll("octocat.github.io/awesome@v1.0.0", 0755)
				if err != nil {
					panic(err)
				}
				err = f.WriteFile("octocat.github.io/awesome@v1.0.0/go.mod", []byte("module octocat.github.io/awesome\n"), 0644)
				if err != nil {
					panic(err)
				}
				err = f.WriteFile("octocat.github.io/awesome@v1.0.0/main.go", []byte("package main\nfunc main() {}"), 0644)
				if err != nil {
					panic(err)
				}
				buffer := &bytes.Buffer{}
				zipWriter := zip.NewWriter(buffer)
				err = zipWriter.AddFS(f)
				if err != nil {
					panic(err)
				}
				return buffer.Bytes()
			}(),
		},
	}
	http.ListenAndServe(":8080", &modproxy.Proxy{Proxier: proxier})
}
