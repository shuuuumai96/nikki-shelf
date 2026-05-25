package images

import "strconv"

type Row struct {
	ID        int64
	EntryID   int64
	FilePath  string
	PublicURL string
	FileName  string
	Size      int64
	MimeType  string
	CreatedAt string
}

type Response struct {
	ID        int64  `json:"id"`
	EntryID   int64  `json:"entryId"`
	URL       string `json:"url"`
	FileName  string `json:"fileName"`
	Size      int64  `json:"size"`
	MimeType  string `json:"mimeType"`
	CreatedAt string `json:"createdAt"`
}

func ContentURL(id int64) string {
	return "/api/images/" + strconv.FormatInt(id, 10) + "/content"
}

type StoredFile struct {
	FilePath  string
	PublicURL string
	FileName  string
	Size      int64
	MimeType  string
}

type Usage struct {
	Count int
	Bytes int64
}
