package modproxy_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jcbhmr/modproxy"
	"github.com/psanford/memfs"
	"golang.org/x/mod/module"
	modzip "golang.org/x/mod/zip"
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

func TestProxy(t *testing.T) {
	proxier := &MemModProxy{
		Latests: map[string]string{
			"octocat.github.io/awesome": "v1.0.0",
		},
		Mods: map[module.Version][]byte{
			{Path: "octocat.github.io/awesome", Version: "v1.0.0"}: []byte("module octocat.github.io/awesome\n"),
		},
		Zips: map[module.Version][]byte{
			{Path: "octocat.github.io/awesome", Version: "v1.0.0"}: func() []byte {
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
	ts := httptest.NewServer(&modproxy.Proxy{Proxier: proxier})
	defer ts.Close()

	t.Run("list", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/octocat.github.io/awesome/@v/list")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			err = fmt.Errorf("%s expected status %d, got %d", ts.URL, http.StatusOK, resp.StatusCode)
			t.Fatal(err)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(string(body), "\n")
		if !slices.Contains(lines, "v1.0.0") {
			err = fmt.Errorf("expected version v1.0.0 in response, got %s", string(body))
			t.Fatal(err)
		}
	})

	t.Run("info", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/octocat.github.io/awesome/@v/v1.0.0.info")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			err = fmt.Errorf("%s expected status %d, got %d", ts.URL, http.StatusOK, resp.StatusCode)
			t.Fatal(err)
		}

		var info modproxy.Info
		err = json.NewDecoder(resp.Body).Decode(&info)
		if err != nil {
			t.Fatal(err)
		}
		if info.Version != "v1.0.0" {
			err = fmt.Errorf("expected version v1.0.0 in response, got %s", info.Version)
			t.Fatal(err)
		}
	})

	t.Run("mod", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/octocat.github.io/awesome/@v/v1.0.0.mod")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			err = fmt.Errorf("%s expected status %d, got %d", ts.URL, http.StatusOK, resp.StatusCode)
			t.Fatal(err)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), "module octocat.github.io/awesome") {
			err = fmt.Errorf("expected module directive in response, got %s", string(body))
			t.Fatal(err)
		}
	})

	t.Run("zip", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/octocat.github.io/awesome/@v/v1.0.0.zip")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			err = fmt.Errorf("%s expected status %d, got %d", ts.URL, http.StatusOK, resp.StatusCode)
			t.Fatal(err)
		}

		f, err := os.Create(filepath.Join(t.TempDir(), "octocat.github.io_awesome_v1.0.0.zip"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()

		_, err = io.Copy(f, resp.Body)
		if err != nil {
			t.Fatal(err)
		}

		modzip.Unzip(t.TempDir(), module.Version{Path: "octocat.github.io/awesome", Version: "v1.0.0"}, f.Name())
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("latest", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/octocat.github.io/awesome/@latest")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			err = fmt.Errorf("%s expected status %d, got %d", ts.URL, http.StatusOK, resp.StatusCode)
			t.Fatal(err)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), "v1.0.0") {
			err = fmt.Errorf("expected version v1.0.0 in response, got %s", string(body))
			t.Fatal(err)
		}
	})
}

type RedirectModProxy struct{}

func (m *RedirectModProxy) List(ctx context.Context, path string) ([]string, error) {
	w := modproxy.ResponseWriterFromContext(ctx)
	r := modproxy.RequestFromContext(ctx)
	http.Redirect(w, r, "https://example.org/", http.StatusFound)
	return nil, fs.SkipAll
}

func (m *RedirectModProxy) Info(ctx context.Context, version module.Version) (modproxy.Info, error) {
	w := modproxy.ResponseWriterFromContext(ctx)
	r := modproxy.RequestFromContext(ctx)
	http.Redirect(w, r, "https://example.org/", http.StatusFound)
	return modproxy.Info{}, fs.SkipAll
}

