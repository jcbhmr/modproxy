package writablefs

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
)

type CreateFS interface {
	fs.FS
	Create(name string) (fs.File, error)
}

type MkdirFS interface {
	fs.FS
	Mkdir(name string, perm fs.FileMode) error
}

type OpenFileFS interface {
	fs.FS
	OpenFile(name string, flag int, perm fs.FileMode) (fs.File, error)
}

type RemoveFS interface {
	fs.FS
	Remove(name string) error
}

type WriteFileFS interface {
	fs.FS
	WriteFile(name string, data []byte, perm fs.FileMode) error
}

type WriterFile interface {
	fs.File
	Write(p []byte) (n int, err error)
}

const (
	O_RDONLY = 0x00000
	O_WRONLY = 0x00001
	O_RDWR   = 0x00002
	O_CREAT  = 0x00040
	O_TRUNC  = 0x00200
	O_APPEND = 0x00400
)

func MkdirAll(fsys fs.FS, name string, perm fs.FileMode) error {
	mfs, ok := fsys.(MkdirFS)
	if !ok {
		return fmt.Errorf("%T is not a MkdirFS", fsys)
	}
	for parent := name; path.Dir(parent) != parent; parent = path.Dir(parent) {
		err := mfs.Mkdir(parent, perm)
		if errors.Is(err, fs.ErrExist) {
			// ignore
		} else if err != nil {
			return err
		}
	}
	return nil
}

func WriteFile(fsys fs.FS, name string, data []byte, perm fs.FileMode) error {
	if wffs, ok := fsys.(WriteFileFS); ok {
		return wffs.WriteFile(name, data, perm)
	}
	var wf WriterFile
	if offs, ok := fsys.(OpenFileFS); ok {
		f, err := offs.OpenFile(name, O_CREAT|O_WRONLY|O_TRUNC, perm)
		if err != nil {
			return err
		}
		defer f.Close()
		wf, ok = f.(WriterFile)
		if !ok {
			return fmt.Errorf("%T is not a WriterFile", f)
		}
	} else if cfs, ok := fsys.(CreateFS); ok {
		f, err := cfs.Create(name)
		if err != nil {
			return err
		}
		defer f.Close()
		wf, ok = f.(WriterFile)
		if !ok {
			return fmt.Errorf("%T is not a WriterFile", f)
		}
	} else {
		return fmt.Errorf("%T is not a CreateFS or OpenFileFS", fsys)
	}
	_, err := wf.Write(data)
	return err
}
