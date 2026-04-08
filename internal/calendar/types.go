package calendar

import "time"

// TimeSlot represents a window of availability.
type TimeSlot struct {
	Start time.Time
	End   time.Time
}

// Availability holds the available time slots returned by a calendar provider.
type Availability struct {
	Provider      string
	URL           string
	EventName     string
	EventDuration time.Duration
	Slots         []TimeSlot
	RangeStart    time.Time
	RangeEnd      time.Time
}

// Provider knows how to fetch availability from a specific calendar service.
type Provider interface {
	Name() string
	FetchAvailability(url string, start, end time.Time) (*Availability, error)
}
