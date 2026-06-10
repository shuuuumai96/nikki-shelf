package auth

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	operationalManifestPath = "manifest.json"
	operationalDumpPath     = "db/postgres.dump"
	operationalUploadsPath  = "uploads/uploads.tar"
	operationalSumsPath     = "SHA256SUMS"
)

type operationalBackup struct {
	tempDir          string
	manifest         operationalManifest
	postgresDumpPath string
	uploadsTarPath   string
	backupSize       int64
	warnings         []string
}

type operationalManifest struct {
	BackupCreatedAt string
	NikkiVersion    string
	SchemaVersion   string
	EntryCount      *int
	ImageCount      *int
}

// prepareOperationalBackup stages an uploaded operational archive into a private
// temp directory. Verification and restore share this contract, so unknown
// members are rejected before any database restore can start.
func prepareOperationalBackup(archivePath string, archiveSize int64) (operationalBackup, error) {
	tempDir, err := os.MkdirTemp("", "nikki-setup-restore-*")
	if err != nil {
		return operationalBackup{}, err
	}
	backup := operationalBackup{
		tempDir:          tempDir,
		postgresDumpPath: filepath.Join(tempDir, "postgres.dump"),
		uploadsTarPath:   filepath.Join(tempDir, "uploads.tar"),
		backupSize:       archiveSize,
	}

	cleanupOnError := true
	defer func() {
		if cleanupOnError {
			_ = os.RemoveAll(tempDir)
		}
	}()

	files, sumsData, err := extractOperationalArchive(archivePath, backup)
	if err != nil {
		return operationalBackup{}, err
	}
	if err := verifyOperationalSums(files, sumsData); err != nil {
		return operationalBackup{}, err
	}
	manifest, warnings, err := parseOperationalManifest(files[operationalManifestPath].data)
	if err != nil {
		return operationalBackup{}, err
	}
	if err := validateUploadsTar(backup.uploadsTarPath); err != nil {
		return operationalBackup{}, err
	}

	backup.manifest = manifest
	backup.warnings = warnings
	cleanupOnError = false
	return backup, nil
}

func (b operationalBackup) cleanup() {
	if b.tempDir != "" {
		_ = os.RemoveAll(b.tempDir)
	}
}

func (b operationalBackup) verifyResponse() SetupRestoreVerifyResponse {
	entryCount := 0
	if b.manifest.EntryCount != nil {
		entryCount = *b.manifest.EntryCount
	}
	imageCount := 0
	if b.manifest.ImageCount != nil {
		imageCount = *b.manifest.ImageCount
	}

	return SetupRestoreVerifyResponse{
		Valid:           true,
		BackupCreatedAt: b.manifest.BackupCreatedAt,
		NikkiVersion:    b.manifest.NikkiVersion,
		SchemaVersion:   b.manifest.SchemaVersion,
		EntryCount:      entryCount,
		ImageCount:      imageCount,
		BackupSizeBytes: b.backupSize,
		Warnings:        b.warnings,
	}
}

type extractedBackupFile struct {
	data []byte
	sum  string
}

