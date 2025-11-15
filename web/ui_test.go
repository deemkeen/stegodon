package web

import (
	"fmt"
	"strconv"
	"testing"
	"time"
)

func TestFormatTimeAgo(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{
			name:     "just now",
			time:     now.Add(-30 * time.Second),
			expected: "just now",
		},
		{
			name:     "1 minute ago",
			time:     now.Add(-1 * time.Minute),
			expected: "1 minute ago",
		},
		{
			name:     "5 minutes ago",
			time:     now.Add(-5 * time.Minute),
			expected: "5 minutes ago",
		},
		{
			name:     "1 hour ago",
			time:     now.Add(-1 * time.Hour),
			expected: "1 hour ago",
		},
		{
			name:     "3 hours ago",
			time:     now.Add(-3 * time.Hour),
			expected: "3 hours ago",
		},
		{
			name:     "1 day ago",
			time:     now.Add(-24 * time.Hour),
			expected: "1 day ago",
		},
		{
			name:     "7 days ago",
			time:     now.Add(-7 * 24 * time.Hour),
			expected: "7 days ago",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTimeAgo(tt.time)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestFormatTimeAgoOldDates(t *testing.T) {
	// Test dates older than 30 days should return formatted date
	oldDate := time.Date(2024, time.January, 15, 10, 0, 0, 0, time.UTC)
	result := formatTimeAgo(oldDate)

	// Should return formatted date like "Jan 15, 2024"
	if result != "Jan 15, 2024" {
		t.Errorf("Expected formatted date, got '%s'", result)
	}
}

func TestFormatTimeAgoEdgeCases(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name string
		time time.Time
		want string
	}{
		{
			name: "exactly 1 minute",
			time: now.Add(-60 * time.Second),
			want: "1 minute ago",
		},
		{
			name: "exactly 1 hour",
			time: now.Add(-60 * time.Minute),
			want: "1 hour ago",
		},
		{
			name: "59 minutes",
			time: now.Add(-59 * time.Minute),
			want: "59 minutes ago",
		},
		{
			name: "23 hours",
			time: now.Add(-23 * time.Hour),
			want: "23 hours ago",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTimeAgo(tt.time)
			if result != tt.want {
				t.Errorf("Expected '%s', got '%s'", tt.want, result)
			}
		})
	}
}

func TestIndexPageDataStructure(t *testing.T) {
	// Test IndexPageData structure
	data := IndexPageData{
		Title:    "Home",
		Host:     "example.com",
		SSHPort:  23232,
		Posts:    []PostView{},
		HasPrev:  false,
		HasNext:  true,
		PrevPage: 0,
		NextPage: 2,
	}

	if data.Title != "Home" {
		t.Error("Title should be set")
	}
	if data.Host != "example.com" {
		t.Error("Host should be set")
	}
	if data.SSHPort != 23232 {
		t.Error("SSHPort should be set")
	}
	if data.HasPrev {
		t.Error("HasPrev should be false for first page")
	}
	if !data.HasNext {
		t.Error("HasNext should be true when there are more pages")
	}
}

func TestProfilePageDataStructure(t *testing.T) {
	// Test ProfilePageData structure
	data := ProfilePageData{
		Title:   "@alice",
		Host:    "example.com",
		SSHPort: 23232,
		User: UserView{
			Username:    "alice",
			DisplayName: "Alice Wonderland",
			Summary:     "Test bio",
			JoinedAgo:   "1 day ago",
		},
		Posts:      []PostView{},
		TotalPosts: 0,
		HasPrev:    false,
		HasNext:    false,
		PrevPage:   0,
		NextPage:   2,
	}

	if data.Title != "@alice" {
		t.Error("Title should include @ prefix")
	}
	if data.User.Username != "alice" {
		t.Error("User username should be set")
	}
	if data.User.DisplayName != "Alice Wonderland" {
		t.Error("User display name should be set")
	}
}

