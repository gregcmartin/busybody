package calendar

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// CalendlyProvider fetches availability from Calendly's public booking API.
type CalendlyProvider struct{}

func (c *CalendlyProvider) Name() string { return "Calendly" }

// FetchAvailability queries Calendly for available time slots in the given range.
func (c *CalendlyProvider) FetchAvailability(rawURL string, start, end time.Time) (*Availability, error) {
	link, err := ParseCalendarURL(rawURL)
	if err != nil {
		return nil, err
	}

	// Discover all event types for this user.
	eventTypes, err := c.listEventTypes(link.Owner)
	if err != nil {
		return nil, err
	}
	if len(eventTypes) == 0 {
		return nil, fmt.Errorf("no event types found for %s", link.Owner)
	}

	// Pick the matching event type, or the first one if no slug specified.
	var et *calendlyEventType
	if link.Slug != "" {
		for i := range eventTypes {
			if eventTypes[i].Slug == link.Slug {
				et = &eventTypes[i]
				break
			}
		}
		if et == nil {
			return nil, fmt.Errorf("event type %q not found for %s (available: %s)", link.Slug, link.Owner, eventTypeSlugs(eventTypes))
		}
	} else {
		et = &eventTypes[0]
	}

	duration := time.Duration(et.Duration) * time.Minute
	if duration == 0 {
		duration = 30 * time.Minute
	}

	// Fetch available spots for the date range.
	slots, err := c.fetchRange(et.UUID, duration, start, end)
	if err != nil {
		return nil, err
	}

	return &Availability{
		Provider:      "Calendly",
		URL:           rawURL,
		EventName:     et.Name,
		EventDuration: duration,
		Slots:         slots,
		RangeStart:    start,
		RangeEnd:      end,
	}, nil
}

// --- Calendly internal API types ---

type calendlyEventType struct {
	UUID     string `json:"uuid"`
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Duration int    `json:"duration"` // minutes
	Color    string `json:"color"`
}

type calendlySpot struct {
	Status    string `json:"status"`
	StartTime string `json:"start_time"`
}

type calendlyDay struct {
	Date   string         `json:"date"`
	Status string         `json:"status"`
	Spots  []calendlySpot `json:"spots"`
}

type calendlyRangeResponse struct {
	Days []calendlyDay `json:"days"`
}

// --- API calls ---

func (c *CalendlyProvider) listEventTypes(owner string) ([]calendlyEventType, error) {
	apiURL := fmt.Sprintf("https://calendly.com/api/booking/profiles/%s/event_types", owner)
	resp, err := calendlyGet(apiURL)
	if err != nil {
		return nil, fmt.Errorf("listing event types: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("calendly event types API returned %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var eventTypes []calendlyEventType
	if err := json.Unmarshal(body, &eventTypes); err != nil {
		return nil, fmt.Errorf("parsing event types: %w", err)
	}
	return eventTypes, nil
}

func (c *CalendlyProvider) fetchRange(uuid string, duration time.Duration, start, end time.Time) ([]TimeSlot, error) {
	tz := LocalTZ()
	apiURL := fmt.Sprintf(
		"https://calendly.com/api/booking/event_types/%s/calendar/range?timezone=%s&diagnostics=false&range_start=%s&range_end=%s",
		uuid,
		tz,
		start.Format("2006-01-02"),
		end.Format("2006-01-02"),
	)

	resp, err := calendlyGet(apiURL)
	if err != nil {
		return nil, fmt.Errorf("fetching calendar range: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("calendly range API returned %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var rangeResp calendlyRangeResponse
	if err := json.Unmarshal(body, &rangeResp); err != nil {
		return nil, fmt.Errorf("parsing range response: %w", err)
	}

	var slots []TimeSlot
	for _, day := range rangeResp.Days {
		for _, spot := range day.Spots {
			if spot.Status != "available" {
				continue
			}
			t, err := time.Parse(time.RFC3339, spot.StartTime)
			if err != nil {
				continue
			}
			slots = append(slots, TimeSlot{
				Start: t.Local(),
				End:   t.Local().Add(duration),
			})
		}
	}
	return slots, nil
}

func calendlyGet(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; busybody/1.0)")
	req.Header.Set("Accept", "application/json")
	return http.DefaultClient.Do(req)
}

func eventTypeSlugs(ets []calendlyEventType) string {
	s := ""
	for i, et := range ets {
		if i > 0 {
			s += ", "
		}
		s += et.Slug
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
