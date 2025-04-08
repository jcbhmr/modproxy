package readerat

import (
	"bytes"
	"io"
)

type SeekingReaderAt struct {
	R io.ReadSeeker
}

func (s *SeekingReaderAt) ReadAt(p []byte, off int64) (n int, err error) {
	_, err = s.R.Seek(off, io.SeekStart)
	if err != nil {
		return 0, err
	}
	return s.R.Read(p)
}

type BufferedReaderAt struct {
	R   io.Reader
	Buf bytes.Buffer
}

func (b *BufferedReaderAt) ReadAt(p []byte, off int64) (n int, err error) {
	end := off + int64(len(p))
	need := end - int64(b.Buf.Len())
	if need > 0 {
		_, err := io.CopyN(&b.Buf, b.R, need)
		if err != nil {
			return 0, err
		}
	}
	copy(p, b.Buf.Bytes()[off:end])
	return len(p), nil
}