func extractOperationalArchive(archivePath string, backup operationalBackup) (map[string]extractedBackupFile, []byte, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, nil, ErrInvalidBackup
	}
	defer gzipReader.Close()

	files := map[string]extractedBackupFile{}
	var sumsData []byte
	reader := tar.NewReader(gzipReader)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, nil, ErrInvalidBackup
		}

		name, err := safeTarPath(header.Name)
		if err != nil {
			return nil, nil, err
		}
		if header.FileInfo().IsDir() {
			continue
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			return nil, nil, ErrInvalidBackup
		}
		if header.Size < 0 {
			return nil, nil, ErrInvalidBackup
		}

		switch name {
		case operationalManifestPath:
			if header.Size > 1<<20 {
				return nil, nil, ErrInvalidBackup
			}
			data, sum, err := readTarFile(reader)
			if err != nil {
				return nil, nil, err
			}
			files[name] = extractedBackupFile{data: data, sum: sum}
		case operationalDumpPath:
			if header.Size > setupRestoreUploadLimitBytes {
				return nil, nil, ErrInvalidBackup
			}
			sum, err := writeTarFile(reader, backup.postgresDumpPath)
			if err != nil {
				return nil, nil, err
			}
			files[name] = extractedBackupFile{sum: sum}
		case operationalUploadsPath:
			if header.Size > setupRestoreUploadLimitBytes {
				return nil, nil, ErrInvalidBackup
			}
			sum, err := writeTarFile(reader, backup.uploadsTarPath)
			if err != nil {
				return nil, nil, err
			}
			files[name] = extractedBackupFile{sum: sum}
		case operationalSumsPath:
			if header.Size > 1<<20 {
				return nil, nil, ErrInvalidBackup
			}
			data, _, err := readTarFile(reader)
			if err != nil {
				return nil, nil, err
			}
			sumsData = data
		default:
			return nil, nil, ErrInvalidBackup
		}
	}

	for _, required := range []string{operationalManifestPath, operationalDumpPath, operationalUploadsPath} {
		if _, ok := files[required]; !ok {
			return nil, nil, ErrInvalidBackup
		}
	}
	return files, sumsData, nil
}

func readTarFile(reader io.Reader) ([]byte, string, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, "", ErrInvalidBackup
	}
	sum := sha256.Sum256(data)
	return data, hex.EncodeToString(sum[:]), nil
}

func writeTarFile(reader io.Reader, target string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return "", err
	}
	file, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(file, io.TeeReader(reader, hasher)); err != nil {
		return "", ErrInvalidBackup
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func verifyOperationalSums(files map[string]extractedBackupFile, sumsData []byte) error {
	if len(sumsData) == 0 {
		return nil
	}

	// SHA256SUMS may be omitted for compatibility, but when present it must
	// cover every payload file the restore path depends on.
	lines := strings.Split(string(sumsData), "\n")
	checked := 0
	seen := map[string]bool{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		sum, name, ok := strings.Cut(line, " ")
		if !ok {
			return ErrInvalidBackup
		}
		name = strings.TrimSpace(strings.TrimPrefix(name, "*"))
		safeName, err := safeTarPath(name)
		if err != nil {
			return err
		}
		file, ok := files[safeName]
		if !ok {
			return ErrInvalidBackup
		}
		if !strings.EqualFold(strings.TrimSpace(sum), file.sum) {
			return ErrInvalidBackup
		}
		seen[safeName] = true
		checked++
	}
	if checked == 0 {
		return ErrInvalidBackup
	}
	for _, required := range []string{operationalManifestPath, operationalDumpPath, operationalUploadsPath} {
		if !seen[required] {
			return ErrInvalidBackup
		}
	}
	return nil
}

func parseOperationalManifest(data []byte) (operationalManifest, []string, error) {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.UseNumber()

	raw := map[string]any{}
	if err := decoder.Decode(&raw); err != nil {
		return operationalManifest{}, nil, ErrInvalidBackup
	}

	// Accept a few historical field names so older operational backups can be
	// inspected, but surface missing current fields as warnings.
	manifest := operationalManifest{
		BackupCreatedAt: firstString(raw, "backupCreatedAt", "createdAt", "timestamp"),
		NikkiVersion:    firstString(raw, "nikkiVersion", "appVersion", "version"),
		SchemaVersion:   firstString(raw, "schemaVersion"),
		EntryCount:      optionalInt(raw, "entryCount"),
		ImageCount:      optionalInt(raw, "imageCount"),
	}
	warnings := []string{}
	if manifest.BackupCreatedAt == "" {
		warnings = append(warnings, "backupCreatedAt is missing")
	}
	if manifest.NikkiVersion == "" {
		warnings = append(warnings, "nikkiVersion is missing")
	}
	if manifest.SchemaVersion == "" {
		warnings = append(warnings, "schemaVersion is missing")
	}
	if manifest.EntryCount == nil {
		warnings = append(warnings, "entryCount is missing")
	}
	if manifest.ImageCount == nil {
		warnings = append(warnings, "imageCount is missing")
	}
	return manifest, warnings, nil
}

func firstString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := values[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return strings.TrimSpace(typed)
			}
		case json.Number:
			return typed.String()
		}
	}
	return ""
}

