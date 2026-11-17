package storage

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path"
	"strings"
)

// Store: object rakha ar muche fela. Key "/" diye, ".." ba khali ongsho na.
type Store interface {
	Put(ctx context.Context, key string, body []byte, contentType string) error
	Delete(ctx context.Context, key string) error
}

var errKey = errors.New("storage: bhul key")

// validKey: relative, clean, shudhu "/" separator; dir er baire jawa ba ajob path na.
func validKey(k string) bool {
	return k != "" && !strings.HasPrefix(k, "/") && !strings.ContainsAny(k, "\\\x00") && path.Clean(k) == k &&
		k != "." && k != ".." && !strings.HasPrefix(k, "../")
}

// Local: UPLOAD_DIR e file (local e Caddy img.localhost serve kore). os.Root dir er baire jete dey na.
type Local struct{ Dir string }

func (l Local) root() (*os.Root, error) {
	if err := os.MkdirAll(l.Dir, 0o750); err != nil {
		return nil, err
	}
	return os.OpenRoot(l.Dir)
}

func (l Local) Put(_ context.Context, key string, body []byte, _ string) error {
	if !validKey(key) {
		return errKey
	}
	r, err := l.root()
	if err != nil {
		return err
	}
	defer r.Close()
	if err := r.MkdirAll(path.Dir(key), 0o750); err != nil {
		return err
	}
	return r.WriteFile(key, body, 0o640)
}

// Delete: nai holeo ok (R2 er moto).
func (l Local) Delete(_ context.Context, key string) error {
	if !validKey(key) {
		return errKey
	}
	r, err := l.root()
	if err != nil {
		return err
	}
	defer r.Close()
	if err := r.Remove(key); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
