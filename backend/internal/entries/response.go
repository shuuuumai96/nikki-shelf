package entries

import "github.com/shuuuumai96/nikki-shelf/backend/internal/images"

func entryResponse(row EntryRow, imageRows []images.Row) EntryResponse {
	return EntryResponse{
		ID:        row.ID,
		EntryDate: row.EntryDate,
		Title:     row.Title,
		Body:      row.Body,
		Mood:      row.Mood,
		Tags:      decodeTags(row.TagsJSON),
		Images:    entryImages(imageRows),
		Version:   row.Version,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}

func entryImages(rows []images.Row) []EntryImage {
	result := make([]EntryImage, 0, len(rows))
	for _, row := range rows {
		result = append(result, EntryImage{
			ID:        row.ID,
			EntryID:   row.EntryID,
			URL:       images.ContentURL(row.ID),
			FileName:  row.FileName,
			Size:      row.Size,
			MimeType:  row.MimeType,
			CreatedAt: row.CreatedAt,
		})
	}
	return result
}