func optionalInt(values map[string]any, key string) *int {
	value, ok := values[key]
	if !ok {
		return nil
	}
	var parsed int64
	var err error
	switch typed := value.(type) {
	case json.Number:
		parsed, err = typed.Int64()
	case float64:
		parsed = int64(typed)
	case string:
		parsed, err = strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
	default:
		return nil
	}
	if err != nil || parsed < 0 || parsed > int64(^uint(0)>>1) {
		return nil
	}
	result := int(parsed)
	return &result
}

func validateUploadsTar(uploadsTarPath string) error {
	file, err := os.Open(uploadsTarPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Validate the nested uploads archive before pg_restore so malicious paths
	// or special files are rejected while the database is still untouched.
	reader := tar.NewReader(file)
	var totalSize int64
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return ErrInvalidBackup
		}
		if _, err := safeTarPath(header.Name); err != nil {
			return err
		}
		if header.Size < 0 {
			return ErrInvalidBackup
		}
		totalSize += header.Size
		if totalSize > setupRestoreUploadLimitBytes {
			return ErrInvalidBackup
		}
		switch header.Typeflag {
		case tar.TypeReg, tar.TypeRegA, tar.TypeDir:
		default:
			return ErrInvalidBackup
		}
	}
}

func extractUploadsTar(uploadsTarPath string, uploadDir string) error {
	if strings.TrimSpace(uploadDir) == "" {
		return ErrRestoreFailed
	}
	targetRoot, err := filepath.Abs(uploadDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(targetRoot, 0o750); err != nil {
		return err
	}

	file, err := os.Open(uploadsTarPath)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := tar.NewReader(file)
	var totalSize int64
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return ErrInvalidBackup
		}
		name, err := safeTarPath(header.Name)
		if err != nil {
			return err
		}
		if header.Size < 0 {
			return ErrInvalidBackup
		}
		totalSize += header.Size
		if totalSize > setupRestoreUploadLimitBytes {
			return ErrInvalidBackup
		}
		if name == "." {
			continue
		}

		target, err := safeUploadTarget(targetRoot, name)
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, fileMode(header.FileInfo().Mode(), 0o750)); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
				return err
			}
			if err := writeUploadFile(target, reader, header.FileInfo().Mode()); err != nil {
				return err
			}
		default:
			return ErrInvalidBackup
		}
	}
}

func writeUploadFile(target string, reader io.Reader, mode os.FileMode) error {
	file, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, fileMode(mode, 0o640))
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, reader)
	return err
}

func safeUploadTarget(root string, name string) (string, error) {
	target := filepath.Join(root, filepath.FromSlash(name))
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(root, targetAbs)
	if err != nil {
		return "", err
	}
	if relative == "." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || relative == ".." {
		return "", ErrInvalidBackup
	}
	return targetAbs, nil
}

func safeTarPath(name string) (string, error) {
	name = strings.ReplaceAll(strings.TrimSpace(name), "\\", "/")
	if name == "" {
		return "", ErrInvalidBackup
	}
	// Tar member names must remain relative on both Unix and Windows; drive
	// letters are rejected before path.Clean can make them look ordinary.
	if strings.Contains(name, ":") {
		return "", ErrInvalidBackup
	}
	cleaned := path.Clean(name)
	cleaned = strings.TrimPrefix(cleaned, "./")
	if cleaned == "" {
		cleaned = "."
	}
	if path.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, "/../") {
		return "", ErrInvalidBackup
	}
	return cleaned, nil
}

func fileMode(mode os.FileMode, fallback os.FileMode) os.FileMode {
	permissions := mode.Perm()
	if permissions == 0 {
		return fallback
	}
	return permissions
}
