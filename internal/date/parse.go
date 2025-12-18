package date

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sjatkinson/threadkeeper/internal/config"
)

// Clock provides the current time for date parsing.
// This interface allows injecting a fixed time for testing.
type Clock interface {
	Now() time.Time
}

// RealClock implements Clock using the system clock.
type RealClock struct{}

func (RealClock) Now() time.Time {
	return time.Now()
}

// FixedClock implements Clock with a fixed time for testing.
type FixedClock struct {
	FixedTime time.Time
}

func (c FixedClock) Now() time.Time {
	return c.FixedTime
}

// ParseDate parses a date string according to the specified locale and returns
// a canonical YYYY-MM-DD string. It handles various input formats based on locale.
//
// Parameters:
//   - input: the date string to parse
//   - locale: the date locale (iso, us, eu)
//   - clock: provides current time for year-omitted dates
//   - tz: timezone for determining "today" (defaults to America/Los_Angeles)
//
// Returns:
//   - canonical date string (YYYY-MM-DD)
//   - error if parsing fails
func ParseDate(input string, locale config.DateLocale, clock Clock, tz *time.Location) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("invalid due date: empty input")
	}

	// Default timezone
	if tz == nil {
		var err error
		tz, err = time.LoadLocation("America/Los_Angeles")
		if err != nil {
			tz = time.UTC
		}
	}

	now := clock.Now().In(tz)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, tz)

	// Step 1: Check for shortcuts (today, +1, +2, etc.)
	if canonical, err := parseShortcuts(input, today); err == nil {
		return canonical, nil
	}

	// Step 2: Try ISO-like formats (YYYY-MM-DD, YYYY/MM/DD, YYYY.MM.DD, YYYYMMDD)
	if canonical, err := parseISOFormats(input); err == nil {
		return canonical, nil
	}

	// Step 3: If locale is us or eu, try locale-specific formats with year
	if locale == config.DateLocaleUS || locale == config.DateLocaleEU {
		if canonical, err := parseLocaleWithYear(input, locale, today); err == nil {
			return canonical, nil
		}
	}

	// Step 4: If locale is us or eu, try locale-specific formats without year (next-occurrence)
	if locale == config.DateLocaleUS || locale == config.DateLocaleEU {
		if canonical, err := parseLocaleWithoutYear(input, locale, today); err == nil {
			return canonical, nil
		}
		// If both locale formats failed but input looks numeric, give helpful error
		if looksLikeNumericFormat(input) {
			var expected string
			if locale == config.DateLocaleUS {
				expected = "MM/DD[/YYYY] or MM-DD[-YYYY]"
			} else {
				expected = "DD/MM[/YYYY] or DD-MM[-YYYY]"
			}
			return "", fmt.Errorf("invalid due date for locale %q: expected %s, got %q", locale, expected, input)
		}
	}

	// Step 5: If we get here and locale is iso, check if input looks like numeric format
	if locale == config.DateLocaleISO {
		if looksLikeNumericFormat(input) {
			return "", fmt.Errorf("invalid due date: ambiguous numeric format %q. Use YYYY-MM-DD or set date_locale=us or date_locale=eu", input)
		}
	}

	// Final error
	return "", fmt.Errorf("invalid due date: unable to parse %q", input)
}

// parseShortcuts handles date shortcuts like "today", "+1", "+2", etc.
func parseShortcuts(input string, today time.Time) (string, error) {
	input = strings.ToLower(strings.TrimSpace(input))

	// Check for "today"
	if input == "today" {
		return today.Format("2006-01-02"), nil
	}

	// Check for "+N" pattern where N is a positive integer
	if strings.HasPrefix(input, "+") {
		daysStr := input[1:]
		days, err := strconv.Atoi(daysStr)
		if err != nil {
			return "", fmt.Errorf("not a shortcut")
		}
		if days < 0 {
			return "", fmt.Errorf("invalid shortcut: days must be non-negative")
		}
		// Add days to today
		targetDate := today.AddDate(0, 0, days)
		return targetDate.Format("2006-01-02"), nil
	}

	return "", fmt.Errorf("not a shortcut")
}

