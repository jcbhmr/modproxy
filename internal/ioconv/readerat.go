package ioconv

import (
	"bytes"
	"io"
)

type seekingReaderAt struct {
	r io.ReadSeeker
}

func (s *seekingReaderAt) ReadAt(p []byte, off int64) (n int, err error) {
	_, err = s.r.Seek(off, io.SeekStart)
	if err != nil {
		return 0, err
	}
	return s.r.Read(p)
}

type bufferedReaderAt struct {
	r   io.Reader
	buf bytes.Buffer
}

func (b *bufferedReaderAt) ReadAt(p []byte, off int64) (n int, err error) {
	end := off + int64(len(p))
	need := end - int64(b.buf.Len())
	if need > 0 {
		_, err := io.CopyN(&b.buf, b.r, need)
		if err != nil {
			return 0, err
		}
	}
	copy(p, b.buf.Bytes()[off:end])
	return len(p), nil
}

func ReaderToReaderAt(r io.Reader) io.ReaderAt {
	if ra, ok := r.(io.ReaderAt); ok {
		return ra
	}
	if sr, ok := r.(io.ReadSeeker); ok {
		return &seekingReaderAt{r: sr}
	}
	return &bufferedReaderAt{r: r}
}