func TestUserViewStructure(t *testing.T) {
	// Test UserView structure
	user := UserView{
		Username:    "bob",
		DisplayName: "Bob Builder",
		Summary:     "Can we fix it? Yes we can!",
		JoinedAgo:   "3 months ago",
	}

	if user.Username != "bob" {
		t.Error("Username should be set")
	}
	if user.DisplayName != "Bob Builder" {
		t.Error("DisplayName should be set")
	}
	if user.Summary != "Can we fix it? Yes we can!" {
		t.Error("Summary should be set")
	}
	if user.JoinedAgo != "3 months ago" {
		t.Error("JoinedAgo should be set")
	}
}

func TestPostViewStructure(t *testing.T) {
	// Test PostView structure
	post := PostView{
		Username: "charlie",
		Message:  "Hello, world!",
		TimeAgo:  "5 minutes ago",
	}

	if post.Username != "charlie" {
		t.Error("Username should be set")
	}
	if post.Message != "Hello, world!" {
		t.Error("Message should be set")
	}
	if post.TimeAgo != "5 minutes ago" {
		t.Error("TimeAgo should be set")
	}
}

func TestPaginationLogic(t *testing.T) {
	// Test pagination calculations
	tests := []struct {
		name         string
		page         int
		postsPerPage int
		totalPosts   int
		wantStart    int
		wantEnd      int
		wantHasPrev  bool
		wantHasNext  bool
	}{
		{
			name:         "first page with more posts",
			page:         1,
			postsPerPage: 20,
			totalPosts:   50,
			wantStart:    0,
			wantEnd:      20,
			wantHasPrev:  false,
			wantHasNext:  true,
		},
		{
			name:         "second page",
			page:         2,
			postsPerPage: 20,
			totalPosts:   50,
			wantStart:    20,
			wantEnd:      40,
			wantHasPrev:  true,
			wantHasNext:  true,
		},
		{
			name:         "last page",
			page:         3,
			postsPerPage: 20,
			totalPosts:   50,
			wantStart:    40,
			wantEnd:      50,
			wantHasPrev:  true,
			wantHasNext:  false,
		},
		{
			name:         "page beyond total",
			page:         10,
			postsPerPage: 20,
			totalPosts:   50,
			wantStart:    50,
			wantEnd:      50,
			wantHasPrev:  true,
			wantHasNext:  false,
		},
		{
			name:         "single page",
			page:         1,
			postsPerPage: 20,
			totalPosts:   10,
			wantStart:    0,
			wantEnd:      10,
			wantHasPrev:  false,
			wantHasNext:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			offset := (tt.page - 1) * tt.postsPerPage

			start := offset
			end := offset + tt.postsPerPage
			if start > tt.totalPosts {
				start = tt.totalPosts
			}
			if end > tt.totalPosts {
				end = tt.totalPosts
			}

			hasPrev := tt.page > 1
			hasNext := end < tt.totalPosts

			if start != tt.wantStart {
				t.Errorf("Expected start %d, got %d", tt.wantStart, start)
			}
			if end != tt.wantEnd {
				t.Errorf("Expected end %d, got %d", tt.wantEnd, end)
			}
			if hasPrev != tt.wantHasPrev {
				t.Errorf("Expected hasPrev %v, got %v", tt.wantHasPrev, hasPrev)
			}
			if hasNext != tt.wantHasNext {
				t.Errorf("Expected hasNext %v, got %v", tt.wantHasNext, hasNext)
			}
		})
	}
}

func TestPageNumberParsing(t *testing.T) {
	// Test page number parsing logic
	tests := []struct {
		name     string
		pageStr  string
		wantPage int
	}{
		{
			name:     "valid page 1",
			pageStr:  "1",
			wantPage: 1,
		},
		{
			name:     "valid page 5",
			pageStr:  "5",
			wantPage: 5,
		},
		{
			name:     "empty string defaults to 1",
			pageStr:  "",
			wantPage: 1,
		},
		{
			name:     "invalid string defaults to 1",
			pageStr:  "abc",
			wantPage: 1,
		},
		{
			name:     "zero defaults to 1",
			pageStr:  "0",
			wantPage: 1,
		},
		{
			name:     "negative defaults to 1",
			pageStr:  "-5",
			wantPage: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page := 1
			if tt.pageStr != "" {
				if p, err := strconv.Atoi(tt.pageStr); err == nil && p > 0 {
					page = p
				}
			}

			if page != tt.wantPage {
				t.Errorf("Expected page %d, got %d", tt.wantPage, page)
			}
		})
	}
}

