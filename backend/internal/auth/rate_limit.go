package auth

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/shuuuumai96/nikki-shelf/backend/internal/httpx"
)

type RateLimiter struct {
	mu              sync.Mutex
	ipAttempts      map[string]rateBucket
	accountAttempts map[string]rateBucket
	ipMaxAttempts   int
	accountMax      int
	window          time.Duration
	lockout         time.Duration
	maxEntries      int
	extractor       ClientIPExtractor
	now             func() time.Time
}

type RateLimiterConfig struct {
	IPAttempts      int
	AccountAttempts int
	Window          time.Duration
	Lockout         time.Duration
	MaxEntries      int
	Extractor       ClientIPExtractor
}

type rateBucket struct {
	Count   int
	ResetAt time.Time
	Lockout time.Time
}

func NewRateLimiter(config RateLimiterConfig) *RateLimiter {
	if config.IPAttempts <= 0 || config.AccountAttempts <= 0 || config.Window <= 0 {
		return nil
	}
	if config.Lockout <= 0 {
		config.Lockout = config.Window
	}
	if config.MaxEntries <= 0 {
		config.MaxEntries = 2048
	}

	return &RateLimiter{
		ipAttempts:      map[string]rateBucket{},
		accountAttempts: map[string]rateBucket{},
		ipMaxAttempts:   config.IPAttempts,
		accountMax:      config.AccountAttempts,
		window:          config.Window,
		lockout:         config.Lockout,
		maxEntries:      config.MaxEntries,
		extractor:       config.Extractor,
		now:             time.Now,
	}
}

func (r *RateLimiter) Allow(c echo.Context, username string) bool {
	if r == nil {
		return true
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.now()
	r.cleanup(now)
	return r.bucketAllows(r.ipAttempts, r.ipKey(c), r.ipMaxAttempts, now) &&
		r.bucketAllows(r.accountAttempts, accountKey(username), r.accountMax, now)
}

func (r *RateLimiter) RecordFailure(c echo.Context, username string) {
	if r == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.now()
	r.increment(r.ipAttempts, r.ipKey(c), r.ipMaxAttempts, now)
	r.increment(r.accountAttempts, accountKey(username), r.accountMax, now)
	r.enforceMaxSize()
}

func (r *RateLimiter) RecordSuccess(c echo.Context, username string) {
	if r == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.accountAttempts, accountKey(username))
	delete(r.ipAttempts, r.ipKey(c))
}

func (r *RateLimiter) bucketAllows(buckets map[string]rateBucket, key string, max int, now time.Time) bool {
	bucket := buckets[key]
	if expired(bucket, now) {
		return true
	}
	return bucket.Lockout.IsZero() || !now.Before(bucket.Lockout)
}

func (r *RateLimiter) increment(buckets map[string]rateBucket, key string, max int, now time.Time) {
	bucket := buckets[key]
	if expired(bucket, now) {
		bucket = rateBucket{ResetAt: now.Add(r.window)}
	}
	bucket.Count++
	if bucket.Count >= max {
		bucket.Lockout = now.Add(r.lockout)
	}
	buckets[key] = bucket
}

func (r *RateLimiter) cleanup(now time.Time) {
	for key, bucket := range r.ipAttempts {
		if expired(bucket, now) {
			delete(r.ipAttempts, key)
		}
	}
	for key, bucket := range r.accountAttempts {
		if expired(bucket, now) {
			delete(r.accountAttempts, key)
		}
	}
	r.enforceMaxSize()
}

func (r *RateLimiter) enforceMaxSize() {
	trimMap(r.ipAttempts, r.maxEntries)
	trimMap(r.accountAttempts, r.maxEntries)
}

func trimMap(items map[string]rateBucket, max int) {
	for len(items) > max {
		for key := range items {
			delete(items, key)
			break
		}
	}
}

func expired(bucket rateBucket, now time.Time) bool {
	return bucket.ResetAt.IsZero() || (!now.Before(bucket.ResetAt) && !now.Before(bucket.Lockout))
}

func (r *RateLimiter) ipKey(c echo.Context) string {
	return "ip:" + r.extractor.ClientIP(c.Request())
}

func accountKey(username string) string {
	key := strings.ToLower(strings.TrimSpace(username))
	if key == "" {
		key = "unknown"
	}
	return "account:" + key
}

func rateLimitError(c echo.Context) error {
	return httpx.ErrorWithKind(c, http.StatusTooManyRequests, "認証リクエストが多すぎます", "auth.rate_limited")
}
