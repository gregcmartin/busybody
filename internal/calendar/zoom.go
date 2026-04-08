package calendar

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// ZoomProvider fetches availability from Zoom Scheduler's public API.
type ZoomProvider struct{}

func (z *ZoomProvider) Name() string { return "Zoom Scheduler" }

func (z *ZoomProvider) FetchAvailability(rawURL string, start, end time.Time) (*Availability, error) {
	link, err := ParseCalendarURL(rawURL)
	if err != nil {
		return nil, err
	}

	appts, err := z.listAppointments(link.Owner)
	if err != nil {
		return nil, err
	}
	if len(appts) == 0 {
		return nil, fmt.Errorf("no appointments found for %s", link.Owner)
	}

	// Pick matching appointment by slug, or default to the first active one.
	var appt *zoomAppointment
	if link.Slug != "" {
		for i := range appts {
			if appts[i].Slug == link.Slug {
				appt = &appts[i]
				break
			}
		}
	}
	if appt == nil {
		for i := range appts {
			if appts[i].Active {
				appt = &appts[i]
				break
			}
		}
	}
	if appt == nil {
		appt = &appts[0]
	}

	duration := time.Duration(appt.Duration) * time.Minute
	if duration == 0 {
		duration = 30 * time.Minute
	}

	slots, err := z.fetchAvailableTimes(link.Owner, appt.ID, duration, start, end)
	if err != nil {
		return nil, err
	}

	return &Availability{
		Provider:      "Zoom Scheduler",
		URL:           rawURL,
		EventName:     appt.Summary,
		EventDuration: duration,
		Slots:         slots,
		RangeStart:    start,
		RangeEnd:      end,
	}, nil
}

// --- Zoom API types ---

type zoomAppointment struct {
	ID       string `json:"id"`
	Summary  string `json:"summary"`
	Slug     string `json:"slug"`
	Duration int    `json:"duration"` // minutes
	Active   bool   `json:"active"`
}

type zoomAppointmentsResponse struct {
	Items []zoomAppointment `json:"items"`
}

type zoomSpot struct {
	Status    string `json:"status"`
	StartTime string `json:"startTime"`
}

type zoomDay struct {
	Date  string     `json:"date"`
	Spots []zoomSpot `json:"spots"`
}

type zoomAvailableTimesResponse struct {
	Days []zoomDay `json:"days"`
}

// --- API calls ---

func (z *ZoomProvider) listAppointments(userSlug string) ([]zoomAppointment, error) {
	apiURL := fmt.Sprintf("https://scheduler.zoom.us/zscheduler/v1/appointments?user=%s",
		url.QueryEscape(userSlug))

	resp, err := zoomGet(apiURL)
	if err != nil {
		return nil, fmt.Errorf("listing appointments: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("zoom appointments API returned %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var apptResp zoomAppointmentsResponse
	if err := json.Unmarshal(body, &apptResp); err != nil {
		return nil, fmt.Errorf("parsing appointments: %w", err)
	}
	return apptResp.Items, nil
}

func (z *ZoomProvider) fetchAvailableTimes(userSlug, apptID string, duration time.Duration, start, end time.Time) ([]TimeSlot, error) {
	tz := LocalTZ()
	apiURL := fmt.Sprintf(
		"https://scheduler.zoom.us/zscheduler/v1/appointments/%s/availableTimes?user=%s&timeZone=%s&timeMin=%s&timeMax=%s",
		url.PathEscape(apptID),
		url.QueryEscape(userSlug),
		url.QueryEscape(tz),
		url.QueryEscape(start.UTC().Format(time.RFC3339)),
		url.QueryEscape(end.UTC().Format(time.RFC3339)),
	)

	resp, err := zoomGet(apiURL)
	if err != nil {
		return nil, fmt.Errorf("fetching available times: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("zoom availableTimes API returned %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var atResp zoomAvailableTimesResponse
	if err := json.Unmarshal(body, &atResp); err != nil {
		return nil, fmt.Errorf("parsing available times: %w", err)
	}

	var slots []TimeSlot
	for _, day := range atResp.Days {
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

func zoomGet(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; busybody/1.0)")
	req.Header.Set("Accept", "application/json")
	return http.DefaultClient.Do(req)
}
