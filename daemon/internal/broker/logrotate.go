package broker

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// rotatingWriter appends log lines to a file and rotates it when it exceeds
// maxSize bytes. Rotated files are renamed .1, .2, … up to maxBackups, then
// deleted. When compress is true, .1 is gzip-compressed to .1.gz.
type rotatingWriter struct {
	mu         sync.Mutex
	path       string
	file       *os.File
	size       int64
	maxSize    int64
	maxBackups int
	compress   bool
}

func newRotatingWriter(path string, maxSizeMB, maxBackups int, compress bool) *rotatingWriter {
	if maxSizeMB <= 0 {
		maxSizeMB = 100
	}
	if maxBackups <= 0 {
		maxBackups = 5
	}
	return &rotatingWriter{
		path:       path,
		maxSize:    int64(maxSizeMB) * 1024 * 1024,
		maxBackups: maxBackups,
		compress:   compress,
	}
}

// writeLine appends msg + newline to the log, rotating first if needed.
func (w *rotatingWriter) writeLine(msg string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	line := msg + "\n"
	if err := w.openIfNeeded(); err != nil {
		return
	}
	if w.size+int64(len(line)) > w.maxSize {
		_ = w.rotate()
		if err := w.openIfNeeded(); err != nil {
			return
		}
	}
	n, err := fmt.Fprint(w.file, line)
	if err == nil {
		w.size += int64(n)
	}
}

func (w *rotatingWriter) openIfNeeded() error {
	if w.file != nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(w.path), 0o750); err != nil {
		return err
	}
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	w.file = f
	w.size = fi.Size()
	return nil
}

// rotate closes the current file and shifts backups: .N → deleted, .N-1 → .N, …, current → .1.
// If compress is enabled, .1 is gzip-compressed to .1.gz.
func (w *rotatingWriter) rotate() error {
	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
		w.size = 0
	}

	// Drop the oldest backup.
	oldest := fmt.Sprintf("%s.%d", w.path, w.maxBackups)
	_ = os.Remove(oldest)
	if w.compress {
		_ = os.Remove(oldest + ".gz")
	}

	// Shift existing backups: .N-1 → .N
	for i := w.maxBackups - 1; i >= 1; i-- {
		src := fmt.Sprintf("%s.%d", w.path, i)
		dst := fmt.Sprintf("%s.%d", w.path, i+1)
		if w.compress && i == 1 {
			src += ".gz"
			dst += ".gz"
		}
		_ = os.Rename(src, dst)
	}

	// Move current → .1 (and optionally compress).
	backup := w.path + ".1"
	if err := os.Rename(w.path, backup); err != nil {
		return err
	}
	if w.compress {
		_ = gzipFile(backup)
	}
	return nil
}

// close closes the underlying file handle.
func (w *rotatingWriter) close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
	}
}

// gzipFile compresses src to src.gz and removes the original.
func gzipFile(src string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	dst := src + ".gz"
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		return err
	}
	defer out.Close()

	gz := gzip.NewWriter(out)
	if _, err := io.Copy(gz, in); err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	in.Close()
	return os.Remove(src)
}
