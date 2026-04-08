package calendar

import (
	"os/exec"
	"strings"
	"time"
)

// LocalTZ returns the IANA timezone name for the system, e.g. "America/New_York".
// Falls back to "America/New_York" if detection fails.
func LocalTZ() string {
	// First try the TZ env or Go's built-in name.
	name := time.Now().Location().String()
	if name != "Local" && name != "" {
		return name
	}

	// macOS: read from systemsetup or /etc/localtime symlink.
	if out, err := exec.Command("readlink", "/etc/localtime").Output(); err == nil {
		s := strings.TrimSpace(string(out))
		// e.g. /var/db/timezone/zoneinfo/America/New_York
		if idx := strings.Index(s, "zoneinfo/"); idx != -1 {
			return s[idx+len("zoneinfo/"):]
		}
	}

	return "America/New_York"
}