func (m *RedirectModProxy) Mod(ctx context.Context, version module.Version) (io.ReadCloser, error) {
	w := modproxy.ResponseWriterFromContext(ctx)
	r := modproxy.RequestFromContext(ctx)
	http.Redirect(w, r, "https://example.org/", http.StatusFound)
	return nil, fs.SkipAll
}

func (m *RedirectModProxy) Zip(ctx context.Context, version module.Version) (io.ReadCloser, error) {
	w := modproxy.ResponseWriterFromContext(ctx)
	r := modproxy.RequestFromContext(ctx)
	http.Redirect(w, r, "https://example.org/", http.StatusFound)
	return nil, fs.SkipAll
}

func (m *RedirectModProxy) Latest(ctx context.Context, path string) (modproxy.Info, error) {
	w := modproxy.ResponseWriterFromContext(ctx)
	r := modproxy.RequestFromContext(ctx)
	http.Redirect(w, r, "https://example.org/", http.StatusFound)
	return modproxy.Info{}, fs.SkipAll
}

func TestProxy_CustomResponse(t *testing.T) {
	proxier := &RedirectModProxy{}
	ts := httptest.NewServer(&modproxy.Proxy{Proxier: proxier})
	defer ts.Close()

	noRedirects := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	t.Run("list", func(t *testing.T) {
		resp, err := noRedirects.Get(ts.URL + "/octocat.github.io/awesome/@v/list")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusFound {
			err = fmt.Errorf("%s expected status %d, got %d", ts.URL, http.StatusFound, resp.StatusCode)
			t.Fatal(err)
		}

		if resp.Header.Get("Location") != "https://example.org/" {
			err = fmt.Errorf("expected redirect to %s, got %s", "https://example.org/", resp.Header.Get("Location"))
			t.Fatal(err)
		}
	})

	t.Run("info", func(t *testing.T) {
		resp, err := noRedirects.Get(ts.URL + "/octocat.github.io/awesome/@v/v1.0.0.info")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusFound {
			err = fmt.Errorf("%s expected status %d, got %d", ts.URL, http.StatusFound, resp.StatusCode)
			t.Fatal(err)
		}

		if resp.Header.Get("Location") != "https://example.org/" {
			err = fmt.Errorf("expected redirect to %s, got %s", "https://example.org/", resp.Header.Get("Location"))
			t.Fatal(err)
		}
	})

	t.Run("mod", func(t *testing.T) {
		resp, err := noRedirects.Get(ts.URL + "/octocat.github.io/awesome/@v/v1.0.0.mod")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusFound {
			err = fmt.Errorf("%s expected status %d, got %d", ts.URL, http.StatusFound, resp.StatusCode)
			t.Fatal(err)
		}

		if resp.Header.Get("Location") != "https://example.org/" {
			err = fmt.Errorf("expected redirect to %s, got %s", "https://example.org/", resp.Header.Get("Location"))
			t.Fatal(err)
		}
	})

	t.Run("zip", func(t *testing.T) {
		resp, err := noRedirects.Get(ts.URL + "/octocat.github.io/awesome/@v/v1.0.0.zip")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusFound {
			err = fmt.Errorf("%s expected status %d, got %d", ts.URL, http.StatusFound, resp.StatusCode)
			t.Fatal(err)
		}

		if resp.Header.Get("Location") != "https://example.org/" {
			err = fmt.Errorf("expected redirect to %s, got %s", "https://example.org/", resp.Header.Get("Location"))
			t.Fatal(err)
		}
	})

	t.Run("latest", func(t *testing.T) {
		resp, err := noRedirects.Get(ts.URL + "/octocat.github.io/awesome/@latest")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusFound {
			err = fmt.Errorf("%s expected status %d, got %d", ts.URL, http.StatusFound, resp.StatusCode)
			t.Fatal(err)
		}

		if resp.Header.Get("Location") != "https://example.org/" {
			err = fmt.Errorf("expected redirect to %s, got %s", "https://example.org/", resp.Header.Get("Location"))
			t.Fatal(err)
		}
	})
}
