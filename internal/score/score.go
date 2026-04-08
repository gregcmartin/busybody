package score

import (
	"sort"
	"time"

	"busybody/internal/calendar"
)

const (
	WorkDayStartHour = 9  // 9 AM
	WorkDayEndHour   = 17 // 5 PM
	WorkMinsPerDay   = (WorkDayEndHour - WorkDayStartHour) * 60
)

// BusynessReport is the final analysis of a calendar's work-week availability.
type BusynessReport struct {
	URL            string
	Provider       string
	EventName      string
	WeekStart      time.Time
	WeekEnd        time.Time
	TotalWorkMins  int
	AvailableMins  int
	BookedMins     int
	BusynessScore  float64 // 0–100
	BusynessLabel  string
	Days           []DayReport
}

// DayReport is the per-day breakdown.
type DayReport struct {
	Date          time.Time
	Weekday       string
	TotalWorkMins int
	AvailableMins int
	BookedMins    int
	FreeWindows   []Window
}

// Window is a contiguous block of free time.
type Window struct {
	Start time.Time
	End   time.Time
}

// Calculate builds a BusynessReport from raw availability data.
// It evaluates the work week (Mon–Fri, 9 AM – 5 PM) that covers avail.RangeStart → RangeEnd.
func Calculate(avail *calendar.Availability) *BusynessReport {
	weekStart, weekEnd := workWeekBounds(avail.RangeStart)

	// Build a per-day map of available minutes using a 1-minute resolution bitmap.
	days := make([]DayReport, 5) // Mon–Fri
	totalAvail := 0

	for d := 0; d < 5; d++ {
		day := weekStart.AddDate(0, 0, d)
		dayStart := time.Date(day.Year(), day.Month(), day.Day(), WorkDayStartHour, 0, 0, 0, day.Location())
		dayEnd := time.Date(day.Year(), day.Month(), day.Day(), WorkDayEndHour, 0, 0, 0, day.Location())

		bitmap := make([]bool, WorkMinsPerDay) // true = available

		duration := avail.EventDuration
		if duration == 0 {
			duration = 30 * time.Minute
		}

		for _, slot := range avail.Slots {
			slotEnd := slot.Start.Add(duration)
			if slot.End.After(slot.Start) {
				slotEnd = slot.End
			}
			// Clamp to work hours.
			s := slot.Start
			e := slotEnd
			if s.Before(dayStart) || s.After(dayEnd) {
				continue
			}
			if e.After(dayEnd) {
				e = dayEnd
			}
			startMin := int(s.Sub(dayStart).Minutes())
			endMin := int(e.Sub(dayStart).Minutes())
			if startMin < 0 {
				startMin = 0
			}
			if endMin > WorkMinsPerDay {
				endMin = WorkMinsPerDay
			}
			for m := startMin; m < endMin; m++ {
				bitmap[m] = true
			}
		}

		availMins := 0
		for _, v := range bitmap {
			if v {
				availMins++
			}
		}

		windows := extractWindows(bitmap, dayStart)

		days[d] = DayReport{
			Date:          day,
			Weekday:       day.Weekday().String(),
			TotalWorkMins: WorkMinsPerDay,
			AvailableMins: availMins,
			BookedMins:    WorkMinsPerDay - availMins,
			FreeWindows:   windows,
		}
		totalAvail += availMins
	}

	totalWork := 5 * WorkMinsPerDay
	bookedMins := totalWork - totalAvail
	score := 0.0
	if totalWork > 0 {
		score = float64(bookedMins) / float64(totalWork) * 100
	}

	return &BusynessReport{
		URL:           avail.URL,
		Provider:      avail.Provider,
		EventName:     avail.EventName,
		WeekStart:     weekStart,
		WeekEnd:       weekEnd,
		TotalWorkMins: totalWork,
		AvailableMins: totalAvail,
		BookedMins:    bookedMins,
		BusynessScore: score,
		BusynessLabel: scoreLabel(score),
		Days:          days,
	}
}

// workWeekBounds returns the Monday 00:00 and Friday 23:59 of the work week
// containing or immediately following the given time.
func workWeekBounds(t time.Time) (time.Time, time.Time) {
	// Find next Monday if we're on a weekend, otherwise this week's Monday.
	weekday := t.Weekday()
	daysUntilMon := (8 - int(weekday)) % 7 // 0 if already Monday
	if weekday == time.Saturday {
		daysUntilMon = 2
	} else if weekday == time.Sunday {
		daysUntilMon = 1
	} else {
		daysUntilMon = -int(weekday-time.Monday)
	}

	mon := t.AddDate(0, 0, daysUntilMon)
	mon = time.Date(mon.Year(), mon.Month(), mon.Day(), 0, 0, 0, 0, t.Location())
	fri := mon.AddDate(0, 0, 4)
	fri = time.Date(fri.Year(), fri.Month(), fri.Day(), 23, 59, 59, 0, t.Location())
	return mon, fri
}

// extractWindows converts the minute bitmap into contiguous free-time windows.
func extractWindows(bitmap []bool, dayStart time.Time) []Window {
	var windows []Window
	inWindow := false
	var start int

	for i, avail := range bitmap {
		if avail && !inWindow {
			start = i
			inWindow = true
		} else if !avail && inWindow {
			windows = append(windows, Window{
				Start: dayStart.Add(time.Duration(start) * time.Minute),
				End:   dayStart.Add(time.Duration(i) * time.Minute),
			})
			inWindow = false
		}
	}
	if inWindow {
		windows = append(windows, Window{
			Start: dayStart.Add(time.Duration(start) * time.Minute),
			End:   dayStart.Add(time.Duration(len(bitmap)) * time.Minute),
		})
	}
	return windows
}

func scoreLabel(s float64) string {
	switch {
	case s >= 90:
		return "Slammed"
	case s >= 75:
		return "Very Busy"
	case s >= 55:
		return "Busy"
	case s >= 35:
		return "Moderate"
	case s >= 15:
		return "Mostly Free"
	default:
		return "Wide Open"
	}
}

// SortSlots sorts time slots chronologically.
func SortSlots(slots []calendar.TimeSlot) {
	sort.Slice(slots, func(i, j int) bool {
		return slots[i].Start.Before(slots[j].Start)
	})
}
