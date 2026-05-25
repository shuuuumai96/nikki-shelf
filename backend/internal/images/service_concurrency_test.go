package images

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"mime/multipart"
	"strings"
	"sync"
	"testing"
)

func TestServiceConcurrentUploadsCannotExceedEntryLimit(t *testing.T) {
	state := newQuotaState()
	state.entries[10] = 42
	state.images = append(state.images,
		Row{ID: 1, EntryID: 10, FilePath: "/tmp/existing-1.jpg", PublicURL: "/uploads/existing-1.jpg", FileName: "existing-1.jpg", Size: 1, MimeType: "image/jpeg", CreatedAt: "2026-05-18T10:00:00Z"},
		Row{ID: 2, EntryID: 10, FilePath: "/tmp/existing-2.jpg", PublicURL: "/uploads/existing-2.jpg", FileName: "existing-2.jpg", Size: 1, MimeType: "image/jpeg", CreatedAt: "2026-05-18T10:00:00Z"},
	)
	state.nextID = 3

	database := openQuotaDB(t, state)
	storage := &recordingStorage{}
	service := NewService(NewRepository(database), storage, fakeEntryReader(true), ServiceConfig{
		Quota: QuotaConfig{UserBytes: 100, UserCount: 100},
	})

	headers := imageHeaders(t, 2)
	errs := make(chan error, 2)
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		header := headers[i]
		go func() {
			<-start
			_, err := service.SaveMany(context.Background(), 42, 10, []*multipart.FileHeader{header})
			errs <- err
		}()
	}
	close(start)

	first := <-errs
	second := <-errs
	if !((first == nil && errors.Is(second, ErrTooManyImages)) || (second == nil && errors.Is(first, ErrTooManyImages))) {
		t.Fatalf("errors = %v, %v; want one success and one too-many failure", first, second)
	}
	if got := state.countByEntry(10); got != MaxImagesPerEntry {
		t.Fatalf("image count = %d, want %d", got, MaxImagesPerEntry)
	}
	if len(storage.deleted) != 1 {
		t.Fatalf("deleted files = %#v, want one failed upload cleanup", storage.deleted)
	}
}

func imageHeaders(t *testing.T, count int) []*multipart.FileHeader {
	t.Helper()
	request := multiUploadRequest(t, "/api/entries/10/images", count, []byte("fake image bytes"))
	if err := request.ParseMultipartForm(12 << 20); err != nil {
		t.Fatalf("ParseMultipartForm: %v", err)
	}
	return request.MultipartForm.File["images"]
}

type quotaState struct {
	mu         sync.Mutex
	entryLocks map[int64]*sync.Mutex
	entries    map[int64]int64
	images     []Row
	nextID     int64
}

func newQuotaState() *quotaState {
	return &quotaState{
		entryLocks: map[int64]*sync.Mutex{},
		entries:    map[int64]int64{},
	}
}

func (s *quotaState) entryLock(entryID int64) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	lock, ok := s.entryLocks[entryID]
	if !ok {
		lock = &sync.Mutex{}
		s.entryLocks[entryID] = lock
	}
	return lock
}

func (s *quotaState) countByEntry(entryID int64) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, row := range s.images {
		if row.EntryID == entryID {
			count++
		}
	}
	return count
}

var (
	quotaDriverOnce sync.Once
	quotaDriverMu   sync.Mutex
	quotaDrivers    = map[string]*quotaState{}
)

func openQuotaDB(t *testing.T, state *quotaState) *sql.DB {
	t.Helper()
	quotaDriverOnce.Do(func() {
		sql.Register("images-quota-state", quotaDriver{})
	})

	name := t.Name()
	quotaDriverMu.Lock()
	quotaDrivers[name] = state
	quotaDriverMu.Unlock()
	t.Cleanup(func() {
		quotaDriverMu.Lock()
		delete(quotaDrivers, name)
		quotaDriverMu.Unlock()
	})

	database, err := sql.Open("images-quota-state", name)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	database.SetMaxOpenConns(2)
	return database
}

type quotaDriver struct{}

