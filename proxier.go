package modproxy

import (
	"context"
	"io"

	"golang.org/x/mod/module"
)

type Proxier interface {
	List(ctx context.Context, path string) ([]string, error)
	Info(ctx context.Context, version module.Version) (Info, error)
	Mod(ctx context.Context, version module.Version) (io.ReadCloser, error)
	Zip(ctx context.Context, version module.Version) (io.ReadCloser, error)
}

type ProxierLatest interface {
	Proxier
	Latest(ctx context.Context, path string) (Info, error)
}
