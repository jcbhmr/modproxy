package zip

import (
	"io"
	"io/fs"

	"golang.org/x/mod/module"
	"golang.org/x/mod/zip"
)

func CreateFromFS(fsys fs.FS, w io.Writer, m module.Version, dir string) (err error) {
	defer func() {
		if zerr, ok := err.(*ZipError); ok {
			zerr.Path = "."
		} else if err != nil {
			err = &ZipError{Verb: "create zip from directory", Path: dir, Err: err}
		}
	}()

	files, _, err := listFilesInDirFS(fsys, dir)
	if err != nil {
		return err
	}

	return zip.Create(w, m, files)
}
