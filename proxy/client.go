package proxy

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"strings"
	"sync/atomic"

	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

var ErrGONOPROXY = errors.New("skipped (listed in GONOPROXY)")

type Client struct {
	ops       ClientOps
	didLookup atomic.Bool
	gonoproxy string
}

type ClientOps interface {
	ReadRemote(path string) ([]byte, error)
	Log(msg string)
}

func NewClient(ops ClientOps) *Client {
	return &Client{ops: ops}
}

func (c *Client) SetGONOPROXY(list string) {
	if c.didLookup.Load() {
		panic("SetGONOPROXY used after lookup")
	}
	if c.gonoproxy != "" {
		panic("multiple calls to SetGONOPROXY")
	}
	c.gonoproxy = list
}

func (c *Client) skip(target string) bool {
	return module.MatchPrefixPatterns(c.gonoproxy, target)
}

func (c *Client) Lookup(path string) (*Repo, error) {
	c.didLookup.Store(true)
	if c.skip(path) {
		return nil, ErrGONOPROXY
	}
	err := module.CheckPath(path)
	if err != nil {
		return nil, err
	}
	return &Repo{c.ops, path}, nil
}

type Repo struct {
	ops  ClientOps
	path string
}

func (r *Repo) Versions(prefix string) ([]string, error) {
	epath, err := module.EscapePath(r.path)
	if err != nil {
		return nil, err
	}
	data, err := r.ops.ReadRemote("/" + epath + "/@v/list")
	if err != nil {
		return nil, err
	}
	data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	if data[len(data)-1] == '\n' {
		data = data[:len(data)-1]
	}
	lines := bytes.Split(data, []byte("\n"))
	versions := []string{}
	for _, line := range lines {
		version := string(line)
		err = module.Check(r.path, version)
		if err != nil {
			return nil, err
		}
		if strings.HasPrefix(version, prefix) {
			versions = append(versions, version)
		}
	}
	return versions, nil
}

func (r *Repo) Latest() (*RevInfo, error) {
	epath, err := module.EscapePath(r.path)
	if err != nil {
		return nil, err
	}
	data, err := r.ops.ReadRemote("/" + epath + "/@latest")
	if errors.Is(err, fs.ErrNotExist) {
		versions, err := r.Versions("")
		if err != nil {
			return nil, err
		}
		if len(versions) == 0 {
			return nil, errors.New("no versions found")
		}
		canonicalVersions := []string{}
		for _, v := range versions {
			if module.CanonicalVersion(v) == v {
				canonicalVersions = append(canonicalVersions, v)
			}
		}
		if len(canonicalVersions) == 0 {
			return nil, errors.New("no canonical versions found")
		}
		semver.Sort(canonicalVersions)
		latest := canonicalVersions[len(canonicalVersions)-1]
		return r.Stat(latest)
	} else if err != nil {
		return nil, err
	}
	var ri *RevInfo
	err = json.Unmarshal(data, &ri)
	if err != nil {
		return nil, err
	}
	if ri.Version == "" {
		return nil, errors.New("RevInfo missing Version")
	}
	return ri, nil
}

func (r *Repo) Stat(version string) (*RevInfo, error) {
	epath, err := module.EscapePath(r.path)
	if err != nil {
		return nil, err
	}
	eversion, err := module.EscapeVersion(version)
	if err != nil {
		return nil, err
	}
	data, err := r.ops.ReadRemote("/" + epath + "/@v/" + eversion + ".info")
	if err != nil {
		return nil, err
	}
	var ri *RevInfo
	err = json.Unmarshal(data, &ri)
	if err != nil {
		return nil, err
	}
	if ri.Version == "" {
		return nil, errors.New("RevInfo missing Version")
	}
	return ri, nil
}

func (r *Repo) GoMod(version string) ([]byte, error) {
	epath, err := module.EscapePath(r.path)
	if err != nil {
		return nil, err
	}
	eversion, err := module.EscapeVersion(version)
	if err != nil {
		return nil, err
	}
	return r.ops.ReadRemote("/" + epath + "/@v/" + eversion + ".mod")
}

func (r *Repo) Zip(dst io.Writer, version string) error {
	epath, err := module.EscapePath(r.path)
	if err != nil {
		return err
	}
	eversion, err := module.EscapeVersion(version)
	if err != nil {
		return err
	}
	if fsys, ok := r.ops.(fs.FS); ok {
		f, err := fsys.Open("/" + epath + "/@v/" + eversion + ".zip")
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(dst, f)
		if err != nil {
			return err
		}
		return nil
	} else {
		data, err := r.ops.ReadRemote("/" + epath + "/@v/" + eversion + ".zip")
		if err != nil {
			return err
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
}
