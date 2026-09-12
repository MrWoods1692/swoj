package app

import (
	"sync"
	"time"
)

// RateLimiter 内存级固定窗口限流，用于防暴力破解与打点。
type RateLimiter struct {
	mu     sync.Mutex
	key    string
	cap    int
	window time.Duration
	hits   map[string]*rateEntry
}

type rateEntry struct {
	windowStart time.Time
	count       int
}

// NewRateLimiter 创建限流器。cap 为窗口内允许的最大请求数。
func NewRateLimiter(key string, cap int, window time.Duration) *RateLimiter {
	return &RateLimiter{key: key, cap: cap, window: window, hits: map[string]*rateEntry{}}
}

// Allow 判断 caller 是否允许通过。
func (r *RateLimiter) Allow(caller string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	e := r.hits[r.key+caller]
	if e == nil || now.Sub(e.windowStart) >= r.window {
		r.hits[r.key+caller] = &rateEntry{windowStart: now, count: 1}
		return true
	}
	e.count++
	return e.count <= r.cap
}
