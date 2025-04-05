package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"strings"

	"golang.org/x/mod/module"
)

type Server struct {
	ops   ServerOps
	mux   http.ServeMux
	remux http.ServeMux
}

type ServerOps interface {
	Versions(ctx context.Context, path string) ([]string, error)
	Stat(ctx context.Context, m module.Version) (*RevInfo, error)
	GoMod(ctx context.Context, m module.Version) ([]byte, error)
	Zip(ctx context.Context, dst io.Writer, m module.Version) error
}

type ServerOpsLatest interface {
	ServerOps
	Latest(ctx context.Context, path string) (*RevInfo, error)
}

func NewServer(ops ServerOps) *Server {
	s := &Server{ops: ops}
	s.mux.HandleFunc("GET /{rest...}", func(w http.ResponseWriter, r *http.Request) {
		rest := r.PathValue("rest")
		var err error
		epath, afterSlashAt, ok := strings.Cut(rest, "/@")
		if !ok {
			err = fmt.Errorf("no %q token in %q", "/@v/", r.URL.Path)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		routePath := "/@" + afterSlashAt
		pathVar, err := module.UnescapePath(epath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var newRoutePath string
		if routePath == "/@v/list" {
			newRoutePath = "/@v/list"
			return
		} else if strings.HasPrefix(routePath, "/@v/") {
			ext := path.Ext(routePath)
			if ext == ".info" || ext == ".mod" || ext == ".zip" {
				newRoutePath = "/@v/" + strings.TrimSuffix(routePath, ext) + "/" + ext
			} else {
				err = fmt.Errorf("unknown extension %q", ext)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		} else if routePath == "/@latest" {
			newRoutePath = "/@latest"
		} else {
			err = fmt.Errorf("unknown route %q", routePath)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		r.URL.RawPath = r.URL.Path
		r.URL.Path = "/" + url.PathEscape(pathVar) + newRoutePath
		s.remux.ServeHTTP(w, r)
	})
	s.remux.HandleFunc("GET /{path}/@v/list", func(w http.ResponseWriter, r *http.Request) {
		versions, err := s.ops.Versions(r.Context(), r.URL.Path[1:])
		if errors.Is(err, fs.ErrNotExist) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		for _, v := range versions {
			fmt.Fprintln(w, v)
		}
	})
	s.remux.HandleFunc("GET /{path}/@v/{version}/.info", func(w http.ResponseWriter, r *http.Request) {
		path, _ := url.PathUnescape(r.PathValue("path"))
		version, _ := url.PathUnescape(r.PathValue("version"))
		ri, err := s.ops.Stat(r.Context(), module.Version{Path: path, Version: version})
		if errors.Is(err, fs.ErrNotExist) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ri)
	})
	s.remux.HandleFunc("GET /{path}/@v/{version}/.mod", func(w http.ResponseWriter, r *http.Request) {
		path, _ := url.PathUnescape(r.PathValue("path"))
		version, _ := url.PathUnescape(r.PathValue("version"))
		data, err := s.ops.GoMod(r.Context(), module.Version{Path: path, Version: version})
		if errors.Is(err, fs.ErrNotExist) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	})
	s.remux.HandleFunc("GET /{path}/@v/{version}/.zip", func(w http.ResponseWriter, r *http.Request) {
		path, _ := url.PathUnescape(r.PathValue("path"))
		version, _ := url.PathUnescape(r.PathValue("version"))
		w.Header().Set("Content-Type", "application/zip")
		w.WriteHeader(http.StatusOK)
		sw := &sizeWriter{Writer: w}
		err := s.ops.Zip(r.Context(), sw, module.Version{Path: path, Version: version})
		if sw.Size == 0 {
			if errors.Is(err, fs.ErrNotExist) {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			} else if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	})
	s.remux.HandleFunc("GET /{path}/@latest", func(w http.ResponseWriter, r *http.Request) {
		path, _ := url.PathUnescape(r.PathValue("path"))
		ri, err := s.ops.(ServerOpsLatest).Latest(r.Context(), path)
		if errors.Is(err, fs.ErrNotExist) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ri)
	})
	return s
}

type sizeWriter struct {
	io.Writer
	Size int64
}

func (w *sizeWriter) Write(p []byte) (int, error) {
	n, err := w.Writer.Write(p)
	w.Size += int64(n)
	return n, err
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}
