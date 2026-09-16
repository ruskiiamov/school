package files

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
)

var (
	ErrTooLarge = errors.New("file too large")
	ErrNotFound = errors.New("file not found")

	idPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)
)

const sniffLength = 512

type Store struct {
	dir string
}

type Saved struct {
	ID          string
	Size        int64
	ContentType string
}

func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create files directory: %w", err)
	}

	return &Store{dir: dir}, nil
}

func (s *Store) Save(r io.Reader, maxSize int64) (Saved, error) {
	id, err := newID()
	if err != nil {
		return Saved{}, err
	}

	file, err := os.OpenFile(s.path(id), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return Saved{}, fmt.Errorf("create file: %w", err)
	}

	saved, err := s.write(file, id, r, maxSize)
	if err != nil {
		_ = file.Close()
		_ = os.Remove(s.path(id))

		return Saved{}, err
	}

	if err := file.Close(); err != nil {
		_ = os.Remove(s.path(id))

		return Saved{}, fmt.Errorf("close file: %w", err)
	}

	return saved, nil
}

func (s *Store) write(file *os.File, id string, r io.Reader, maxSize int64) (Saved, error) {
	head := make([]byte, sniffLength)

	n, err := io.ReadFull(r, head)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return Saved{}, fmt.Errorf("read file: %w", err)
	}

	head = head[:n]

	size, err := io.Copy(file, io.MultiReader(bytes.NewReader(head), io.LimitReader(r, maxSize-int64(n)+1)))
	if err != nil {
		return Saved{}, fmt.Errorf("write file: %w", err)
	}

	if size > maxSize {
		return Saved{}, ErrTooLarge
	}

	return Saved{ID: id, Size: size, ContentType: http.DetectContentType(head)}, nil
}

func (s *Store) Open(id string) (*os.File, error) {
	if !idPattern.MatchString(id) {
		return nil, ErrNotFound
	}

	file, err := os.Open(s.path(id))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}

	return file, nil
}

func (s *Store) Delete(id string) error {
	if !idPattern.MatchString(id) {
		return ErrNotFound
	}

	err := os.Remove(s.path(id))
	if errors.Is(err, os.ErrNotExist) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("remove file: %w", err)
	}

	return nil
}

func (s *Store) IDs() ([]string, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("read files directory: %w", err)
	}

	var ids []string

	for _, entry := range entries {
		if !entry.IsDir() && idPattern.MatchString(entry.Name()) {
			ids = append(ids, entry.Name())
		}
	}

	return ids, nil
}

func (s *Store) path(id string) string {
	return filepath.Join(s.dir, id)
}

func newID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate file id: %w", err)
	}

	return hex.EncodeToString(raw), nil
}
