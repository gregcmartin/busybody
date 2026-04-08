package calendar

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"
)

// GoogleProvider fetches availability from Google Calendar appointment scheduling pages.
type GoogleProvider struct{}

func (g *GoogleProvider) Name() string { return "Google Calendar" }

func (g *GoogleProvider) FetchAvailability(rawURL string, start, end time.Time) (*Availability, error) {
	link, err := ParseCalendarURL(rawURL)
	if err != nil {
		return nil, err
	}

	slots, err := g.scrapeAppointmentPage(link.Raw, start, end)
	if err != nil {
		return nil, err
	}

	return &Availability{
		Provider:      "Google Calendar",
		URL:           rawURL,
		EventName:     "Appointment",
		EventDuration: 30 * time.Minute,
		Slots:         slots,
		RangeStart:    start,
		RangeEnd:      end,
	}, nil
}

// scrapeAppointmentPage fetches a Google Calendar appointment page and attempts
// to extract availability data from embedded JSON.
func (g *GoogleProvider) scrapeAppointmentPage(pageURL string, start, end time.Time) ([]TimeSlot, error) {
	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; busybody/1.0)")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching Google appointment page: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google appointment page returned %d", resp.StatusCode)
	}

	return g.parseEmbeddedSlots(string(body), start, end)
}

// parseEmbeddedSlots extracts availability from Google's embedded AF_initDataCallback JSON.
// Google appointment pages embed scheduling data via AF_initDataCallback calls.
func (g *GoogleProvider) parseEmbeddedSlots(html string, start, end time.Time) ([]TimeSlot, error) {
	// Google embeds data in AF_initDataCallback calls with JSON arrays.
	// Look for timestamp arrays that represent available time slots.
	re := regexp.MustCompile(`AF_initDataCallback\(\{[^}]*data:(\[[\s\S]*?\])\s*\}\)`)
	matches := re.FindAllStringSubmatch(html, -1)

	var slots []TimeSlot
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		slots = append(slots, g.extractTimestamps(match[1], start, end)...)
	}

	if len(slots) == 0 {
		// Fallback: try to find ISO 8601 timestamps in the page.
		return g.extractISOTimestamps(html, start, end)
	}
	return slots, nil
}

// extractTimestamps looks for Unix millisecond timestamps in a JSON array.
func (g *GoogleProvider) extractTimestamps(jsonStr string, start, end time.Time) []TimeSlot {
	var raw interface{}
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil
	}
	var timestamps []int64
	g.walkJSON(raw, &timestamps)

	var slots []TimeSlot
	for i := 0; i+1 < len(timestamps); i += 2 {
		s := time.UnixMilli(timestamps[i]).Local()
		e := time.UnixMilli(timestamps[i+1]).Local()
		if s.After(start.Add(-24*time.Hour)) && e.Before(end.Add(24*time.Hour)) {
			slots = append(slots, TimeSlot{Start: s, End: e})
		}
	}
	return slots
}

// walkJSON recursively finds numbers that look like Unix-ms timestamps (2020-2030 range).
func (g *GoogleProvider) walkJSON(v interface{}, out *[]int64) {
	switch val := v.(type) {
	case []interface{}:
		for _, item := range val {
			g.walkJSON(item, out)
		}
	case float64:
		ms := int64(val)
		// Unix-ms range roughly 2020 to 2030
		if ms > 1577836800000 && ms < 1893456000000 {
			*out = append(*out, ms)
		}
	}
}

// extractISOTimestamps is a fallback that regex-matches ISO 8601 datetimes in the HTML.
func (g *GoogleProvider) extractISOTimestamps(html string, start, end time.Time) ([]TimeSlot, error) {
	re := regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[Z+-][\d:]*`)
	matches := re.FindAllString(html, -1)

	var times []time.Time
	for _, m := range matches {
		t, err := time.Parse(time.RFC3339, m)
		if err != nil {
			continue
		}
		t = t.Local()
		if t.After(start.Add(-24*time.Hour)) && t.Before(end.Add(24*time.Hour)) {
			times = append(times, t)
		}
	}

	var slots []TimeSlot
	for i := 0; i+1 < len(times); i += 2 {
		slots = append(slots, TimeSlot{Start: times[i], End: times[i+1]})
	}

	if len(slots) == 0 {
		return nil, fmt.Errorf("could not extract availability from Google Calendar page — the page may require JavaScript rendering")
	}
	return slots, nil
}
