package proxy_test

import (
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jcbhmr/xmod/proxy"
	"golang.org/x/mod/module"
)

type StaticServerOps struct {
	RevInfos      map[string][]*proxy.RevInfo
	LatestVersion map[string]string
	GoModData     map[module.Version][]byte
	ZipData       map[module.Version][]byte
}

func (r *StaticServerOps) Versions(ctx context.Context, path string) ([]string, error) {
	if r.RevInfos == nil {
		return nil, fs.ErrNotExist
	}
	ri, ok := r.RevInfos[path]
	if !ok {
		return nil, fs.ErrNotExist
	}
	versions := make([]string, len(ri))
	for i, v := range ri {
		versions[i] = v.Version
	}
	return versions, nil
}

func (r *StaticServerOps) Stat(ctx context.Context, m module.Version) (*proxy.RevInfo, error) {
	if r.RevInfos == nil {
		return nil, fs.ErrNotExist
	}
	ri, ok := r.RevInfos[m.Path]
	if !ok {
		return nil, fs.ErrNotExist
	}
	for _, v := range ri {
		if v.Version == m.Version {
			return v, nil
		}
	}
	return nil, fs.ErrNotExist
}

func (r *StaticServerOps) GoMod(ctx context.Context, m module.Version) ([]byte, error) {
	if r.GoModData == nil {
		return nil, fs.ErrNotExist
	}
	data, ok := r.GoModData[m]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return data, nil
}

func (r *StaticServerOps) Zip(ctx context.Context, dst io.Writer, m module.Version) error {
	if r.ZipData == nil {
		return fs.ErrNotExist
	}
	data, ok := r.ZipData[m]
	if !ok {
		return fs.ErrNotExist
	}
	n, err := dst.Write(data)
	if err != nil {
		return err
	}
	if n != len(data) {
		return io.ErrShortWrite
	}
	return nil
}

func (r *StaticServerOps) Latest(ctx context.Context, path string) (*proxy.RevInfo, error) {
	if r.LatestVersion == nil {
		return nil, fs.ErrNotExist
	}
	version, ok := r.LatestVersion[path]
	if !ok {
		return nil, fs.ErrNotExist
	}
	ri, ok := r.RevInfos[path]
	if !ok {
		return nil, fs.ErrNotExist
	}
	for _, v := range ri {
		if v.Version == version {
			return v, nil
		}
	}
	return nil, fs.ErrNotExist
}

func ExampleServer() {
	server := proxy.NewServer(&StaticServerOps{
		RevInfos: map[string][]*proxy.RevInfo{
			"example.org/awesome": {
				{Version: "v0.0.1"},
				{Version: "v1.0.0"},
			},
		},
		LatestVersion: map[string]string{
			"example.org/awesome": "v1.0.0",
		},
		GoModData: map[module.Version][]byte{
			{Path: "example.org/awesome", Version: "v1.0.0"}: []byte("module example.org/awesome\n"),
		},
		ZipData: map[module.Version][]byte{
			{Path: "example.org/awesome", Version: "v1.0.0"}: {},
		},
	})
	http.ListenAndServe(":8080", server)
}

func TestServer_Redirect(t *testing.T) {
	server := proxy.NewServer(&StaticServerOps{
		RevInfos: map[string][]*proxy.RevInfo{
			"example.org/awesome": {
				{Version: "v0.0.1"},
				{Version: "v1.0.0"},
			},
		},
		LatestVersion: map[string]string{
			"example.org/awesome": "v1.0.0",
		},
		GoModData: map[module.Version][]byte{
			{Path: "example.org/awesome", Version: "v1.0.0"}: []byte("module example.org/awesome\n"),
		},
		ZipData: map[module.Version][]byte{
			{Path: "example.org/awesome", Version: "v1.0.0"}: {},
		},
	})

	testServer := httptest.NewServer(server)
	defer testServer.Close()

	resp, err := http.Get(testServer.URL + "/example.org/awesome/@latest")
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, resp.StatusCode)
	}
	var revInfo *proxy.RevInfo
	err = json.NewDecoder(resp.Body).Decode(&revInfo)
	if err != nil {
		t.Fatal(err)
	}
	if revInfo.Version != "v1.0.0" {
		t.Fatalf("expected %s, got %s", "v1.0.0", revInfo.Version)
	}
}
