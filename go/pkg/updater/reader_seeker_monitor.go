package updater

import "io"

type ReaderSeekerMonitor struct {
	io.ReadSeeker
	f func(offset int64, whence int)
}

// Read implements io.ReadSeeker.
func (r *ReaderSeekerMonitor) Read(p []byte) (int, error) {
	n, err := r.ReadSeeker.Read(p)
	if err != nil {
		return n, err
	}
	r.f(int64(n), io.SeekCurrent)
	return n, err
}

// Seek implements io.ReadSeeker.
func (r *ReaderSeekerMonitor) Seek(offset int64, whence int) (int64, error) {
	n, err := r.ReadSeeker.Seek(offset, whence)
	r.f(offset, whence)
	return n, err
}

func NewReadSeekerMonitor(r io.ReadSeeker, f func(offset int64, whence int)) io.ReadSeeker {
	return &ReaderSeekerMonitor{r, f}
}
