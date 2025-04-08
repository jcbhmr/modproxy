package zip

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/jcbhmr/xmod/internal/readerat"
	"golang.org/x/mod/module"
	modzip "golang.org/x/mod/zip"
)

var (
	errPathNotClean    = errors.New("file path is not clean")
	errPathNotRelative = errors.New("file path is not relative")
	errGoModCase       = errors.New("go.mod files must have lowercase names")
	errGoModSize       = fmt.Errorf("go.mod file too large (max size is %d bytes)", modzip.MaxGoMod)
	errLICENSESize     = fmt.Errorf("LICENSE file too large (max size is %d bytes)", modzip.MaxLICENSE)

	errVCS           = errors.New("directory is a version control repository")
	errVendored      = errors.New("file is in vendor directory")
	errSubmoduleFile = errors.New("file is in another module")
	errSubmoduleDir  = errors.New("directory is in another module")
	errHgArchivalTxt = errors.New("file is inserted by 'hg archive' and is always omitted")
	errSymlink       = errors.New("file is a symbolic link")
	errNotRegular    = errors.New("not a regular file")
)

func checkZip(m module.Version, f fs.File) (*zip.Reader, modzip.CheckedFiles, error) {
	if vers := module.CanonicalVersion(m.Version); vers != m.Version {
		return nil, modzip.CheckedFiles{}, fmt.Errorf("version %q is not canonical (should be %q)", m.Version, vers)
	}
	if err := module.Check(m.Path, m.Version); err != nil {
		return nil, modzip.CheckedFiles{}, err
	}

	info, err := f.Stat()
	if err != nil {
		return nil, modzip.CheckedFiles{}, err
	}
	zipSize := info.Size()
	if zipSize > modzip.MaxZipFile {
		cf := modzip.CheckedFiles{SizeError: fmt.Errorf("module zip file is too large (%d bytes; limit is %d bytes)", zipSize, modzip.MaxZipFile)}
		return nil, cf, cf.Err()
	}

	var cf modzip.CheckedFiles
	addError := func(zf *zip.File, err error) {
		cf.Invalid = append(cf.Invalid, modzip.FileError{Path: zf.Name, Err: err})
	}
	var ra io.ReaderAt
	if ra2, ok := f.(io.ReaderAt); ok {
		ra = ra2
	} else if rs, ok := f.(io.ReadSeeker); ok {
		ra = &readerat.SeekingReaderAt{R: rs}
	} else {
		ra = &readerat.BufferedReaderAt{R: f}
	}
	z, err := zip.NewReader(ra, zipSize)
	if err != nil {
		return nil, modzip.CheckedFiles{}, err
	}
	prefix := fmt.Sprintf("%s@%s/", m.Path, m.Version)
	collisions := make(collisionChecker)
	var size int64
	for _, zf := range z.File {
		if !strings.HasPrefix(zf.Name, prefix) {
			addError(zf, fmt.Errorf("path does not have prefix %q", prefix))
			continue
		}
		name := zf.Name[len(prefix):]
		if name == "" {
			continue
		}
		isDir := strings.HasSuffix(name, "/")
		if isDir {
			name = name[:len(name)-1]
		}
		if path.Clean(name) != name {
			addError(zf, errPathNotClean)
			continue
		}
		if err := module.CheckFilePath(name); err != nil {
			addError(zf, err)
			continue
		}
		if err := collisions.check(name, isDir); err != nil {
			addError(zf, err)
			continue
		}
		if isDir {
			continue
		}
		if base := path.Base(name); strings.EqualFold(base, "go.mod") {
			if base != name {
				addError(zf, fmt.Errorf("go.mod file not in module root directory"))
				continue
			}
			if name != "go.mod" {
				addError(zf, errGoModCase)
				continue
			}
		}
		sz := int64(zf.UncompressedSize64)
		if sz >= 0 && modzip.MaxZipFile-size >= sz {
			size += sz
		} else if cf.SizeError == nil {
			cf.SizeError = fmt.Errorf("total uncompressed size of module contents too large (max size is %d bytes)", modzip.MaxZipFile)
		}
		if name == "go.mod" && sz > modzip.MaxGoMod {
			addError(zf, fmt.Errorf("go.mod file too large (max size is %d bytes)", modzip.MaxGoMod))
			continue
		}
		if name == "LICENSE" && sz > modzip.MaxLICENSE {
			addError(zf, fmt.Errorf("LICENSE file too large (max size is %d bytes)", modzip.MaxLICENSE))
			continue
		}
		cf.Valid = append(cf.Valid, zf.Name)
	}

	return z, cf, cf.Err()
}

type collisionChecker map[string]pathInfo

type pathInfo struct {
	path  string
	isDir bool
}

func (cc collisionChecker) check(p string, isDir bool) error {
	fold := strToFold(p)
	if other, ok := cc[fold]; ok {
		if p != other.path {
			return fmt.Errorf("case-insensitive file name collision: %q and %q", other.path, p)
		}
		if isDir != other.isDir {
			return fmt.Errorf("entry %q is both a file and a directory", p)
		}
		if !isDir {
			return fmt.Errorf("multiple entries for file %q", p)
		}
		// It's not an error if check is called with the same directory multiple
		// times. check is called recursively on parent directories, so check
		// may be called on the same directory many times.
	} else {
		cc[fold] = pathInfo{path: p, isDir: isDir}
	}

	if parent := path.Dir(p); parent != "." {
		return cc.check(parent, true)
	}
	return nil
}

