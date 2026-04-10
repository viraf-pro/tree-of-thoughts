package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	AllowLocalhost = true
	os.Exit(m.Run())
}

func TestValidateURL(t *testing.T) {
	// Temporarily disable localhost for this test
	AllowLocalhost = false
	defer func() { AllowLocalhost = true }()
	// Valid URLs
	if err := validateURL("https://example.com"); err != nil {
		t.Fatalf("https should be valid: %v", err)
	}
	if err := validateURL("http://example.com/path?q=1"); err != nil {
		t.Fatalf("http with path should be valid: %v", err)
	}

	// Invalid schemes
	if err := validateURL("ftp://example.com"); err == nil {
		t.Fatal("ftp should be rejected")
	}
	if err := validateURL("file:///etc/passwd"); err == nil {
		t.Fatal("file:// should be rejected")
	}

	// Local addresses blocked
	if err := validateURL("http://localhost/secret"); err == nil {
		t.Fatal("localhost should be blocked")
	}
	if err := validateURL("http://127.0.0.1/secret"); err == nil {
		t.Fatal("127.0.0.1 should be blocked")
	}
	if err := validateURL("http://0.0.0.0/"); err == nil {
		t.Fatal("0.0.0.0 should be blocked")
	}

	// Cloud metadata blocked
	if err := validateURL("http://169.254.169.254/latest/meta-data/"); err == nil {
		t.Fatal("metadata endpoint should be blocked")
	}
}

func TestStripHTML(t *testing.T) {
	tests := []struct {
		input    string
		contains string
		excludes string
	}{
		{
			input:    "<p>Hello <b>world</b></p>",
			contains: "Hello world",
			excludes: "<p>",
		},
		{
			input:    "<script>alert('xss')</script>visible text",
			contains: "visible text",
			excludes: "alert",
		},
		{
			input:    "<style>.foo{color:red}</style>visible",
			contains: "visible",
			excludes: "color",
		},
		{
			input:    "plain &amp; simple &lt;tag&gt;",
			contains: "plain & simple <tag>",
		},
	}
	for _, tt := range tests {
		result := stripHTML(tt.input)
		if tt.contains != "" && !strings.Contains(result, tt.contains) {
			t.Errorf("stripHTML(%q) should contain %q, got %q", tt.input, tt.contains, result)
		}
		if tt.excludes != "" && strings.Contains(result, tt.excludes) {
			t.Errorf("stripHTML(%q) should not contain %q, got %q", tt.input, tt.excludes, result)
		}
	}
}

func TestExtractTitle(t *testing.T) {
	html := `<html><head><title>My Page Title</title></head><body>content</body></html>`
	title := extractTitle(html)
	if title != "My Page Title" {
		t.Fatalf("title: got %q, want 'My Page Title'", title)
	}

	// No title
	noTitle := `<html><body>no title here</body></html>`
	if extractTitle(noTitle) != "" {
		t.Fatal("expected empty title")
	}
}

func TestCollapseWhitespace(t *testing.T) {
	input := "  hello   world  \n\n\n  foo  \n  bar  "
	result := collapseWhitespace(input)
	if strings.Contains(result, "  ") {
		t.Fatalf("should not have double spaces: %q", result)
	}
	if strings.Contains(result, "\n\n") {
		t.Fatalf("should not have double newlines: %q", result)
	}
}

func TestFetchFromTestServer(t *testing.T) {
	// Create a test HTTP server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head><title>Test Page</title></head><body><h1>Hello</h1><p>This is test content.</p></body></html>`))
	}))
	defer ts.Close()

	result, err := Fetch(ts.URL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if result.Title != "Test Page" {
		t.Fatalf("title: got %q", result.Title)
	}
	if !strings.Contains(result.Content, "Hello") {
		t.Fatalf("content should contain 'Hello', got: %s", result.Content)
	}
	if !strings.Contains(result.Content, "This is test content.") {
		t.Fatalf("content should contain 'This is test content.', got: %s", result.Content)
	}
	if result.BytesFetched == 0 {
		t.Fatal("bytesFetched should be > 0")
	}
}

func TestFetchPlainText(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("This is plain text content."))
	}))
	defer ts.Close()

	result, err := Fetch(ts.URL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if result.Content != "This is plain text content." {
		t.Fatalf("content: got %q", result.Content)
	}
}

func TestFetchNon200(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	_, err := Fetch(ts.URL)
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Fatalf("error should mention 403: %v", err)
	}
}

func TestFetchSizeLimit(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		// Write 2MB of data
		data := strings.Repeat("x", 2<<20)
		w.Write([]byte(data))
	}))
	defer ts.Close()

	result, err := Fetch(ts.URL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	// Should be capped at 1MB
	if result.BytesFetched > maxBodySize {
		t.Fatalf("bytesFetched %d exceeds max %d", result.BytesFetched, maxBodySize)
	}
}

func TestFetchInvalidScheme(t *testing.T) {
	_, err := Fetch("ftp://example.com")
	if err == nil {
		t.Fatal("expected error for ftp scheme")
	}
}
