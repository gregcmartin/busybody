package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"busybody/internal/calendar"
	"busybody/internal/score"
	"busybody/internal/scraper"
)

func main() {
	site := flag.String("site", "", "Website URL to scrape for calendar links")
	cal := flag.String("calendar", "", "Direct calendar URL to check (calendly.com/x, cal.com/x, etc.)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "busybody - reverse-engineer calendar availability\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  busybody -site <website>      Scrape site for calendar links\n")
		fmt.Fprintf(os.Stderr, "  busybody -calendar <url>      Check a calendar directly\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  busybody -site startup.com\n")
		fmt.Fprintf(os.Stderr, "  busybody -calendar calendly.com/johndoe/30min\n")
		fmt.Fprintf(os.Stderr, "  busybody -calendar cal.com/founder/intro\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *site == "" && *cal == "" {
		flag.Usage()
		os.Exit(1)
	}

	if *site != "" {
		runSiteMode(*site)
	} else {
		runCalendarMode(*cal)
	}
}

func runSiteMode(siteURL string) {
	fmt.Printf("Scanning %s for calendar links...\n\n", siteURL)

	links, err := scraper.ScrapeCalendarLinks(siteURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(links) == 0 {
		fmt.Println("No calendar links found.")
		return
	}

	fmt.Printf("Found %d calendar link(s):\n", len(links))
	for i, l := range links {
		fmt.Printf("  %d. [%s] %s\n", i+1, l.Provider, l.Raw)
	}
	fmt.Println()

	for _, l := range links {
		checkCalendar(l.Raw)
		fmt.Println()
	}
}

func runCalendarMode(calURL string) {
	checkCalendar(calURL)
}

func checkCalendar(rawURL string) {
	provider, link, err := calendar.DetectProvider(rawURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	start, end := nextWorkWeek()
	label := link.Owner
	if link.Slug != "" {
		label += "/" + link.Slug
	}

	fmt.Printf("Checking %s (%s)...\n", label, provider.Name())
	fmt.Printf("Week: %s - %s\n\n", start.Format("Mon Jan 2"), end.Format("Mon Jan 2, 2006"))

	avail, err := provider.FetchAvailability(rawURL, start, end)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Error fetching availability: %v\n", err)
		return
	}

	report := score.Calculate(avail)
	printReport(report)
}

func nextWorkWeek() (time.Time, time.Time) {
	now := time.Now()
	weekday := now.Weekday()

	// Find this week's Monday; if Sat/Sun advance to next Monday.
	var daysToMon int
	switch weekday {
	case time.Saturday:
		daysToMon = 2
	case time.Sunday:
		daysToMon = 1
	default:
		daysToMon = -int(weekday - time.Monday)
	}

	mon := now.AddDate(0, 0, daysToMon)
	mon = time.Date(mon.Year(), mon.Month(), mon.Day(), 0, 0, 0, 0, now.Location())
	fri := mon.AddDate(0, 0, 4)
	fri = time.Date(fri.Year(), fri.Month(), fri.Day(), 23, 59, 59, 0, now.Location())
	return mon, fri
}

func printReport(r *score.BusynessReport) {
	// Header
	fmt.Printf("%-12s %10s %10s   %s\n", "Day", "Available", "Booked", "Free Windows")
	fmt.Printf("%-12s %10s %10s   %s\n", strings.Repeat("-", 12), strings.Repeat("-", 10), strings.Repeat("-", 10), strings.Repeat("-", 30))

	for _, d := range r.Days {
		windows := formatWindows(d.FreeWindows)
		if windows == "" {
			windows = "none"
		}
		fmt.Printf("%-12s %10s %10s   %s\n",
			d.Weekday,
			fmtDuration(d.AvailableMins),
			fmtDuration(d.BookedMins),
			windows,
		)
	}

	fmt.Println()
	fmt.Printf("Busyness Score: %.0f/100 (%s)\n", r.BusynessScore, r.BusynessLabel)
	fmt.Printf("Available: %s / %s work week\n", fmtDuration(r.AvailableMins), fmtDuration(r.TotalWorkMins))
	fmt.Printf("Booked:    %s / %s work week\n", fmtDuration(r.BookedMins), fmtDuration(r.TotalWorkMins))
}

func fmtDuration(mins int) string {
	h := mins / 60
	m := mins % 60
	return fmt.Sprintf("%dh %02dm", h, m)
}

func formatWindows(windows []score.Window) string {
	if len(windows) == 0 {
		return ""
	}
	parts := make([]string, len(windows))
	for i, w := range windows {
		parts[i] = fmt.Sprintf("%s-%s", w.Start.Format("3:04pm"), w.End.Format("3:04pm"))
	}
	return strings.Join(parts, ", ")
}
