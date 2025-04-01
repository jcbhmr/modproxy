package modproxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strconv"
	"strings"

	"golang.org/x/mod/module"
)

type Proxy struct {
	Proxier Proxier
}

type key int

const (
	requestKey key = iota
	responseWriterKey
)

func RequestFromContext(ctx context.Context) *http.Request {
	return ctx.Value(requestKey).(*http.Request)
}

func ResponseWriterFromContext(ctx context.Context) http.ResponseWriter {
	return ctx.Value(responseWriterKey).(http.ResponseWriter)
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var err error

	if !(r.Method == http.MethodGet || r.Method == http.MethodHead) {
		http.Error(w, "405 method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if cleanPath(r.URL.Path) != r.URL.Path {
		err = fmt.Errorf("path %q is not clean", r.URL.Path)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.HasSuffix(r.URL.Path, "/") {
		err = fmt.Errorf("path %q has trailing slash", r.URL.Path)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	escapedModulePath, after, ok := strings.Cut(r.URL.Path[1:], "/@")
	if !ok {
		err = fmt.Errorf("path %q does not contain %q", r.URL.Path, "/@")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	modulePath, err := module.UnescapePath(escapedModulePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	ctx = context.WithValue(ctx, requestKey, r)
	ctx = context.WithValue(ctx, responseWriterKey, w)

	if after == "latest" {
		p.serveLatest(w, r, ctx, modulePath)
		return
	}
	if after == "v/list" {
		p.serveList(w, r, ctx, modulePath)
		return
	}

	if !strings.HasPrefix(after, "v/") {
		err = &module.ModuleError{
			Path: modulePath,
			Err:  fmt.Errorf("path %q does not start with %q", after, "v/"),
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	after = after[2:]

	ext := path.Ext(after)
	if !(ext == ".info" || ext == ".mod" || ext == ".zip") {
		err = &module.ModuleError{
			Path: modulePath,
			Err:  fmt.Errorf("extension %q not in %v", ext, []string{".info", ".mod", ".zip"}),
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	escapedVersion := strings.TrimSuffix(after, ext)
	version, err := module.UnescapeVersion(escapedVersion)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if version == "latest" || version == "upgrade" || version == "patch" {
		err = &module.ModuleError{
			Path: modulePath,
			Err: &module.InvalidVersionError{
				Version: version,
				Err:     errors.New("version is special"),
			},
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if ext == ".info" {
		p.serveInfo(w, r, ctx, modulePath, version)
		return
	}

	err = module.Check(modulePath, version)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if module.CanonicalVersion(version) != version {
		err = &module.ModuleError{
			Path: modulePath,
			Err: &module.InvalidVersionError{
				Version: version,
				Err:     errors.New("version is not canonical"),
			},
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if ext == ".mod" {
		p.serveMod(w, r, ctx, modulePath, version)
		return
	}
	if ext == ".zip" {
		p.serveZip(w, r, ctx, modulePath, version)
		return
	}
	panic("unreachable")
}

func (p *Proxy) serveLatest(w http.ResponseWriter, r *http.Request, ctx context.Context, modulePath string) {
	var err error

	p2, ok := p.Proxier.(ProxierLatest)
	if !ok {
		err = fmt.Errorf("proxier %T does not implement ProxierLatest", p.Proxier)
		http.Error(w, err.Error(), http.StatusNotImplemented)
		return
	}

	version, err := p2.Latest(ctx, modulePath)
	if err != nil {
		if errors.Is(err, fs.SkipAll) {
			return
		} else if errors.Is(err, fs.ErrNotExist) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		} else if errors.Is(err, fs.ErrPermission) {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	body, err := json.Marshal(version)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.Write(body)
}

func (p *Proxy) serveList(w http.ResponseWriter, r *http.Request, ctx context.Context, modulePath string) {
	var err error

	versions, err := p.Proxier.List(ctx, modulePath)
	if err != nil {
		if errors.Is(err, fs.SkipAll) {
			return
		} else if errors.Is(err, fs.ErrNotExist) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		} else if errors.Is(err, fs.ErrPermission) {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	body := strings.Join(versions, "\n")

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.Write([]byte(body))
}

func (p *Proxy) serveInfo(w http.ResponseWriter, r *http.Request, ctx context.Context, modulePath, version string) {
	var err error

	info, err := p.Proxier.Info(ctx, module.Version{Path: modulePath, Version: version})
	if err != nil {
		if errors.Is(err, fs.SkipAll) {
			return
		} else if errors.Is(err, fs.ErrNotExist) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		} else if errors.Is(err, fs.ErrPermission) {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	body, err := json.Marshal(info)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.Write(body)
}

func (p *Proxy) serveMod(w http.ResponseWriter, r *http.Request, ctx context.Context, modulePath, version string) {
	var err error

	mod, err := p.Proxier.Mod(ctx, module.Version{Path: modulePath, Version: version})
	if err != nil {
		if errors.Is(err, fs.SkipAll) {
			return
		} else if errors.Is(err, fs.ErrNotExist) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		} else if errors.Is(err, fs.ErrPermission) {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	defer mod.Close()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	io.Copy(w, mod)
}

func (p *Proxy) serveZip(w http.ResponseWriter, r *http.Request, ctx context.Context, modulePath, version string) {
	var err error

	zip2, err := p.Proxier.Zip(ctx, module.Version{Path: modulePath, Version: version})
	if err != nil {
		if errors.Is(err, fs.SkipAll) {
			return
		} else if errors.Is(err, fs.ErrNotExist) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		} else if errors.Is(err, fs.ErrPermission) {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	defer zip2.Close()

	w.Header().Set("Content-Type", "application/zip")
	io.Copy(w, zip2)
}

func cleanPath(p string) string {
	if p == "" {
		return "/"
	}
	if p[0] != '/' {
		p = "/" + p
	}
	np := path.Clean(p)
	if p[len(p)-1] == '/' && np != "/" {
		if len(p) == len(np)+1 && strings.HasPrefix(p, np) {
			return p
		}
		return np + "/"
	}
	return np
}
