package controllers

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type rateEntry struct {
	count   int
	resetAt time.Time
}

const maxRateLimitEntries = 8192

// RateLimit memberi perlindungan brute-force dasar untuk satu instance.
// Deployment multi-instance sebaiknya menggantinya dengan limiter bersama (mis. Redis).
func RateLimit(limit int, window time.Duration) gin.HandlerFunc {
	var mu sync.Mutex
	entries := make(map[string]rateEntry)
	return func(c *gin.Context) {
		now := time.Now().UTC()
		key := c.ClientIP()
		mu.Lock()
		// Bersihkan entry kedaluwarsa secara bertahap agar map tidak tumbuh tanpa
		// batas pada instance yang menerima banyak IP unik.
		if len(entries) > 1024 {
			for storedKey, storedEntry := range entries {
				if now.After(storedEntry.resetAt) {
					delete(entries, storedKey)
				}
			}
		}
		entry, exists := entries[key]
		if !exists && len(entries) >= maxRateLimitEntries {
			mu.Unlock()
			c.Header("Retry-After", fmtInt(max(int(window.Seconds()), 1)))
			respondError(c, http.StatusTooManyRequests, "rate_limited", "Terlalu banyak sumber request aktif. Coba kembali beberapa saat lagi.")
			c.Abort()
			return
		}
		if !exists || now.After(entry.resetAt) {
			entry = rateEntry{resetAt: now.Add(window)}
		}
		entry.count++
		entries[key] = entry
		remaining := limit - entry.count
		resetAt := entry.resetAt
		mu.Unlock()

		c.Header("X-RateLimit-Limit", fmtInt(limit))
		c.Header("X-RateLimit-Remaining", fmtInt(max(remaining, 0)))
		if entry.count > limit {
			c.Header("Retry-After", fmtInt(max(int(time.Until(resetAt).Seconds()), 1)))
			respondError(c, http.StatusTooManyRequests, "rate_limited", "Terlalu banyak percobaan. Coba kembali beberapa saat lagi.")
			c.Abort()
			return
		}
		c.Next()
	}
}

func fmtInt(value int) string {
	if value == 0 {
		return "0"
	}
	digits := make([]byte, 0, 10)
	for value > 0 {
		digits = append(digits, byte('0'+value%10))
		value /= 10
	}
	for left, right := 0, len(digits)-1; left < right; left, right = left+1, right-1 {
		digits[left], digits[right] = digits[right], digits[left]
	}
	return string(digits)
}
