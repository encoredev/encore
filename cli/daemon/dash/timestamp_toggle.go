package dash

import (
	"fmt"
	"time"
)

// TimestampFormat represents the format preference for displaying timestamps
type TimestampFormat string

const (
	TimestampRelative TimestampFormat = "relative"
	TimestampAbsolute TimestampFormat = "absolute"
)

// TimestampPreference holds user preference for timestamp display
type TimestampPreference struct {
	Format TimestampFormat `json:"format"`
}

// FormatTimestamp formats a timestamp based on user preference
func FormatTimestamp(t time.Time, format TimestampFormat) string {
	switch format {
	case TimestampAbsolute:
		return t.Format("2006-01-02 15:04:05.000")
	case TimestampRelative:
		fallthrough
	default:
		return formatRelativeTime(t)
	}
}

// formatRelativeTime formats time as relative (e.g., "2 min ago")
func formatRelativeTime(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	if diff < time.Minute {
		return "just now"
	} else if diff < time.Hour {
		mins := int(diff.Minutes())
		if mins == 1 {
			return "1 min ago"
		}
		return fmt.Sprintf("%d mins ago", mins)
	} else if diff < 24*time.Hour {
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else {
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}