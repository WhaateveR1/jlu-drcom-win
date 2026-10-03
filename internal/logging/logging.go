package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

const MaxLogBytes int64 = 4 * 1024 * 1024

type RotatingWriter struct {
	mu    sync.Mutex
	path  string
	limit int64
	file  *os.File
	size  int64
}

func Open(path string, limit int64) (*RotatingWriter, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	return &RotatingWriter{path: path, limit: limit, file: f, size: info.Size()}, nil
}

func (w *RotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return 0, os.ErrClosed
	}
	if w.size > 0 && w.size+int64(len(p)) > w.limit {
		if err := w.file.Close(); err != nil {
			return 0, err
		}
		w.file = nil
		if err := os.Remove(w.path + ".old"); err != nil && !os.IsNotExist(err) {
			return 0, err
		}
		if err := os.Rename(w.path, w.path+".old"); err != nil {
			return 0, err
		}
		f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			return 0, err
		}
		w.file, w.size = f, 0
	}
	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *RotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

func UserLogger() (*slog.Logger, io.Closer, string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return nil, nil, "", err
	}
	dir := filepath.Join(base, "jlu-drcom-win", "logs")
	w, err := Open(filepath.Join(dir, "tray.log"), MaxLogBytes)
	if err != nil {
		return nil, nil, "", err
	}
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelDebug})), w, dir, nil
}
