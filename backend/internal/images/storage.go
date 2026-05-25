package images

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type Storage interface {
	Save(ctx context.Context, entryID int64, file *multipart.FileHeader) (StoredFile, error)
	Delete(ctx context.Context, path string) error
}

type LocalStorage struct {
	dir           string
	publicBase    string
	stripMetadata bool
}

type StorageConfig struct {
	StripMetadata bool
}

var imageExtensions = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

func NewLocalStorage(dir string, publicBase string, configs ...StorageConfig) *LocalStorage {
	cfg := StorageConfig{}
	if len(configs) > 0 {
		cfg = configs[0]
	}
	return &LocalStorage{
		dir:           dir,
		publicBase:    strings.TrimRight(publicBase, "/"),
		stripMetadata: cfg.StripMetadata,
	}
}

func (s *LocalStorage) Save(_ context.Context, entryID int64, header *multipart.FileHeader) (StoredFile, error) {
	source, err := header.Open()
	if err != nil {
		return StoredFile{}, err
	}
	defer source.Close()

	contentType, err := detectContentType(source)
	if err != nil {
		return StoredFile{}, err
	}

	ext, ok := imageExtensions[contentType]
	if !ok {
		return StoredFile{}, ErrInvalidImage
	}

	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return StoredFile{}, err
	}

	name, err := randomName(ext)
	if err != nil {
		return StoredFile{}, err
	}

	path := filepath.Join(s.dir, name)
	target, err := os.Create(path)
	if err != nil {
		return StoredFile{}, err
	}
	defer target.Close()

	size, err := s.writeImage(target, source, contentType)
	if err != nil {
		_ = os.Remove(path)
		return StoredFile{}, err
	}

	return StoredFile{
		FilePath:  path,
		PublicURL: s.publicBase + "/" + name,
		FileName:  safeFileName(header.Filename, entryID, ext),
		Size:      size,
		MimeType:  contentType,
	}, nil
}

func (s *LocalStorage) writeImage(target *os.File, source multipart.File, contentType string) (int64, error) {
	if s.stripMetadata {
		switch contentType {
		case "image/jpeg":
			image, err := jpeg.Decode(source)
			if err != nil {
				return 0, ErrInvalidImage
			}
			if err := jpeg.Encode(target, image, &jpeg.Options{Quality: 90}); err != nil {
				return 0, err
			}
			return target.Seek(0, io.SeekCurrent)
		case "image/png":
			image, err := png.Decode(source)
			if err != nil {
				return 0, ErrInvalidImage
			}
			if err := png.Encode(target, image); err != nil {
				return 0, err
			}
			return target.Seek(0, io.SeekCurrent)
		}
	}

	written, err := io.Copy(target, source)
	return written, err
}

func (s *LocalStorage) Delete(_ context.Context, path string) error {
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (s *LocalStorage) Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (s *LocalStorage) ListFiles() ([]string, error) {
	items, err := os.ReadDir(s.dir)
	if errors.Is(err, os.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}

	files := []string{}
	for _, item := range items {
		if item.IsDir() {
			continue
		}
		files = append(files, filepath.Join(s.dir, item.Name()))
	}
	return files, nil
}

func detectContentType(file multipart.File) (string, error) {
	head := make([]byte, 512)
	n, err := file.Read(head)
	if err != nil && err != io.EOF {
		return "", err
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	return http.DetectContentType(head[:n]), nil
}

func randomName(ext string) (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes) + ext, nil
}

func safeFileName(name string, entryID int64, ext string) string {
	base := filepath.Base(strings.TrimSpace(name))
	if base != "." && base != "" {
		return base
	}
	return strings.TrimSpace("entry") + "-" + hex.EncodeToString([]byte{byte(entryID)}) + ext
}
