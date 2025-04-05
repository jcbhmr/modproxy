package zip

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/jcbhmr/xmod/internal/xfs"
	"golang.org/x/mod/module"
)

func UnzipFS(fsys fs.FS, dir string, m module.Version, zipFile string) (err error) {
	defer func() {
		if err != nil {
			err = &ZipError{Verb: "unzip", Path: zipFile, Err: err}
		}
	}()

	if files, _ := fs.ReadDir(fsys, dir); len(files) > 0 {
		return fmt.Errorf("target directory %v exists and is not empty", dir)
	}

	f, err := fsys.Open(zipFile)
	if err != nil {
		return err
	}
	defer f.Close()
	z, cf, err := checkZip(m, f)
	if err != nil {
		return err
	}
	if err := cf.Err(); err != nil {
		return err
	}

	prefix := fmt.Sprintf("%s@%s/", m.Path, m.Version)
	if err := xfs.MkdirAll(fsys, dir, 0777); err != nil {
		return err
	}
	for _, zf := range z.File {
		name := zf.Name[len(prefix):]
		if name == "" || strings.HasSuffix(name, "/") {
			continue
		}
		dst := path.Join(dir, name)
		if err := os.MkdirAll(path.Dir(dst), 0777); err != nil {
			return err
		}
		w, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0444)
		if err != nil {
			return err
		}
		r, err := zf.Open()
		if err != nil {
			w.Close()
			return err
		}
		lr := &io.LimitedReader{R: r, N: int64(zf.UncompressedSize64) + 1}
		_, err = io.Copy(w, lr)
		r.Close()
		if err != nil {
			w.Close()
			return err
		}
		if err := w.Close(); err != nil {
			return err
		}
		if lr.N <= 0 {
			return fmt.Errorf("uncompressed size of file %s is larger than declared size (%d bytes)", zf.Name, zf.UncompressedSize64)
		}
	}

	return nil
}
