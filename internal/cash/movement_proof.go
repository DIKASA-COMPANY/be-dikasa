package cash

import (
	"be-dikasa/internal/shared/imagefile"
	"context"
	"github.com/google/uuid"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ProofObject names one confined private evidence file.
type ProofObject struct{ Key string }

// MovementProofStore is separate from public product and avatar media.
type MovementProofStore interface {
	Save(context.Context, string, string, imagefile.Image) (ProofObject, error)
	Read(context.Context, string) ([]byte, string, error)
	Delete(context.Context, string) error
}

// LocalMovementProofStore exposes no public HTTP handler.
type LocalMovementProofStore struct{ root *os.Root }

// NewLocalMovementProofStore rejects symlink roots before opening its filesystem boundary.
func NewLocalMovementProofStore(directory string) (*LocalMovementProofStore, error) {
	a, e := filepath.Abs(directory)
	if e != nil {
		return nil, e
	}
	if e = os.MkdirAll(a, 0700); e != nil {
		return nil, e
	}
	r, e := filepath.EvalSymlinks(a)
	if e != nil || !strings.EqualFold(a, r) {
		return nil, ErrProofStorage
	}
	root, e := os.OpenRoot(a)
	if e != nil {
		return nil, e
	}
	return &LocalMovementProofStore{root}, nil
}

// Close releases the private filesystem boundary.
func (s *LocalMovementProofStore) Close() error { return s.root.Close() }

// Save writes original validated bytes with an exclusive generated filename.
func (s *LocalMovementProofStore) Save(_ context.Context, out, sid string, v imagefile.Image) (ProofObject, error) {
	if _, e := uuid.Parse(out); e != nil {
		return ProofObject{}, ErrProofInvalid
	}
	if _, e := uuid.Parse(sid); e != nil {
		return ProofObject{}, ErrProofInvalid
	}
	if len(v.Bytes) < 1 || len(v.Bytes) > 3145728 || (v.Extension != ".png" && v.Extension != ".jpg") {
		return ProofObject{}, ErrProofInvalid
	}
	key := uuid.MustParse(out).String() + "_" + uuid.MustParse(sid).String() + "_" + uuid.NewString() + v.Extension
	f, e := s.root.OpenFile(key, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return ProofObject{}, ErrProofStorage
	}
	_, e = f.Write(v.Bytes)
	ce := f.Close()
	if e != nil || ce != nil {
		s.root.Remove(key)
		return ProofObject{}, ErrProofStorage
	}
	return ProofObject{key}, nil
}

// validProofKey accepts only canonical generated identifiers and an image extension.
func validProofKey(key string) bool {
	ext := filepath.Ext(key)
	if ext != ".png" && ext != ".jpg" {
		return false
	}
	p := strings.Split(strings.TrimSuffix(key, ext), "_")
	if len(p) != 3 {
		return false
	}
	for _, v := range p {
		u, e := uuid.Parse(v)
		if e != nil || u.String() != v {
			return false
		}
	}
	return true
}

// Read refuses symlinks, traversal, directories and oversized stored files.
func (s *LocalMovementProofStore) Read(_ context.Context, key string) ([]byte, string, error) {
	if !validProofKey(key) {
		return nil, "", ErrProofInvalid
	}
	i, e := s.root.Lstat(key)
	if e != nil || !i.Mode().IsRegular() || i.Size() > 3145728 {
		return nil, "", ErrProofStorage
	}
	f, e := s.root.Open(key)
	if e != nil {
		return nil, "", ErrProofStorage
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, 3145729))
	if e != nil || len(b) > 3145728 {
		return nil, "", ErrProofStorage
	}
	mime := "image/jpeg"
	if strings.HasSuffix(key, ".png") {
		mime = "image/png"
	}
	return b, mime, nil
}

// Delete removes generated regular files only; failures may leave an orphan.
func (s *LocalMovementProofStore) Delete(_ context.Context, key string) error {
	if !validProofKey(key) {
		return ErrProofInvalid
	}
	i, e := s.root.Lstat(key)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil || !i.Mode().IsRegular() {
		return ErrProofStorage
	}
	return s.root.Remove(key)
}
