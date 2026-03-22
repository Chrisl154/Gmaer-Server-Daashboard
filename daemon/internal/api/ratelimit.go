package api

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// ipRateLimiter holds per-IP token buckets and evicts stale entries periodically.
type ipRateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rateLimiterEntry
	r        rate.Limit
	burst    int
}

type rateLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func newIPRateLimiter(r rate.Limit, burst int) *ipRateLimiter {
	rl := &ipRateLimiter{
		limiters: make(map[string]*rateLimiterEntry),
		r:        r,
		burst:    burst,
	}
	go rl.evictLoop()
	return rl
}

func (rl *ipRateLimiter) get(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	e, ok := rl.limiters[ip]
	if !ok {
		e = &rateLimiterEntry{limiter: rate.NewLimiter(rl.r, rl.burst)}
		rl.limiters[ip] = e
	}
	e.lastSeen = time.Now()
	return e.limiter
}

// evictLoop removes entries that haven't been seen in 10 minutes.
func (rl *ipRateLimiter) evictLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		for ip, e := range rl.limiters {
			if time.Since(e.lastSeen) > 10*time.Minute {
				delete(rl.limiters, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// rateLimitMiddleware returns a Gin middleware that allows at most `burst`
// requests per IP before enforcing `r` requests/second. Returns 429 when
// the bucket is empty.
func rateLimitMiddleware(r rate.Limit, burst int) gin.HandlerFunc {
	rl := newIPRateLimiter(r, burst)
	return func(c *gin.Context) {
		if !rl.get(c.ClientIP()).Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many requests — please wait before trying again"})
			c.Abort()
			return
		}
		c.Next()
	}
}
