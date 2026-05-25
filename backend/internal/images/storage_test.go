package images

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestLocalStorageStripsJPEGEXIF(t *testing.T) {
	storage := NewLocalStorage(t.TempDir(), "/uploads", StorageConfig{StripMetadata: true})
	header := fileHeader(t, "original.jpg", jpegWithEXIF(t))

	stored, err := storage.Save(context.Background(), 12, header)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	data, err := os.ReadFile(stored.FilePath)
	if err != nil {
		t.Fatalf("read stored file: %v", err)
	}
	if bytes.Contains(data, []byte("Exif")) {
		t.Fatal("stored JPEG still contains EXIF marker")
	}
	if strings.Contains(stored.PublicURL, "original.jpg") {
		t.Fatalf("PublicURL preserves original filename: %q", stored.PublicURL)
	}
	if stored.MimeType != "image/jpeg" {
		t.Fatalf("MimeType = %q, want image/jpeg", stored.MimeType)
	}
}

func TestLocalStorageStrippedPNGUploads(t *testing.T) {
	storage := NewLocalStorage(t.TempDir(), "/uploads", StorageConfig{StripMetadata: true})
	var imageBytes bytes.Buffer
	if err := png.Encode(&imageBytes, testImage()); err != nil {
		t.Fatalf("encode png: %v", err)
	}

	stored, err := storage.Save(context.Background(), 12, fileHeader(t, "original.png", imageBytes.Bytes()))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if stored.MimeType != "image/png" {
		t.Fatalf("MimeType = %q, want image/png", stored.MimeType)
	}
	if strings.Contains(stored.PublicURL, "original.png") {
		t.Fatalf("PublicURL preserves original filename: %q", stored.PublicURL)
	}
}

func fileHeader(t *testing.T, name string, content []byte) *multipart.FileHeader {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("images", name)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	request := httptest.NewRequest("POST", "/upload", &body)
	request.Header.Set(echo.HeaderContentType, writer.FormDataContentType())
	if err := request.ParseMultipartForm(1 << 20); err != nil {
		t.Fatalf("ParseMultipartForm: %v", err)
	}
	return request.MultipartForm.File["images"][0]
}

func jpegWithEXIF(t *testing.T) []byte {
	t.Helper()

	var imageBytes bytes.Buffer
	if err := jpeg.Encode(&imageBytes, testImage(), nil); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	data := imageBytes.Bytes()
	app1 := []byte{0xff, 0xe1, 0x00, 0x0c, 'E', 'x', 'i', 'f', 0x00, 0x00, 't', 'e', 's', 't'}
	withEXIF := append([]byte{}, data[:2]...)
	withEXIF = append(withEXIF, app1...)
	withEXIF = append(withEXIF, data[2:]...)
	return withEXIF
}

func testImage() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	img.Set(1, 0, color.RGBA{G: 255, A: 255})
	img.Set(0, 1, color.RGBA{B: 255, A: 255})
	img.Set(1, 1, color.RGBA{R: 255, G: 255, A: 255})
	return img
}
