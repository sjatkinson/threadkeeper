package date

import (
	"testing"
	"time"

	"github.com/sjatkinson/threadkeeper/internal/config"
)

func TestParseDate_ISOFormats(t *testing.T) {
	clock := FixedClock{FixedTime: time.Date(2025, 12, 15, 10, 0, 0, 0, time.UTC)}
	locale := config.DateLocaleISO

	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{"YYYY-MM-DD", "2025-12-15", "2025-12-15", false},
		{"YYYY/MM/DD", "2025/12/15", "2025-12-15", false},
		{"YYYY.MM.DD", "2025.12.15", "2025-12-15", false},
		{"YYYYMMDD", "20251215", "2025-12-15", false},
		{"reject numeric US format", "12/15/2025", "", true},
		{"reject numeric EU format", "15/12/2025", "", true},
		{"reject short numeric", "12/15", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseDate(tt.input, locale, clock, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && result != tt.expected {
				t.Errorf("ParseDate() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestParseDate_USLocale(t *testing.T) {
	clock := FixedClock{FixedTime: time.Date(2025, 12, 15, 10, 0, 0, 0, time.UTC)}
	locale := config.DateLocaleUS
	tz, _ := time.LoadLocation("America/Los_Angeles")

	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		// ISO formats should still work
		{"YYYY-MM-DD", "2025-12-15", "2025-12-15", false},
		{"YYYY/MM/DD", "2025/12/15", "2025-12-15", false},
		// US formats with year
		{"MM/DD/YYYY", "12/15/2025", "2025-12-15", false},
		{"MM-DD-YYYY", "12-15-2025", "2025-12-15", false},
		{"MM/DD/YYYY single digit", "1/5/2025", "2025-01-05", false},
		// US formats without year (next occurrence)
		{"MM/DD future", "12/20", "2025-12-20", false}, // Same year, future
		{"MM/DD past", "12/01", "2026-12-01", false},   // Past date, roll forward
		{"MM-DD future", "12-20", "2025-12-20", false},
		{"MM-DD past", "12-01", "2026-12-01", false},
		// Invalid dates
		{"invalid month", "13/15/2025", "", true},
		{"invalid day", "12/32/2025", "", true},
		{"invalid date", "02/30/2025", "", true},
		// Wrong locale format
		{"EU format rejected", "15/12/2025", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseDate(tt.input, locale, clock, tz)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && result != tt.expected {
				t.Errorf("ParseDate() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestParseDate_EULocale(t *testing.T) {
	clock := FixedClock{FixedTime: time.Date(2025, 12, 15, 10, 0, 0, 0, time.UTC)}
	locale := config.DateLocaleEU
	tz, _ := time.LoadLocation("America/Los_Angeles")

	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		// ISO formats should still work
		{"YYYY-MM-DD", "2025-12-15", "2025-12-15", false},
		{"YYYY/MM/DD", "2025/12/15", "2025-12-15", false},
		// EU formats with year
		{"DD/MM/YYYY", "15/12/2025", "2025-12-15", false},
		{"DD-MM-YYYY", "15-12-2025", "2025-12-15", false},
		{"DD/MM/YYYY single digit", "5/1/2025", "2025-01-05", false},
		// EU formats without year (next occurrence)
		{"DD/MM future", "20/12", "2025-12-20", false}, // Same year, future
		{"DD/MM past", "01/12", "2026-12-01", false},   // Past date, roll forward
		{"DD-MM future", "20-12", "2025-12-20", false},
		{"DD-MM past", "01-12", "2026-12-01", false},
		// Invalid dates
		{"invalid day", "32/01/2025", "", true},
		{"invalid month", "15/13/2025", "", true},
		{"invalid date", "30/02/2025", "", true},
		// Wrong locale format
		{"US format rejected", "12/15/2025", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseDate(tt.input, locale, clock, tz)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && result != tt.expected {
				t.Errorf("ParseDate() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestParseDate_NextOccurrence(t *testing.T) {
	tz, _ := time.LoadLocation("America/Los_Angeles")

	tests := []struct {
		name        string
		today       time.Time
		input       string
		locale      config.DateLocale
		expected    string
		description string
	}{
		{
			name:        "US: same month future",
			today:       time.Date(2025, 12, 15, 10, 0, 0, 0, tz),
			input:       "12/20",
			locale:      config.DateLocaleUS,
			expected:    "2025-12-20",
			description: "Dec 20 is after Dec 15, same year",
		},
		{
			name:        "US: same month past",
			today:       time.Date(2025, 12, 15, 10, 0, 0, 0, tz),
			input:       "12/01",
			locale:      config.DateLocaleUS,
			expected:    "2026-12-01",
			description: "Dec 1 is before Dec 15, roll to next year",
		},
		{
			name:        "US: next month",
			today:       time.Date(2025, 12, 15, 10, 0, 0, 0, tz),
			input:       "01/15",
			locale:      config.DateLocaleUS,
			expected:    "2026-01-15",
			description: "Jan 15 next year",
		},
		{
			name:        "EU: same month future",
			today:       time.Date(2025, 12, 15, 10, 0, 0, 0, tz),
			input:       "20/12",
			locale:      config.DateLocaleEU,
			expected:    "2025-12-20",
			description: "Dec 20 is after Dec 15, same year",
		},
		{
			name:        "EU: same month past",
			today:       time.Date(2025, 12, 15, 10, 0, 0, 0, tz),
			input:       "01/12",
			locale:      config.DateLocaleEU,
			expected:    "2026-12-01",
			description: "Dec 1 is before Dec 15, roll to next year",
		},
		{
			name:        "EU: next month",
			today:       time.Date(2025, 12, 15, 10, 0, 0, 0, tz),
			input:       "15/01",
			locale:      config.DateLocaleEU,
			expected:    "2026-01-15",
			description: "Jan 15 next year",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clock := FixedClock{FixedTime: tt.today}
			result, err := ParseDate(tt.input, tt.locale, clock, tz)
			if err != nil {
				t.Errorf("ParseDate() error = %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("ParseDate() = %v, want %v (%s)", result, tt.expected, tt.description)
			}
		})
	}
}

func TestParseDate_ErrorMessages(t *testing.T) {
	clock := FixedClock{FixedTime: time.Date(2025, 12, 15, 10, 0, 0, 0, time.UTC)}
	tz, _ := time.LoadLocation("America/Los_Angeles")

	tests := []struct {
		name           string
		input          string
		locale         config.DateLocale
		wantErrContain string
	}{
		{
			name:           "ISO rejects numeric format",
			input:          "12/15/2025",
			locale:         config.DateLocaleISO,
			wantErrContain: "ambiguous numeric format",
		},
		{
			name:           "US rejects EU format",
			input:          "15/12/2025",
			locale:         config.DateLocaleUS,
			wantErrContain: "invalid due date for locale",
		},
		{
			name:           "EU rejects US format",
			input:          "12/15/2025",
			locale:         config.DateLocaleEU,
			wantErrContain: "invalid due date for locale",
		},
		{
			name:           "invalid date",
			input:          "02/30/2025",
			locale:         config.DateLocaleUS,
			wantErrContain: "invalid due date",
		},
		{
			name:           "completely invalid",
			input:          "not-a-date",
			locale:         config.DateLocaleISO,
			wantErrContain: "unable to parse",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseDate(tt.input, tt.locale, clock, tz)
			if err == nil {
				t.Errorf("ParseDate() expected error containing %q, got nil", tt.wantErrContain)
				return
			}
			if !contains(err.Error(), tt.wantErrContain) {
				t.Errorf("ParseDate() error = %q, want error containing %q", err.Error(), tt.wantErrContain)
			}
			// Verify original input is in error message
			if !contains(err.Error(), tt.input) {
				t.Errorf("ParseDate() error = %q, want error containing input %q", err.Error(), tt.input)
			}
		})
	}
}

func TestParseDate_Shortcuts(t *testing.T) {
	// Use a fixed date: 2025-12-15
	clock := FixedClock{FixedTime: time.Date(2025, 12, 15, 10, 0, 0, 0, time.UTC)}
	tz, _ := time.LoadLocation("America/Los_Angeles")

	tests := []struct {
		name     string
		input    string
		expected string
		locale   config.DateLocale
		wantErr  bool
	}{
		{"today", "today", "2025-12-15", config.DateLocaleISO, false},
		{"TODAY uppercase", "TODAY", "2025-12-15", config.DateLocaleISO, false},
		{"Today mixed case", "Today", "2025-12-15", config.DateLocaleISO, false},
		{"+0", "+0", "2025-12-15", config.DateLocaleISO, false},
		{"+1", "+1", "2025-12-16", config.DateLocaleISO, false},
		{"+2", "+2", "2025-12-17", config.DateLocaleISO, false},
		{"+7", "+7", "2025-12-22", config.DateLocaleISO, false},
		{"+30", "+30", "2026-01-14", config.DateLocaleISO, false},
		{"+365", "+365", "2026-12-15", config.DateLocaleISO, false},
		{"today with US locale", "today", "2025-12-15", config.DateLocaleUS, false},
		{"+1 with EU locale", "+1", "2025-12-16", config.DateLocaleEU, false},
		{"invalid: +abc", "+abc", "", config.DateLocaleISO, true},
		{"invalid: +-1", "+-1", "", config.DateLocaleISO, true},
		{"invalid: just +", "+", "", config.DateLocaleISO, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseDate(tt.input, tt.locale, clock, tz)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && result != tt.expected {
				t.Errorf("ParseDate() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFormatCanonical(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected string
	}{
		{"normal date", time.Date(2025, 12, 15, 10, 30, 0, 0, time.UTC), "2025-12-15"},
		{"start of year", time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), "2025-01-01"},
		{"end of year", time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC), "2025-12-31"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatCanonical(tt.input)
			if result != tt.expected {
				t.Errorf("FormatCanonical() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			containsMiddle(s, substr))))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
