package zip

import (
	"io"
	"io/fs"
	"path"
	"strings"
	"time"

	"golang.org/x/mod/zip"
)

func Files(fsys fs.FS) (files []zip.File, err error) {
	err = fs.WalkDir(fsys, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		files = append(files, &entryWrapper{fsys: fsys, path: name})
		return nil
	})
	return
}

type entryWrapper struct {
	fsys fs.FS
	path string
}

func (ew *entryWrapper) Path() string {
	return ew.path
}
func (ew *entryWrapper) Lstat() (fs.FileInfo, error) {
	return lstat(ew.fsys, ew.path)
}
func (ew *entryWrapper) Open() (io.ReadCloser, error) {
	return ew.fsys.Open(ew.path)
}

type fileGroup map[string]zip.File

func FS(files ...zip.File) fs.FS {
	fsys := fileGroup{}
	for _, file := range files {
		p := path.Clean(path.Join("/", file.Path()))
		fsys[p] = file
	}
	return fsys
}

func (f fileGroup) Open(name string) (fs.File, error) {
	name = path.Clean(path.Join("/", name))

	if file, ok := f[name]; ok {
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		return &openedFile{file, rc}, nil
	}

	files := map[string]zip.File{}
	for _, file := range f {
		filePath := path.Clean(path.Join("/", file.Path()))
		rel, ok := strings.CutPrefix(filePath, name+"/")
		if !ok {
			continue
		}
		files[rel] = file
	}
	if len(files) == 0 {
		return nil, fs.ErrNotExist
	}
	return &openedDir{name: path.Base(name), files: files}, nil
}

type openedFile struct {
	f  zip.File
	rc io.ReadCloser
}

func (of *openedFile) Stat() (fs.FileInfo, error) {
	return of.f.Lstat()
}
func (of *openedFile) Read(b []byte) (int, error) {
	return of.rc.Read(b)
}
func (of *openedFile) Close() error {
	return of.rc.Close()
}

type openedDir struct {
	name  string
	files map[string]zip.File
}

func (od *openedDir) Stat() (fs.FileInfo, error) {
	return (*openedDirInfo)(od), nil
}
func (od *openedDir) Read(b []byte) (int, error) {
	return 0, fs.ErrInvalid
}
func (od *openedDir) Close() error {
	return nil
}
func (od *openedDir) ReadDir(n int) ([]fs.DirEntry, error) {
	entries := []fs.DirEntry{}
	subdirs := map[string]*openedDir{}
	for rel, file := range od.files {
		if root, rel2, ok := strings.Cut(rel, "/"); ok {
			sd, ok := subdirs[root]
			if !ok {
				sd = &openedDir{name: root, files: map[string]zip.File{}}
				subdirs[root] = sd
				entries = append(entries, fs.FileInfoToDirEntry((*openedDirInfo)(sd)))
			}
			sd.files[rel2] = file
		} else {
			info, err := file.Lstat()
			if err != nil {
				return nil, err
			}
			entries = append(entries, fs.FileInfoToDirEntry(info))
		}
	}
	return entries, nil
}

type openedDirInfo openedDir

func (odi *openedDirInfo) Name() string {
	return odi.name
}
func (odi *openedDirInfo) Size() int64 {
	return 0
}
func (odi *openedDirInfo) Mode() fs.FileMode {
	return fs.ModeDir
}
func (odi *openedDirInfo) ModTime() time.Time {
	return time.Time{}
}
func (odi *openedDirInfo) IsDir() bool {
	return true
}
func (odi *openedDirInfo) Sys() any {
	return (*openedDir)(odi)
}
