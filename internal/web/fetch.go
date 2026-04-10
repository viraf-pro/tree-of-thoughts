package web

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	maxBodySize    = 1 << 20 // 1 MB
	requestTimeout = 30 * time.Second
	userAgent      = "tot-mcp/1.0 (knowledge-ingest)"
)

// AllowLocalhost is set to true in tests to allow httptest servers.
var AllowLocalhost = false

// FetchResult contains the extracted content from a URL.
type FetchResult struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Content     string `json:"content"`
	ContentType string `json:"contentType"`
	BytesFetched int   `json:"bytesFetched"`
}

// Fetch retrieves a URL and extracts text content.
// Only http/https URLs are allowed. Response body is capped at 1 MB.
func Fetch(rawURL string) (*FetchResult, error) {
	if err := validateURL(rawURL); err != nil {
		return nil, err
	}

	client := &http.Client{
		Timeout: requestTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			// Validate redirect targets
			if err := validateURL(req.URL.String()); err != nil {
				return fmt.Errorf("redirect blocked: %w", err)
			}
			return nil
		},
	}

	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html, text/plain, application/json, */*")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Read with size limit
	limited := io.LimitReader(resp.Body, maxBodySize+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if len(body) > maxBodySize {
		body = body[:maxBodySize]
	}

	ct := resp.Header.Get("Content-Type")
	raw := string(body)

	result := &FetchResult{
		URL:          rawURL,
		ContentType:  ct,
		BytesFetched: len(body),
	}

	if strings.Contains(ct, "text/html") {
		result.Title = extractTitle(raw)
		result.Content = stripHTML(raw)
	} else {
		// Plain text, JSON, etc — use as-is
		result.Content = raw
	}

	// Trim excessive whitespace
	result.Content = collapseWhitespace(result.Content)

	return result, nil
}

// validateURL ensures only http/https schemes are used.
func validateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("only http/https URLs allowed, got %q", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("URL must have a host")
	}
	// Block obviously local addresses (unless testing)
	host := strings.ToLower(u.Hostname())
	if !AllowLocalhost && (host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "0.0.0.0") {
		return fmt.Errorf("local addresses not allowed")
	}
	// Block cloud metadata endpoints
	if host == "169.254.169.254" || host == "metadata.google.internal" {
		return fmt.Errorf("metadata endpoints not allowed")
	}
	return nil
}

// stripHTML removes HTML tags and extracts visible text.
func stripHTML(html string) string {
	var result strings.Builder
	inTag := false
	inScript := false
	inStyle := false

	lower := strings.ToLower(html)
	i := 0
	for i < len(html) {
		if !inTag && i < len(html) && html[i] == '<' {
			inTag = true
			// Check for script/style opening tags
			rest := lower[i:]
			if strings.HasPrefix(rest, "<script") {
				inScript = true
			} else if strings.HasPrefix(rest, "<style") {
				inStyle = true
			}
			// Check for closing script/style
			if strings.HasPrefix(rest, "</script") {
				inScript = false
			} else if strings.HasPrefix(rest, "</style") {
				inStyle = false
			}
			// Block-level tags get newlines
			if strings.HasPrefix(rest, "<p") || strings.HasPrefix(rest, "<div") ||
				strings.HasPrefix(rest, "<br") || strings.HasPrefix(rest, "<h") ||
				strings.HasPrefix(rest, "<li") || strings.HasPrefix(rest, "<tr") {
				result.WriteByte('\n')
			}
			i++
			continue
		}
		if inTag {
			if html[i] == '>' {
				inTag = false
			}
			i++
			continue
		}
		if inScript || inStyle {
			i++
			continue
		}
		result.WriteByte(html[i])
		i++
	}

	// Decode common HTML entities
	text := result.String()
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")
	text = strings.ReplaceAll(text, "&nbsp;", " ")

	return text
}

// extractTitle pulls the <title> content from HTML.
func extractTitle(html string) string {
	lower := strings.ToLower(html)
	start := strings.Index(lower, "<title")
	if start < 0 {
		return ""
	}
	// Find end of opening tag
	gtIdx := strings.Index(lower[start:], ">")
	if gtIdx < 0 {
		return ""
	}
	contentStart := start + gtIdx + 1
	end := strings.Index(lower[contentStart:], "</title>")
	if end < 0 {
		return ""
	}
	title := strings.TrimSpace(html[contentStart : contentStart+end])
	// Strip any remaining tags inside title
	title = stripHTML(title)
	return strings.TrimSpace(title)
}

// collapseWhitespace reduces runs of whitespace to single spaces/newlines.
func collapseWhitespace(s string) string {
	var result strings.Builder
	prevSpace := false
	prevNewline := false
	for _, c := range s {
		if c == '\n' || c == '\r' {
			if !prevNewline {
				result.WriteByte('\n')
				prevNewline = true
			}
			prevSpace = false
			continue
		}
		if c == ' ' || c == '\t' {
			if !prevSpace && !prevNewline {
				result.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		result.WriteRune(c)
		prevSpace = false
		prevNewline = false
	}
	return strings.TrimSpace(result.String())
}
