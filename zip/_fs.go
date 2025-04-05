package zip

import (
	"errors"
	"go/version"
	"io"
	"io/fs"
	"os"
	"path"

	"golang.org/x/mod/module"
	"golang.org/x/mod/zip"
)

func CreateFromFS(w io.Writer, m module.Version, dir fs.FS) (err error) {
	defer func() {
		if zerr, ok := err.(*ZipError); ok {
			zerr.Path = "."
		} else if err != nil {
			err = &ZipError{Verb: "create zip from directory", Path: ".", Err: err}
		}
	}()

	files, _, err := listFilesInFS(dir)
	if err != nil {
		return err
	}

	return zip.Create(w, m, files)
}

func CreateFromVCSFS(w io.Writer, m module.Version, repoRoot fs.FS, revision, subdir string) (err error) {
	defer func() {
		if zerr, ok := err.(*ZipError); ok {
			zerr.Path = "."
		} else if err != nil {
			err = &ZipError{Verb: "create zip from version control system", Path: ".", Err: err}
		}
	}()

	var filesToCreate []zip.File

	if isGitRepoFS(repoRoot) {
		files, err := filesInGitRepoFS(repoRoot, revision, subdir)
		if err != nil {
			return err
		}
		filesToCreate = files
	} else {
		return &zip.UnrecognizedVCSError{RepoRoot: "."}
	}

	return zip.Create(w, m, filesToCreate)
}

func CheckDirFS(dir fs.FS) (zip.CheckedFiles, error) {

}

func CheckZipFile(m module.Version, zipFile zip.File) (zip.CheckedFiles, error) {
	panic("todo")
}

func listFilesInDir(dir fs.FS) (files []zip.File, omitted []zip.FileError, err error) {
	var vers string
	if data, err := fs.ReadFile(dir, "go.mod"); err == nil {
		vers = version.Lang(parseGoVers("go.mod", data))
	}
	err = fs.WalkDir(dir, ".", func(filePath string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relPath := filePath
		slashPath := relPath

		// Skip some subdirectories inside vendor.
		// We would like Create and CreateFromDir to produce the same result
		// for a set of files, whether expressed as a directory tree or zip.
		if isVendoredPackage(slashPath, vers) {
			omitted = append(omitted, zip.FileError{Path: slashPath, Err: errors.New("errVendored")})
			return nil
		}

		if info.IsDir() {
			if filePath == "." {
				// Don't skip the top-level directory.
				return nil
			}

			// Skip VCS directories.
			// fossil repos are regular files with arbitrary names, so we don't try
			// to exclude them.
			switch path.Base(filePath) {
			case ".bzr", ".git", ".hg", ".svn":
				omitted = append(omitted, zip.FileError{Path: slashPath, Err: errors.New("errVCS")})
				return fs.SkipDir
			}

			// Skip submodules (directories containing go.mod files).
			if goModInfo, err := os.Lstat(path.Join(filePath, "go.mod")); err == nil && !goModInfo.IsDir() {
				omitted = append(omitted, zip.FileError{Path: slashPath, Err: errors.New("errSubmoduleDir")})
				return fs.SkipDir
			}
			return nil
		}

		// Skip irregular files and files in vendor directories.
		// Irregular files are ignored. They're typically symbolic links.
		if !info.Type().IsRegular() {
			omitted = append(omitted, zip.FileError{Path: slashPath, Err: errors.New("errNotRegular")})
			return nil
		}

		files = append(files, dirFile{
			filePath:  filePath,
			slashPath: slashPath,
			info:      info,
		})
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return files, omitted, nil
}

func filesInGitRepoFS(dir fs.FS, rev, subdir string) ([]zip.File, error) {
	panic("todo")
}

func isGitRepoFS(dir fs.FS) bool {
	info, err := fs.Stat(dir, ".git")
	if err != nil {
		return false
	}
	return info.IsDir()
}
