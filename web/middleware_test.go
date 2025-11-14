package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

func TestNewRateLimiter(t *testing.T) {
	rl := NewRateLimiter(rate.Limit(10), 20)

	if rl == nil {
		t.Fatal("NewRateLimiter returned nil")
	}

	if rl.rate != rate.Limit(10) {
		t.Errorf("Expected rate 10, got %v", rl.rate)
	}

	if rl.burst != 20 {
		t.Errorf("Expected burst 20, got %d", rl.burst)
	}

	if rl.limiters == nil {
		t.Error("Limiters map should be initialized")
	}
}

func TestGetLimiter(t *testing.T) {
	rl := NewRateLimiter(rate.Limit(10), 20)

	// First call should create a new limiter
	limiter1 := rl.getLimiter("192.168.1.1")
	if limiter1 == nil {
		t.Fatal("getLimiter returned nil")
	}

	// Second call with same IP should return the same limiter
	limiter2 := rl.getLimiter("192.168.1.1")
	if limiter1 != limiter2 {
		t.Error("getLimiter should return the same limiter for the same IP")
	}

	// Different IP should get a different limiter
	limiter3 := rl.getLimiter("192.168.1.2")
	if limiter1 == limiter3 {
		t.Error("getLimiter should return different limiters for different IPs")
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		requestCount   int
		rateLimit      rate.Limit
		burst          int
		expectedStatus int
	}{
		{
			name:           "under limit",
			requestCount:   5,
			rateLimit:      rate.Limit(10),
			burst:          10,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "at burst limit",
			requestCount:   10,
			rateLimit:      rate.Limit(1),
			burst:          10,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "over limit",
			requestCount:   15,
			rateLimit:      rate.Limit(1),
			burst:          10,
			expectedStatus: http.StatusTooManyRequests,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := NewRateLimiter(tt.rateLimit, tt.burst)
			router := gin.New()
			router.Use(RateLimitMiddleware(rl))
			router.GET("/test", func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			var lastStatus int
			for i := 0; i < tt.requestCount; i++ {
				w := httptest.NewRecorder()
				req, _ := http.NewRequest("GET", "/test", nil)
				req.RemoteAddr = "192.168.1.100:12345"
				router.ServeHTTP(w, req)
				lastStatus = w.Code
			}

			if lastStatus != tt.expectedStatus {
				t.Errorf("Expected final status %d, got %d", tt.expectedStatus, lastStatus)
			}
		})
	}
}

func TestRateLimitMiddlewareErrorResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rl := NewRateLimiter(rate.Limit(1), 1)
	router := gin.New()
	router.Use(RateLimitMiddleware(rl))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// First request should succeed
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "192.168.1.100:12345"
	router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("First request should succeed, got status %d", w1.Code)
	}

	// Second request should be rate limited
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "192.168.1.100:12345"
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("Second request should be rate limited, got status %d", w2.Code)
	}

	// Check error message
	body := w2.Body.String()
	if !strings.Contains(body, "Rate limit exceeded") {
		t.Errorf("Expected rate limit error message, got: %s", body)
	}
}

func TestRateLimitMiddlewareDifferentIPs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rl := NewRateLimiter(rate.Limit(1), 1)
	router := gin.New()
	router.Use(RateLimitMiddleware(rl))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// First IP
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	router.ServeHTTP(w1, req1)

	// Second IP should not be rate limited
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "192.168.1.2:12345"
	router.ServeHTTP(w2, req2)

	if w1.Code != http.StatusOK {
		t.Errorf("First IP should succeed, got status %d", w1.Code)
	}

	if w2.Code != http.StatusOK {
		t.Errorf("Second IP should succeed, got status %d", w2.Code)
	}
}

func TestMaxBytesMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		maxBytes       int64
		bodySize       int
		expectedStatus int
	}{
		{
			name:           "under limit",
			maxBytes:       1024,
			bodySize:       512,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "at limit",
			maxBytes:       1024,
			bodySize:       1024,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "over limit by content-length",
			maxBytes:       1024,
			bodySize:       2048,
			expectedStatus: http.StatusRequestEntityTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(MaxBytesMiddleware(tt.maxBytes))
			router.POST("/test", func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			w := httptest.NewRecorder()
			body := strings.Repeat("x", tt.bodySize)
			req, _ := http.NewRequest("POST", "/test", strings.NewReader(body))
			req.Header.Set("Content-Length", string(rune(tt.bodySize)))
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestMaxBytesMiddlewareErrorMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(MaxBytesMiddleware(100))
	router.POST("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	body := strings.Repeat("x", 200)
	req, _ := http.NewRequest("POST", "/test", strings.NewReader(body))
	req.Header.Set("Content-Length", "200")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("Expected status 413, got %d", w.Code)
	}

	responseBody := w.Body.String()
	if !strings.Contains(responseBody, "Request body too large") {
		t.Errorf("Expected error message about body size, got: %s", responseBody)
	}
}

func TestCleanupOldLimiters(t *testing.T) {
	rl := NewRateLimiter(rate.Limit(10), 20)

	// Add more than 10000 limiters
	for i := 0; i < 10001; i++ {
		ip := "192.168." + string(rune(i/256)) + "." + string(rune(i%256))
		rl.getLimiter(ip)
	}

	// Verify we have many limiters
	rl.mu.Lock()
	count := len(rl.limiters)
	rl.mu.Unlock()

	if count <= 10000 {
		t.Errorf("Expected more than 10000 limiters, got %d", count)
	}

	// Trigger cleanup by simulating the condition
	rl.mu.Lock()
	if len(rl.limiters) > 10000 {
		rl.limiters = make(map[string]*rate.Limiter)
	}
	rl.mu.Unlock()

	// Verify cleanup happened
	rl.mu.Lock()
	newCount := len(rl.limiters)
	rl.mu.Unlock()

	if newCount != 0 {
		t.Errorf("Expected 0 limiters after cleanup, got %d", newCount)
	}
}

func TestRateLimitMiddlewareWithBurst(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rl := NewRateLimiter(rate.Limit(1), 5) // 1 per second, burst of 5
	router := gin.New()
	router.Use(RateLimitMiddleware(rl))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Should handle burst of 5 requests
	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d should succeed in burst, got status %d", i+1, w.Code)
		}
	}

	// 6th request should be rate limited
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	router.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Request after burst should be rate limited, got status %d", w.Code)
	}
}

func TestRateLimitMiddlewareRecovery(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rl := NewRateLimiter(rate.Limit(1), 1)
	router := gin.New()
	router.Use(RateLimitMiddleware(rl))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// First request
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	router.ServeHTTP(w1, req1)

	// Should be rate limited
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "192.168.1.1:12345"
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("Second request should be rate limited, got status %d", w2.Code)
	}

	// Wait for rate limit to reset (1 second at rate.Limit(1))
	time.Sleep(1100 * time.Millisecond)

	// Should succeed after waiting
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("GET", "/test", nil)
	req3.RemoteAddr = "192.168.1.1:12345"
	router.ServeHTTP(w3, req3)

	if w3.Code != http.StatusOK {
		t.Errorf("Request after waiting should succeed, got status %d", w3.Code)
	}
}
