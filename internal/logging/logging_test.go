package logging

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestRotationWhileRunning(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "tray.log")
	w, err := Open(path, 16)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		if _, err = w.Write([]byte("12345678")); err != nil {
			t.Fatal(err)
		}
	}
	w.Close()
	for _, p := range []string{path, path + ".old"} {
		info, err := os.Stat(p)
		if err != nil || info.Size() > 16 {
			t.Fatal(p, err)
		}
	}
	if _, err = w.Write([]byte("closed")); err == nil {
		t.Fatal("write after close")
	}
}

func TestConcurrentLogWrites(t *testing.T) {
	w, err := Open(filepath.Join(t.TempDir(), "tray.log"), 64)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				if _, err := w.Write([]byte("message\n")); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()
}