// strToFold returns a string with the property that
//
//	strings.EqualFold(s, t) iff strToFold(s) == strToFold(t)
//
// This lets us test a large set of strings for fold-equivalent
// duplicates without making a quadratic number of calls
// to EqualFold. Note that strings.ToUpper and strings.ToLower
// do not have the desired property in some corner cases.
func strToFold(s string) string {
	// Fast path: all ASCII, no upper case.
	// Most paths look like this already.
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= utf8.RuneSelf || 'A' <= c && c <= 'Z' {
			goto Slow
		}
	}
	return s

Slow:
	var buf bytes.Buffer
	for _, r := range s {
		// SimpleFold(x) cycles to the next equivalent rune > x
		// or wraps around to smaller values. Iterate until it wraps,
		// and we've found the minimum value.
		for {
			r0 := r
			r = unicode.SimpleFold(r0)
			if r <= r0 {
				break
			}
		}
		// Exception to allow fast path above: A-Z => a-z
		if 'A' <= r && r <= 'Z' {
			r += 'a' - 'A'
		}
		buf.WriteRune(r)
	}
	return buf.String()
}

func CheckDirFS(fsys fs.FS, dir string) (modzip.CheckedFiles, error) {
	// List files (as CreateFromDir would) and check which ones are omitted
	// or invalid.
	files, omitted, err := listFilesInDirFS(fsys, dir)
	if err != nil {
		return modzip.CheckedFiles{}, err
	}
	cf, cfErr := modzip.CheckFiles(files)
	_ = cfErr // ignore this error; we'll generate our own after rewriting paths.

	// Replace all paths with file system paths.
	// Paths returned by CheckFiles will be slash-separated paths relative to dir.
	// That's probably not appropriate for error messages.
	for i := range cf.Valid {
		cf.Valid[i] = path.Join(".", cf.Valid[i])
	}
	cf.Omitted = append(cf.Omitted, omitted...)
	for i := range cf.Omitted {
		cf.Omitted[i].Path = path.Join(".", cf.Omitted[i].Path)
	}
	for i := range cf.Invalid {
		cf.Invalid[i].Path = path.Join(".", cf.Invalid[i].Path)
	}
	return cf, cf.Err()
}

func listFilesInDirFS(fsys fs.FS, dir string) (files []modzip.File, omitted []modzip.FileError, err error) {
	err = fs.WalkDir(fsys, dir, func(filePath string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relPath, ok := strings.CutPrefix(filePath, dir+"/")
		if !ok {
			return fmt.Errorf("%q not relative to %q", filePath, dir)
		}
		slashPath := relPath

		// Skip some subdirectories inside vendor, but maintain bug
		// golang.org/issue/31562, described in isVendoredPackage.
		// We would like Create and CreateFromDir to produce the same result
		// for a set of files, whether expressed as a directory tree or zip.
		if isVendoredPackage(slashPath) {
			omitted = append(omitted, modzip.FileError{Path: slashPath, Err: errVendored})
			return nil
		}

		if info.IsDir() {
			if filePath == dir {
				// Don't skip the top-level directory.
				return nil
			}

			// Skip VCS directories.
			// fossil repos are regular files with arbitrary names, so we don't try
			// to exclude them.
			switch path.Base(filePath) {
			case ".bzr", ".git", ".hg", ".svn":
				omitted = append(omitted, modzip.FileError{Path: slashPath, Err: errVCS})
				return fs.SkipDir
			}

			// Skip submodules (directories containing go.mod files).
			if goModInfo, err := lstat(fsys, path.Join(filePath, "go.mod")); err == nil && !goModInfo.IsDir() {
				omitted = append(omitted, modzip.FileError{Path: slashPath, Err: errSubmoduleDir})
				return fs.SkipDir
			}
			return nil
		}

		// Skip irregular files and files in vendor directories.
		// Irregular files are ignored. They're typically symbolic links.
		if !info.Type().IsRegular() {
			omitted = append(omitted, modzip.FileError{Path: slashPath, Err: errNotRegular})
			return nil
		}

		files = append(files, dirFileFS{
			fsys:     fsys,
			filePath: filePath,
		})
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return files, omitted, nil
}

type dirFileFS struct {
	fsys     fs.FS
	filePath string
}

func (d dirFileFS) Path() string {
	return d.filePath
}
func (d dirFileFS) Open() (io.ReadCloser, error) {
	return d.fsys.Open(d.filePath)
}
func (d dirFileFS) Lstat() (fs.FileInfo, error) {
	return fs.Stat(d.fsys, d.filePath)
}

// isVendoredPackage attempts to report whether the given filename is contained
// in a package whose import path contains (but does not end with) the component
// "vendor".
//
// Unfortunately, isVendoredPackage reports false positives for files in any
// non-top-level package whose import path ends in "vendor".
func isVendoredPackage(name string) bool {
	var i int
	if strings.HasPrefix(name, "vendor/") {
		i += len("vendor/")
	} else if j := strings.Index(name, "/vendor/"); j >= 0 {
		// This offset looks incorrect; this should probably be
		//
		// 	i = j + len("/vendor/")
		//
		// (See https://golang.org/issue/31562 and https://golang.org/issue/37397.)
		// Unfortunately, we can't fix it without invalidating module checksums.
		i += len("/vendor/")
	} else {
		return false
	}
	return strings.Contains(name[i:], "/")
}

type readLinkFS interface {
	fs.FS
	ReadLink(name string) (string, error)
	Lstat(name string) (fs.FileInfo, error)
}

func lstat(fsys fs.FS, name string) (fs.FileInfo, error) {
	if rlfs, ok := fsys.(readLinkFS); ok {
		return rlfs.Lstat(name)
	}
	return fs.Stat(fsys, name)
}
