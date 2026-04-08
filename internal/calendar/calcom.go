package calendar

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"time"
)

// CalcomProvider fetches availability from Cal.com's public v2 slots API.
type CalcomProvider struct{}

func (c *CalcomProvider) Name() string { return "Cal.com" }

func (c *CalcomProvider) FetchAvailability(rawURL string, start, end time.Time) (*Availability, error) {
	link, err := ParseCalendarURL(rawURL)
	if err != nil {
		return nil, err
	}

	slug := link.Slug
	if slug == "" {
		slug, err = c.discoverEventSlug(link.Owner)
		if err != nil {
			return nil, fmt.Errorf("no event type specified and auto-discovery failed: %w", err)
		}
	}

	slots, duration, err := c.fetchSlots(link.Owner, slug, start, end)
	if err != nil {
		return nil, err
	}

	return &Availability{
		Provider:      "Cal.com",
		URL:           rawURL,
		EventName:     slug,
		EventDuration: duration,
		Slots:         slots,
		RangeStart:    start,
		RangeEnd:      end,
	}, nil
}

// --- Cal.com API types ---

type calcomV2Response struct {
	Status string          `json:"status"`
	Data   calcomV2Data    `json:"data"`
}

type calcomV2Data struct {
	Slots map[string][]calcomSlot `json:"slots"`
}

type calcomSlot struct {
	Time string `json:"time"`
}

// --- API calls ---

func (c *CalcomProvider) fetchSlots(owner, slug string, start, end time.Time) ([]TimeSlot, time.Duration, error) {
	// Cal.com v2 public slots API
	apiURL := fmt.Sprintf(
		"https://cal.com/api/v2/slots/available?usernameList[]=%s&eventTypeSlug=%s&startTime=%s&endTime=%s",
		url.QueryEscape(owner),
		url.QueryEscape(slug),
		url.QueryEscape(start.UTC().Format(time.RFC3339)),
		url.QueryEscape(end.UTC().Format(time.RFC3339)),
	)

	resp, err := calcomGet(apiURL)
	if err != nil {
		return nil, 0, fmt.Errorf("fetching slots: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("cal.com slots API returned %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var v2Resp calcomV2Response
	if err := json.Unmarshal(body, &v2Resp); err != nil {
		return nil, 0, fmt.Errorf("parsing slots response: %w", err)
	}

	duration := 30 * time.Minute // default
	var slots []TimeSlot
	for _, daySlots := range v2Resp.Data.Slots {
		for _, s := range daySlots {
			t, err := time.Parse(time.RFC3339, s.Time)
			if err != nil {
				t, err = time.Parse("2006-01-02T15:04:05.000Z", s.Time)
				if err != nil {
					continue
				}
			}
			slots = append(slots, TimeSlot{
				Start: t.Local(),
				End:   t.Local().Add(duration),
			})
		}
	}
	return slots, duration, nil
}

// discoverEventSlug scrapes the user's Cal.com profile page to find event type slugs.
func (c *CalcomProvider) discoverEventSlug(owner string) (string, error) {
	profileURL := fmt.Sprintf("https://cal.com/%s", url.PathEscape(owner))
	resp, err := calcomGet(profileURL)
	if err != nil {
		return "", fmt.Errorf("fetching profile page: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("cal.com profile page returned %d", resp.StatusCode)
	}

	// Extract event slugs from links like /username/slug in the HTML.
	pattern := regexp.MustCompile(fmt.Sprintf(`/%s/([a-zA-Z0-9_-]+)`, regexp.QuoteMeta(owner)))
	matches := pattern.FindAllStringSubmatch(string(body), -1)

	seen := make(map[string]bool)
	for _, m := range matches {
		if len(m) >= 2 && !seen[m[1]] {
			return m[1], nil // return the first event type found
		}
	}
	return "", fmt.Errorf("no event types found on %s's profile page", owner)
}

func calcomGet(rawURL string) (*http.Response, error) {
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; busybody/1.0)")
	req.Header.Set("Accept", "application/json")
	return http.DefaultClient.Do(req)
}
