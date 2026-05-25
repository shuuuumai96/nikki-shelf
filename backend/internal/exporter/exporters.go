package exporter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shuuuumai96/nikki-shelf/backend/internal/entries"
)

type Exporter interface {
	ContentType() string
	FileName() string
	Export([]entries.EntryResponse) ([]byte, error)
}

var Exporters = map[string]Exporter{
	"json":     JSONExporter{},
	"markdown": MarkdownExporter{},
}

type BackupExporter struct{}

func (BackupExporter) ContentType() string {
	return "application/zip"
}

func (BackupExporter) FileName() string {
	return "nikki-backup.zip"
}

func (BackupExporter) Export([]entries.EntryResponse) ([]byte, error) {
	return nil, ErrUnsupportedFormat
}

type JSONExporter struct{}

func (JSONExporter) ContentType() string {
	return "application/json; charset=utf-8"
}

func (JSONExporter) FileName() string {
	return "nikki-export.json"
}

func (JSONExporter) Export(items []entries.EntryResponse) ([]byte, error) {
	return json.MarshalIndent(items, "", "  ")
}

type MarkdownExporter struct{}

func (MarkdownExporter) ContentType() string {
	return "text/markdown; charset=utf-8"
}

func (MarkdownExporter) FileName() string {
	return "nikki-export.md"
}

func (MarkdownExporter) Export(items []entries.EntryResponse) ([]byte, error) {
	buffer := bytes.NewBuffer(nil)
	for _, item := range items {
		title := strings.TrimSpace(item.Title)
		if title == "" {
			title = item.EntryDate
		}

		buffer.WriteString("## " + title + "\n\n")
		buffer.WriteString("- Date: " + item.EntryDate + "\n")
		buffer.WriteString("- Mood: " + item.Mood + "\n")
		if len(item.Tags) > 0 {
			buffer.WriteString("- Tags: " + strings.Join(item.Tags, ", ") + "\n")
		}
		buffer.WriteString("\n")
		buffer.WriteString(item.Body)
		buffer.WriteString("\n\n")
	}
	return buffer.Bytes(), nil
}

type EntryMarkdownExporter struct {
	EntryDate string
}

func (EntryMarkdownExporter) ContentType() string {
	return "text/markdown; charset=utf-8"
}

func (e EntryMarkdownExporter) FileName() string {
	return "nikki-entry-" + e.EntryDate + ".md"
}

func (EntryMarkdownExporter) Export(items []entries.EntryResponse) ([]byte, error) {
	if len(items) != 1 {
		return nil, ErrUnsupportedFormat
	}
	item := items[0]
	buffer := bytes.NewBuffer(nil)
	title := strings.TrimSpace(item.Title)
	if title == "" {
		title = item.EntryDate
	}

	buffer.WriteString("# " + title + "\n\n")
	buffer.WriteString("Date: " + item.EntryDate + "\n")
	if strings.TrimSpace(item.Mood) != "" {
		buffer.WriteString("Mood: " + item.Mood + "\n")
	}
	if len(item.Tags) > 0 {
		buffer.WriteString("Tags: " + strings.Join(item.Tags, ", ") + "\n")
	}
	buffer.WriteString("\n")
	buffer.WriteString(item.Body)
	buffer.WriteString("\n")

	imageURLs := safeImageURLs(item.Images)
	if len(imageURLs) > 0 {
		buffer.WriteString("\n## Images\n\n")
		for index, url := range imageURLs {
			buffer.WriteString(fmt.Sprintf("![Image %d](%s)\n", index+1, url))
		}
	}

	return buffer.Bytes(), nil
}

func safeImageURLs(images []entries.EntryImage) []string {
	urls := make([]string, 0, len(images))
	for _, image := range images {
		url := strings.TrimSpace(image.URL)
		if isSafePublicURL(url) {
			urls = append(urls, url)
		}
	}
	return urls
}

func isSafePublicURL(url string) bool {
	if strings.ContainsAny(url, "\r\n\t") {
		return false
	}
	if strings.HasPrefix(url, "/") {
		return !strings.HasPrefix(url, "//") && !strings.Contains(url, "..")
	}
	return strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://")
}
