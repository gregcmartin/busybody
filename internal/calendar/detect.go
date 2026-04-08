package calendar

import (
	"fmt"
	"net/url"
	"strings"
)

// CalendarLink holds a normalized calendar URL and its detected provider type.
type CalendarLink struct {
	Raw      string
	Provider string // "calendly", "calcom", "google"
	Owner    string
	Slug     string // event type slug (may be empty)
}

// DetectProvider inspects a URL and returns the matching Provider implementation,
// or an error if the service is not recognized.
func DetectProvider(raw string) (Provider, *CalendarLink, error) {
	link, err := ParseCalendarURL(raw)
	if err != nil {
		return nil, nil, err
	}
	switch link.Provider {
	case "calendly":
		return &CalendlyProvider{}, link, nil
	case "calcom":
		return &CalcomProvider{}, link, nil
	case "google":
		return &GoogleProvider{}, link, nil
	default:
		return nil, nil, fmt.Errorf("unsupported calendar provider: %s", link.Provider)
	}
}

// ParseCalendarURL normalizes a raw URL string and detects the calendar service.
func ParseCalendarURL(raw string) (*CalendarLink, error) {
	raw = strings.TrimSpace(raw)
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid URL %q: %w", raw, err)
	}
	host := strings.ToLower(u.Hostname())
	path := strings.Trim(u.Path, "/")
	parts := strings.SplitN(path, "/", 3)

	link := &CalendarLink{Raw: u.String()}

	switch {
	case strings.Contains(host, "calendly.com"):
		link.Provider = "calendly"
		if len(parts) >= 1 && parts[0] != "" {
			link.Owner = parts[0]
		}
		if len(parts) >= 2 {
			link.Slug = parts[1]
		}
	case host == "cal.com" || strings.HasSuffix(host, ".cal.com"):
		link.Provider = "calcom"
		if len(parts) >= 1 && parts[0] != "" {
			link.Owner = parts[0]
		}
		if len(parts) >= 2 {
			link.Slug = parts[1]
		}
	case strings.Contains(host, "calendar.google.com") || strings.Contains(host, "calendar.app.google"):
		link.Provider = "google"
		link.Owner = path
	default:
		return nil, fmt.Errorf("unrecognized calendar service in URL %q (supported: calendly.com, cal.com, calendar.google.com)", raw)
	}

	if link.Provider != "google" && link.Owner == "" {
		return nil, fmt.Errorf("could not extract username from URL %q", raw)
	}
	return link, nil
}

// IsCalendarURL returns true if the URL appears to belong to a supported calendar service.
func IsCalendarURL(raw string) bool {
	_, err := ParseCalendarURL(raw)
	return err == nil
}
