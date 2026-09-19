package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/ruskiiamov/school/internal/files"
	"github.com/ruskiiamov/school/internal/storage"
)

const (
	DatabaseName = "school.db"
	FilesDir     = "files"
)

type Service struct {
	dbPath string
	files  *files.Store
}

type Usage struct {
	FileCount int
	FileBytes int64
}

type Archive struct {
	dbCopy string
	files  *files.Store
}

func New(dbPath string, store *files.Store) *Service {
	return &Service{dbPath: dbPath, files: store}
}

func (s *Service) Usage() (Usage, error) {
	ids, err := s.files.IDs()
	if err != nil {
		return Usage{}, err
	}

	usage := Usage{FileCount: len(ids)}

	for _, id := range ids {
		size, err := s.fileSize(id)
		if err != nil {
			return Usage{}, err
		}

		usage.FileBytes += size
	}

	return usage, nil
}

func (s *Service) fileSize(id string) (int64, error) {
	file, err := s.files.Open(id)
	if err != nil {
		return 0, err
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return 0, fmt.Errorf("stat file %s: %w", id, err)
	}

	return info.Size(), nil
}

func (s *Service) Prepare(ctx context.Context) (*Archive, error) {
	tmp, err := os.CreateTemp(filepath.Dir(s.dbPath), "backup-*.db")
	if err != nil {
		return nil, fmt.Errorf("create database copy: %w", err)
	}

	dbCopy := tmp.Name()

	if err := tmp.Close(); err != nil {
		_ = os.Remove(dbCopy)

		return nil, fmt.Errorf("close database copy: %w", err)
	}

	if err := storage.BackupDatabase(ctx, s.dbPath, dbCopy); err != nil {
		_ = os.Remove(dbCopy)

		return nil, err
	}

	return &Archive{dbCopy: dbCopy, files: s.files}, nil
}

func (a *Archive) WriteTo(w io.Writer) (int64, error) {
	counter := &countingWriter{w: w}
	gz := gzip.NewWriter(counter)
	tw := tar.NewWriter(gz)

	if err := a.write(tw); err != nil {
		return counter.n, err
	}

	if err := tw.Close(); err != nil {
		return counter.n, fmt.Errorf("finish archive: %w", err)
	}

	if err := gz.Close(); err != nil {
		return counter.n, fmt.Errorf("finish compression: %w", err)
	}

	return counter.n, nil
}

func (a *Archive) write(tw *tar.Writer) error {
	db, err := os.Open(a.dbCopy)
	if err != nil {
		return fmt.Errorf("open database copy: %w", err)
	}

	if err := addFile(tw, DatabaseName, db); err != nil {
		return err
	}

	if err := tw.WriteHeader(&tar.Header{Name: FilesDir + "/", Typeflag: tar.TypeDir, Mode: 0o700}); err != nil {
		return fmt.Errorf("write header %s: %w", FilesDir, err)
	}

	ids, err := a.files.IDs()
	if err != nil {
		return err
	}

	for _, id := range ids {
		file, err := a.files.Open(id)
		if errors.Is(err, files.ErrNotFound) {
			continue
		}
		if err != nil {
			return err
		}

		if err := addFile(tw, FilesDir+"/"+id, file); err != nil {
			return err
		}
	}

	return nil
}

func addFile(tw *tar.Writer, name string, file *os.File) error {
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat %s: %w", name, err)
	}

	header := &tar.Header{
		Name:    name,
		Mode:    0o600,
		Size:    info.Size(),
		ModTime: info.ModTime().Truncate(time.Second),
	}

	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("write header %s: %w", name, err)
	}

	if _, err := io.Copy(tw, file); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}

	return nil
}

func (a *Archive) Close() error {
	if err := os.Remove(a.dbCopy); err != nil {
		return fmt.Errorf("remove database copy: %w", err)
	}

	return nil
}

type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)

	return n, err
}