// parseISOFormats tries to parse ISO-like formats: YYYY-MM-DD, YYYY/MM/DD, YYYY.MM.DD, YYYYMMDD
func parseISOFormats(input string) (string, error) {
	// Try YYYY-MM-DD
	if t, err := time.Parse("2006-01-02", input); err == nil {
		return t.Format("2006-01-02"), nil
	}

	// Try YYYY/MM/DD
	if t, err := time.Parse("2006/01/02", input); err == nil {
		return t.Format("2006-01-02"), nil
	}

	// Try YYYY.MM.DD
	if t, err := time.Parse("2006.01.02", input); err == nil {
		return t.Format("2006-01-02"), nil
	}

	// Try YYYYMMDD (8 digits)
	if matched, _ := regexp.MatchString(`^\d{8}$`, input); matched {
		if t, err := time.Parse("20060102", input); err == nil {
			return t.Format("2006-01-02"), nil
		}
	}

	return "", fmt.Errorf("not an ISO format")
}

// parseLocaleWithYear parses locale-specific formats with year: MM/DD/YYYY or DD/MM/YYYY
func parseLocaleWithYear(input string, locale config.DateLocale, today time.Time) (string, error) {
	var layouts []string
	if locale == config.DateLocaleUS {
		// Try MM/DD/YYYY (try both 2-digit and flexible formats)
		layouts = []string{"01/02/2006", "1/2/2006", "01/2/2006", "1/02/2006"}
	} else { // EU
		// Try DD/MM/YYYY (try both 2-digit and flexible formats)
		layouts = []string{"02/01/2006", "2/1/2006", "02/1/2006", "2/01/2006"}
	}

	// Try with slashes
	for _, layout := range layouts {
		if t, err := time.Parse(layout, input); err == nil {
			// Validate the parsed date makes sense (time.Parse already validates date validity)
			if t.Year() < 1900 || t.Year() > 2100 {
				return "", fmt.Errorf("invalid year")
			}
			return t.Format("2006-01-02"), nil
		}
	}

	// Try with dashes
	for _, layout := range layouts {
		layoutDash := strings.ReplaceAll(layout, "/", "-")
		if t, err := time.Parse(layoutDash, input); err == nil {
			if t.Year() < 1900 || t.Year() > 2100 {
				return "", fmt.Errorf("invalid year")
			}
			return t.Format("2006-01-02"), nil
		}
	}

	return "", fmt.Errorf("not a locale format with year")
}

// parseLocaleWithoutYear parses locale-specific formats without year: MM/DD or DD/MM
// Applies the "next occurrence" rule.
func parseLocaleWithoutYear(input string, locale config.DateLocale, today time.Time) (string, error) {
	var layouts []string
	if locale == config.DateLocaleUS {
		// MM/DD format (try both 2-digit and flexible formats)
		layouts = []string{"01/02", "1/2", "01/2", "1/02"}
	} else { // EU
		// DD/MM format (try both 2-digit and flexible formats)
		layouts = []string{"02/01", "2/1", "02/1", "2/01"}
	}

	var t time.Time
	var err error

	// Try with slashes
	for _, layout := range layouts {
		if t, err = time.Parse(layout, input); err == nil {
			goto found
		}
	}

	// Try with dashes
	for _, layout := range layouts {
		layoutDash := strings.ReplaceAll(layout, "/", "-")
		if t, err = time.Parse(layoutDash, input); err == nil {
			goto found
		}
	}

	return "", fmt.Errorf("not a locale format without year")

found:
	// Apply next-occurrence rule
	// Set year to current year
	candidate := time.Date(today.Year(), t.Month(), t.Day(), 0, 0, 0, 0, today.Location())

	// If the candidate date is before today, roll forward one year
	if candidate.Before(today) {
		candidate = time.Date(today.Year()+1, t.Month(), t.Day(), 0, 0, 0, 0, today.Location())
	}

	return candidate.Format("2006-01-02"), nil
}

// looksLikeNumericFormat checks if input looks like a numeric date format (e.g., 12/15/2025 or 12/15)
func looksLikeNumericFormat(input string) bool {
	// Check for patterns like MM/DD/YYYY, MM/DD, DD/MM/YYYY, DD/MM
	matched, _ := regexp.MatchString(`^\d{1,2}[/-]\d{1,2}([/-]\d{2,4})?$`, input)
	return matched
}

// FormatCanonical formats a time.Time as canonical YYYY-MM-DD.
// This is the single source of truth for canonical date formatting.
func FormatCanonical(t time.Time) string {
	return t.Format("2006-01-02")
}
