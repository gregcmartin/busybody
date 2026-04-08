package scraper

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"busybody/internal/calendar"
)

// calendarPatterns matches URLs for supported calendar services.
var calendarPatterns = []*regexp.Regexp{
	regexp.MustCompile(`https?://(?:www\.)?calendly\.com/[a-zA-Z0-9_-]+(?:/[a-zA-Z0-9_-]+)?`),
	regexp.MustCompile(`https?://(?:www\.)?cal\.com/[a-zA-Z0-9_-]+(?:/[a-zA-Z0-9_-]+)?`),
	regexp.MustCompile(`https?://calendar\.google\.com/calendar/appointments/[^\s"'<>]+`),
	regexp.MustCompile(`https?://calendar\.app\.google/[^\s"'<>]+`),
}

// hrefPattern extracts href values from HTML.
var hrefPattern = regexp.MustCompile(`(?i)href\s*=\s*["']([^"']+)["']`)

// ScrapeCalendarLinks fetches a website and returns all calendar booking links found.
func ScrapeCalendarLinks(siteURL string) ([]calendar.CalendarLink, error) {
	siteURL = strings.TrimSpace(siteURL)
	if !strings.Contains(siteURL, "://") {
		siteURL = "https://" + siteURL
	}

	req, err := http.NewRequest("GET", siteURL, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid site URL: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; busybody/1.0)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching site: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("site returned HTTP %d", resp.StatusCode)
	}

	html := string(body)
	return extractCalendarLinks(html), nil
}

func extractCalendarLinks(html string) []calendar.CalendarLink {
	seen := make(map[string]bool)
	var links []calendar.CalendarLink

	// Extract from href attributes.
	for _, m := range hrefPattern.FindAllStringSubmatch(html, -1) {
		if len(m) >= 2 {
			addIfCalendar(m[1], seen, &links)
		}
	}

	// Also scan raw text for calendar URLs not in href attrs.
	for _, pat := range calendarPatterns {
		for _, u := range pat.FindAllString(html, -1) {
			addIfCalendar(u, seen, &links)
		}
	}

	return links
}

func addIfCalendar(raw string, seen map[string]bool, out *[]calendar.CalendarLink) {
	raw = strings.TrimSpace(raw)
	if seen[raw] {
		return
	}
	link, err := calendar.ParseCalendarURL(raw)
	if err != nil {
		return
	}
	seen[raw] = true
	*out = append(*out, *link)
}