func (quotaDriver) Open(name string) (driver.Conn, error) {
	quotaDriverMu.Lock()
	state := quotaDrivers[name]
	quotaDriverMu.Unlock()
	if state == nil {
		return nil, errors.New("missing quota state")
	}
	return &quotaConn{state: state}, nil
}

type quotaConn struct {
	state *quotaState
	locks []*sync.Mutex
}

func (*quotaConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare is not supported")
}

func (*quotaConn) Close() error {
	return nil
}

func (c *quotaConn) Begin() (driver.Tx, error) {
	return quotaTx{conn: c}, nil
}

func (c *quotaConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	switch {
	case strings.Contains(query, "FOR UPDATE"):
		entryID := args[0].Value.(int64)
		userID := args[1].Value.(int64)
		lock := c.state.entryLock(entryID)
		lock.Lock()
		c.locks = append(c.locks, lock)

		c.state.mu.Lock()
		owner := c.state.entries[entryID]
		c.state.mu.Unlock()
		if owner != userID {
			c.unlockAll()
			return &quotaRows{columns: []string{"id"}}, nil
		}
		return &quotaRows{columns: []string{"id"}, rows: [][]driver.Value{{entryID}}}, nil
	case strings.Contains(query, "COUNT(*) FROM images WHERE entry_id"):
		entryID := args[0].Value.(int64)
		return &quotaRows{columns: []string{"count"}, rows: [][]driver.Value{{int64(c.state.countByEntry(entryID))}}}, nil
	case strings.Contains(query, "JOIN entries"):
		userID := args[0].Value.(int64)
		count, bytes := c.state.usageForUser(userID)
		return &quotaRows{columns: []string{"count", "bytes"}, rows: [][]driver.Value{{int64(count), bytes}}}, nil
	case strings.Contains(query, "COUNT(id), COALESCE"):
		count, bytes := c.state.totalUsage()
		return &quotaRows{columns: []string{"count", "bytes"}, rows: [][]driver.Value{{int64(count), bytes}}}, nil
	case strings.Contains(query, "INSERT INTO images"):
		row := c.state.insert(args)
		return &quotaRows{columns: imageColumns, rows: [][]driver.Value{{
			row.ID, row.EntryID, row.FilePath, row.PublicURL, row.FileName, row.Size, row.MimeType, row.CreatedAt,
		}}}, nil
	default:
		return nil, errors.New("unexpected query: " + query)
	}
}

func (s *quotaState) usageForUser(userID int64) (int, int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	bytes := int64(0)
	for _, row := range s.images {
		if s.entries[row.EntryID] == userID {
			count++
			bytes += row.Size
		}
	}
	return count, bytes
}

func (s *quotaState) totalUsage() (int, int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := len(s.images)
	bytes := int64(0)
	for _, row := range s.images {
		bytes += row.Size
	}
	return count, bytes
}

func (s *quotaState) insert(args []driver.NamedValue) Row {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	row := Row{
		ID:        s.nextID,
		EntryID:   args[0].Value.(int64),
		FilePath:  args[1].Value.(string),
		PublicURL: args[2].Value.(string),
		FileName:  args[3].Value.(string),
		Size:      args[4].Value.(int64),
		MimeType:  args[5].Value.(string),
		CreatedAt: args[6].Value.(string),
	}
	s.images = append(s.images, row)
	return row
}

func (c *quotaConn) unlockAll() {
	for i := len(c.locks) - 1; i >= 0; i-- {
		c.locks[i].Unlock()
	}
	c.locks = nil
}

type quotaTx struct {
	conn *quotaConn
}

func (tx quotaTx) Commit() error {
	tx.conn.unlockAll()
	return nil
}

func (tx quotaTx) Rollback() error {
	tx.conn.unlockAll()
	return nil
}

type quotaRows struct {
	columns []string
	rows    [][]driver.Value
	index   int
}

func (r *quotaRows) Columns() []string {
	return r.columns
}

func (r *quotaRows) Close() error {
	return nil
}

func (r *quotaRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}
