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

	// Add limiters with old timestamps (simulating inactive IPs)
	rl.mu.Lock()
	oldTime := time.Now().Add(-15 * time.Minute)
	for i := range 100 {
		ip := "10.0." + string(rune(i/256+1)) + "." + string(rune(i%256+1))
		rl.limiters[ip] = &limiterEntry{
			limiter:  rate.NewLimiter(rl.rate, rl.burst),
			lastSeen: oldTime,
		}
	}
	rl.mu.Unlock()

	// Add a recent limiter that should survive cleanup
	recentLimiter := rl.getLimiter("192.168.1.1")

	// Verify we have 101 limiters (100 old + 1 recent)
	rl.mu.Lock()
	count := len(rl.limiters)
	rl.mu.Unlock()

	if count != 101 {
		t.Errorf("Expected 101 limiters, got %d", count)
	}

	// Simulate cleanup: evict entries older than 10 minutes
	rl.mu.Lock()
	cutoff := time.Now().Add(-10 * time.Minute)
	for ip, entry := range rl.limiters {
		if entry.lastSeen.Before(cutoff) {
			delete(rl.limiters, ip)
		}
	}
	rl.mu.Unlock()

	// Verify only the recent limiter survived
	rl.mu.Lock()
	newCount := len(rl.limiters)
	rl.mu.Unlock()

	if newCount != 1 {
		t.Errorf("Expected 1 limiter after cleanup, got %d", newCount)
	}

	// Verify the surviving limiter is the recent one
	survivingLimiter := rl.getLimiter("192.168.1.1")
	if survivingLimiter != recentLimiter {
		t.Error("Recent limiter should survive cleanup")
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
	for i := range 5 {
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

func TestIsHTMLRequest(t *testing.T) {
	tests := []struct {
		name     string
		accept   string
		expected bool
	}{
		// Browser requests (should return true - redirect to /u/)
		{
			name:     "empty accept header",
			accept:   "",
			expected: true,
		},
		{
			name:     "wildcard accept",
			accept:   "*/*",
			expected: true,
		},
		{
			name:     "text/html",
			accept:   "text/html",
			expected: true,
		},
		{
			name:     "browser typical header",
			accept:   "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
			expected: true,
		},
		{
			name:     "text/html with charset",
			accept:   "text/html; charset=utf-8",
			expected: true,
		},

		// ActivityPub requests (should return false - serve JSON)
		{
			name:     "application/activity+json",
			accept:   "application/activity+json",
			expected: false,
		},
		{
			name:     "application/ld+json",
			accept:   "application/ld+json",
			expected: false,
		},
		{
			name:     "application/ld+json with profile",
			accept:   `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`,
			expected: false,
		},
		{
			name:     "application/json",
			accept:   "application/json",
			expected: false,
		},
		{
			name:     "Mastodon typical header",
			accept:   "application/activity+json, application/ld+json",
			expected: false,
		},
		{
			name:     "mixed with activity+json priority",
			accept:   "application/activity+json, text/html;q=0.9",
			expected: false,
		},

		// Edge cases
		{
			name:     "unknown content type defaults to HTML",
			accept:   "application/xml",
			expected: true,
		},
		{
			name:     "image type defaults to HTML",
			accept:   "image/png",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsHTMLRequest(tt.accept)
			if result != tt.expected {
				t.Errorf("IsHTMLRequest(%q) = %v, expected %v", tt.accept, result, tt.expected)
			}
		})
	}
}

// TestUsersActorContentNegotiation tests that /users/:actor redirects browsers to /u/:actor
// but serves JSON to ActivityPub clients. This fixes Lemmy federation issue #36.
func TestUsersActorContentNegotiation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		accept         string
		expectedStatus int
		expectRedirect bool
		redirectTarget string
	}{
		// Browser requests should redirect to /u/
		{
			name:           "browser with text/html",
			accept:         "text/html",
			expectedStatus: http.StatusFound, // 302
			expectRedirect: true,
			redirectTarget: "/u/testuser",
		},
		{
			name:           "browser with typical accept header",
			accept:         "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
			expectedStatus: http.StatusFound,
			expectRedirect: true,
			redirectTarget: "/u/testuser",
		},
		{
			name:           "browser with empty accept",
			accept:         "",
			expectedStatus: http.StatusFound,
			expectRedirect: true,
			redirectTarget: "/u/testuser",
		},
		{
			name:           "browser with wildcard",
			accept:         "*/*",
			expectedStatus: http.StatusFound,
			expectRedirect: true,
			redirectTarget: "/u/testuser",
		},
		// ActivityPub requests should get JSON (mocked as OK here)
		{
			name:           "ActivityPub with activity+json",
			accept:         "application/activity+json",
			expectedStatus: http.StatusOK,
			expectRedirect: false,
		},
		{
			name:           "ActivityPub with ld+json",
			accept:         "application/ld+json",
			expectedStatus: http.StatusOK,
			expectRedirect: false,
		},
		{
			name:           "ActivityPub with json",
			accept:         "application/json",
			expectedStatus: http.StatusOK,
			expectRedirect: false,
		},
		{
			name:           "Mastodon typical header",
			accept:         "application/activity+json, application/ld+json",
			expectedStatus: http.StatusOK,
			expectRedirect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()

			// Mock the /users/:actor endpoint with content negotiation
			router.GET("/users/:actor", func(c *gin.Context) {
				actorName := c.Param("actor")
				accept := c.GetHeader("Accept")

				if IsHTMLRequest(accept) {
					c.Redirect(302, "/u/"+actorName)
					return
				}

				// Mock ActivityPub JSON response
				c.Header("Content-Type", "application/activity+json")
				c.JSON(http.StatusOK, gin.H{"type": "Person", "name": actorName})
			})

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/users/testuser", nil)
			if tt.accept != "" {
				req.Header.Set("Accept", tt.accept)
			}
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectRedirect {
				location := w.Header().Get("Location")
				if location != tt.redirectTarget {
					t.Errorf("Expected redirect to %s, got %s", tt.redirectTarget, location)
				}
			}
		})
	}
}

// TestLemmyFederationFix specifically tests the Lemmy federation issue from GitHub #36
// Lemmy uses the 'id' field (/users/username) instead of 'url' field (/u/username)
func TestLemmyFederationFix(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/users/:actor", func(c *gin.Context) {
		actorName := c.Param("actor")
		accept := c.GetHeader("Accept")

		if IsHTMLRequest(accept) {
			c.Redirect(302, "/u/"+actorName)
			return
		}

		c.Header("Content-Type", "application/activity+json")
		c.JSON(http.StatusOK, gin.H{"type": "Person"})
	})

	// Simulate a Lemmy user clicking a federated profile link
	// Lemmy sends browser to /users/username with typical browser Accept header
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/users/demigodrick", nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	router.ServeHTTP(w, req)

	// Should redirect to the human-readable profile page
	if w.Code != http.StatusFound {
		t.Errorf("Expected 302 redirect, got %d", w.Code)
	}

	location := w.Header().Get("Location")
	if location != "/u/demigodrick" {
		t.Errorf("Expected redirect to /u/demigodrick, got %s", location)
	}
}