func TestHostSelection(t *testing.T) {
	// Test logic for choosing between Host and SslDomain
	tests := []struct {
		name      string
		host      string
		sslDomain string
		withAp    bool
		wantHost  string
	}{
		{
			name:      "ActivityPub enabled uses SslDomain",
			host:      "127.0.0.1",
			sslDomain: "example.com",
			withAp:    true,
			wantHost:  "example.com",
		},
		{
			name:      "ActivityPub disabled uses Host",
			host:      "127.0.0.1",
			sslDomain: "example.com",
			withAp:    false,
			wantHost:  "127.0.0.1",
		},
		{
			name:      "No ActivityPub uses Host",
			host:      "localhost",
			sslDomain: "",
			withAp:    false,
			wantHost:  "localhost",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := tt.host
			if tt.withAp {
				host = tt.sslDomain
			}

			if host != tt.wantHost {
				t.Errorf("Expected host '%s', got '%s'", tt.wantHost, host)
			}
		})
	}
}

func TestPaginationPageNumbers(t *testing.T) {
	// Test prev/next page number calculations
	tests := []struct {
		name     string
		page     int
		wantPrev int
		wantNext int
	}{
		{
			name:     "page 1",
			page:     1,
			wantPrev: 0,
			wantNext: 2,
		},
		{
			name:     "page 5",
			page:     5,
			wantPrev: 4,
			wantNext: 6,
		},
		{
			name:     "page 10",
			page:     10,
			wantPrev: 9,
			wantNext: 11,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prevPage := tt.page - 1
			nextPage := tt.page + 1

			if prevPage != tt.wantPrev {
				t.Errorf("Expected prevPage %d, got %d", tt.wantPrev, prevPage)
			}
			if nextPage != tt.wantNext {
				t.Errorf("Expected nextPage %d, got %d", tt.wantNext, nextPage)
			}
		})
	}
}

func TestProfileTitleFormat(t *testing.T) {
	// Test profile page title formatting
	tests := []struct {
		username  string
		wantTitle string
	}{
		{username: "alice", wantTitle: "@alice"},
		{username: "bob", wantTitle: "@bob"},
		{username: "user_123", wantTitle: "@user_123"},
	}

	for _, tt := range tests {
		t.Run(tt.username, func(t *testing.T) {
			title := fmt.Sprintf("@%s", tt.username)
			if title != tt.wantTitle {
				t.Errorf("Expected title '%s', got '%s'", tt.wantTitle, title)
			}
		})
	}
}

func TestTimeAgoPluralization(t *testing.T) {
	// Test that pluralization works correctly
	now := time.Now()

	singularTests := []struct {
		name string
		time time.Time
		want string
	}{
		{"1 minute", now.Add(-1 * time.Minute), "1 minute ago"},
		{"1 hour", now.Add(-1 * time.Hour), "1 hour ago"},
		{"1 day", now.Add(-24 * time.Hour), "1 day ago"},
	}

	for _, tt := range singularTests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTimeAgo(tt.time)
			if result != tt.want {
				t.Errorf("Expected '%s', got '%s'", tt.want, result)
			}
		})
	}

	pluralTests := []struct {
		name string
		time time.Time
		want string
	}{
		{"2 minutes", now.Add(-2 * time.Minute), "2 minutes ago"},
		{"2 hours", now.Add(-2 * time.Hour), "2 hours ago"},
		{"2 days", now.Add(-48 * time.Hour), "2 days ago"},
	}

	for _, tt := range pluralTests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTimeAgo(tt.time)
			if result != tt.want {
				t.Errorf("Expected '%s', got '%s'", tt.want, result)
			}
		})
	}
}
