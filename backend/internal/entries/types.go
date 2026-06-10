package entries

type EntryImage struct {
	ID        int64  `json:"id"`
	EntryID   int64  `json:"entryId"`
	URL       string `json:"url"`
	FileName  string `json:"fileName"`
	Size      int64  `json:"size"`
	MimeType  string `json:"mimeType"`
	CreatedAt string `json:"createdAt"`
}

type EntryResponse struct {
	ID        int64        `json:"id"`
	EntryDate string       `json:"entryDate"`
	Title     string       `json:"title"`
	Body      string       `json:"body"`
	Mood      string       `json:"mood"`
	Tags      []string     `json:"tags"`
	Images    []EntryImage `json:"images"`
	Version   int64        `json:"version"`
	CreatedAt string       `json:"createdAt"`
	UpdatedAt string       `json:"updatedAt"`
}

type EntryDateLookupResponse struct {
	Entry  *EntryResponse `json:"entry"`
	Date   string         `json:"date"`
	Exists bool           `json:"exists"`
}

type EntryPageResponse struct {
	Items      []EntryResponse `json:"items"`
	NextCursor string          `json:"nextCursor"`
	HasMore    bool            `json:"hasMore"`
}

type CreateInput struct {
	UserID    int64    `json:"-"`
	EntryDate string   `json:"entryDate"`
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	Mood      string   `json:"mood"`
	Tags      []string `json:"tags"`
}

type UpdateInput struct {
	EntryDate string   `json:"entryDate"`
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	Mood      string   `json:"mood"`
	Tags      []string `json:"tags"`
	Version   int64    `json:"expectedVersion"`
}

type EntryFilter struct {
	Query string
	Tag   string
	Mood  string
	From  string
	To    string
}

type EntryPageRequest struct {
	Filter  EntryFilter
	PerPage int
	Cursor  string
}

type EntryPage struct {
	Rows       []EntryRow
	NextCursor string
	HasMore    bool
}

type SearchFilter struct {
	Query    string
	From     string
	To       string
	Mood     string
	Tag      string
	HasImage string
	Limit    int
	Offset   int
}

type SearchResponse struct {
	Results []SearchResult `json:"results"`
}

type MemoryFilter struct {
	Date         string
	ExcludeMoods []string
	Limit        int
}

type MemoryResponse struct {
	Items []MemoryItem `json:"items"`
}

type MemoryItem struct {
	ID         int64    `json:"id"`
	EntryDate  string   `json:"entryDate"`
	Title      string   `json:"title"`
	Preview    string   `json:"preview"`
	Mood       string   `json:"mood"`
	Tags       []string `json:"tags"`
	HasImage   bool     `json:"hasImage"`
	ImageCount int      `json:"imageCount"`
	UpdatedAt  string   `json:"updatedAt"`
}

type SearchResult struct {
	ID         int64    `json:"id"`
	EntryDate  string   `json:"entryDate"`
	Title      string   `json:"title"`
	Preview    string   `json:"preview"`
	Mood       string   `json:"mood"`
	Tags       []string `json:"tags"`
	HasImage   bool     `json:"hasImage"`
	ImageCount int      `json:"imageCount"`
	UpdatedAt  string   `json:"updatedAt"`
}

type EntryRow struct {
	ID        int64
	UserID    int64
	EntryDate string
	Title     string
	Body      string
	Mood      string
	TagsJSON  string
	Version   int64
	CreatedAt string
	UpdatedAt string
}
