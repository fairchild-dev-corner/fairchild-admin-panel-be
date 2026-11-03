package middlewares

import (
	"net/http"
	"sync"
	"time"

	loggers "fairchild-admin-panel-be/internal/utils/loggers"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

const (
	cleanupInterval = 5 * time.Minute
	idleTimeout     = 10 * time.Minute
)

// IPRateLimiter enforces a token-bucket limit per client IP. It is
// mechanism-only - the caller decides the rate/burst policy.
type IPRateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rate     rate.Limit
	burst    int
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewIPRateLimiter starts a background cleanup goroutine to evict visitors
// that have been idle past idleTimeout, so the map doesn't grow unbounded.
func NewIPRateLimiter(r rate.Limit, burst int) *IPRateLimiter {
	i := &IPRateLimiter{
		visitors: make(map[string]*visitor),
		rate:     r,
		burst:    burst,
	}

	go i.cleanupVisitors()

	return i
}

func (i *IPRateLimiter) getLimiter(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	v, exists := i.visitors[ip]
	if !exists {
		v = &visitor{limiter: rate.NewLimiter(i.rate, i.burst)}
		i.visitors[ip] = v
	}
	v.lastSeen = time.Now()

	return v.limiter
}

func (i *IPRateLimiter) cleanupVisitors() {
	for range time.Tick(cleanupInterval) {
		i.mu.Lock()
		for ip, v := range i.visitors {
			if time.Since(v.lastSeen) > idleTimeout {
				delete(i.visitors, ip)
			}
		}
		i.mu.Unlock()
	}
}

// Middleware rejects a request with 429 once the requesting IP has
// exhausted its token bucket.
func (i *IPRateLimiter) Middleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if !i.getLimiter(ctx.ClientIP()).Allow() {
			loggers.GetCommonError(ctx, "too many requests, please try again later", http.StatusTooManyRequests)
			ctx.Abort()
			return
		}

		ctx.Next()
	}
}
