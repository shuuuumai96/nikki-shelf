package entries

import (
	"context"
	"errors"
	"time"

	"github.com/shuuuumai96/nikki-shelf/backend/internal/images"
)

type ImageReader interface {
	ListByEntryID(ctx context.Context, entryID int64) ([]images.Row, error)
}

type ImageFileDeleter interface {
	DeleteFiles(ctx context.Context, rows []images.Row)
}

type Service struct {
	repo       *Repository
	imageRepo  ImageReader
	imageFiles ImageFileDeleter
	now        func() time.Time
	todayLocal func() string
}

func NewService(repo *Repository, imageRepo ImageReader, imageFiles ...ImageFileDeleter) *Service {
	var deleter ImageFileDeleter
	if len(imageFiles) > 0 {
		deleter = imageFiles[0]
	}
	return &Service{
		repo:       repo,
		imageRepo:  imageRepo,
		imageFiles: deleter,
		now:        time.Now,
		todayLocal: func() string {
			return time.Now().Format(time.DateOnly)
		},
	}
}

func (s *Service) Create(ctx context.Context, userID int64, input CreateInput) (EntryResponse, error) {
	normalized, err := normalizeCreateInput(input, s.todayLocal)
	if err != nil {
		return EntryResponse{}, err
	}
	normalized.UserID = userID

	row, err := s.repo.Create(ctx, normalized, s.now())
	if err != nil {
		return EntryResponse{}, err
	}

	return s.response(ctx, row)
}

func (s *Service) Update(ctx context.Context, userID int64, id int64, input UpdateInput) (EntryResponse, error) {
	normalized, err := normalizeUpdateInput(input, s.todayLocal)
	if err != nil {
		return EntryResponse{}, err
	}

	row, err := s.repo.Update(ctx, userID, id, normalized, s.now())
	if err != nil {
		return EntryResponse{}, err
	}

	return s.response(ctx, row)
}

func (s *Service) GetByID(ctx context.Context, userID int64, id int64) (EntryResponse, error) {
	row, err := s.repo.GetByID(ctx, userID, id)
	if err != nil {
		return EntryResponse{}, err
	}

	return s.response(ctx, row)
}

func (s *Service) GetByDate(ctx context.Context, userID int64, date string) (EntryResponse, error) {
	if !isDate(date) {
		return EntryResponse{}, ErrInvalidInput
	}

	row, err := s.repo.GetByDate(ctx, userID, date)
	if err != nil {
		return EntryResponse{}, err
	}

	return s.response(ctx, row)
}

func (s *Service) List(ctx context.Context, userID int64, filter EntryFilter) ([]EntryResponse, error) {
	rows, err := s.repo.List(ctx, userID, filter)
	if err != nil {
		return nil, err
	}

	responses := make([]EntryResponse, 0, len(rows))
	for _, row := range rows {
		response, err := s.response(ctx, row)
		if err != nil {
			return nil, err
		}
		responses = append(responses, response)
	}

	return responses, nil
}

func (s *Service) ListPage(ctx context.Context, userID int64, request EntryPageRequest) (EntryPageResponse, error) {
	if request.PerPage <= 0 || request.PerPage > MaxEntriesPerPage {
		return EntryPageResponse{}, ErrInvalidInput
	}

	page, err := s.repo.ListPage(ctx, userID, request)
	if err != nil {
		return EntryPageResponse{}, err
	}

	responses := make([]EntryResponse, 0, len(page.Rows))
	for _, row := range page.Rows {
		response, err := s.response(ctx, row)
		if err != nil {
			return EntryPageResponse{}, err
		}
		responses = append(responses, response)
	}

	return EntryPageResponse{
		Items:      responses,
		NextCursor: page.NextCursor,
		HasMore:    page.HasMore,
	}, nil
}

func (s *Service) Count(ctx context.Context, userID int64, filter EntryFilter) (int, error) {
	return s.repo.Count(ctx, userID, filter)
}

func (s *Service) ListForExport(ctx context.Context, userID int64) ([]EntryResponse, error) {
	return s.List(ctx, userID, EntryFilter{})
}

func (s *Service) Search(ctx context.Context, userID int64, filter SearchFilter) (SearchResponse, error) {
	normalized, err := normalizeSearchFilter(filter)
	if err != nil {
		return SearchResponse{}, err
	}
	if !searchHasActiveFilter(normalized) {
		return SearchResponse{Results: []SearchResult{}}, nil
	}

	rows, err := s.repo.Search(ctx, userID, normalized)
	if err != nil {
		return SearchResponse{}, err
	}

	results := make([]SearchResult, 0, len(rows))
	for _, row := range rows {
		results = append(results, searchResult(row, normalized.Query))
	}

	return SearchResponse{Results: results}, nil
}

func (s *Service) Tags(ctx context.Context, userID int64) ([]string, error) {
	return s.repo.Tags(ctx, userID)
}

func (s *Service) Delete(ctx context.Context, userID int64, id int64) error {
	imageRows, err := s.imageRepo.ListByEntryID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, userID, id); err != nil {
		return err
	}
	if s.imageFiles != nil {
		s.imageFiles.DeleteFiles(ctx, imageRows)
	}
	return nil
}

func (s *Service) response(ctx context.Context, row EntryRow) (EntryResponse, error) {
	imageRows, err := s.imageRepo.ListByEntryID(ctx, row.ID)
	if err != nil {
		return EntryResponse{}, err
	}

	return entryResponse(row, imageRows), nil
}

var errorSpecs = []struct {
	target error
	status int
	kind   string
}{
	{ErrInvalidCursor, 400, "entries.invalid_cursor"},
	{ErrInvalidInput, 400, "entries.invalid_input"},
	{ErrNotFound, 404, "entries.not_found"},
	{ErrDateExists, 409, "entries.date_exists"},
	{ErrStaleVersion, 409, "entries.stale_version"},
}

func StatusFor(err error) int {
	for _, item := range errorSpecs {
		if errors.Is(err, item.target) {
			return item.status
		}
	}

	return 500
}

func KindFor(err error) string {
	for _, item := range errorSpecs {
		if errors.Is(err, item.target) {
			return item.kind
		}
	}

	return "server.internal"
}
