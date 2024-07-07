package updater

import "io"

type ReaderMonitor struct {
	io.Reader
	f func(offset int64)
}

// Read implements io.Reader.
func (r *ReaderMonitor) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if err != nil {
		return n, err
	}
	r.f(int64(n))
	return n, err
}

func NewReaderMonitor(r io.Reader, f func(offset int64)) io.Reader {
	return &ReaderMonitor{r, f}
}
