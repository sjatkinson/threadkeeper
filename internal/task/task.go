package task

import (
	"encoding/json"
	"strings"
	"time"
)

// Status represents the status of a task.
type Status string

const (
	StatusOpen     Status = "open"
	StatusDone     Status = "done"
	StatusArchived Status = "archived"
)

// Task represents a task in the system.
type Task struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      Status     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DueAt       *time.Time `json:"due_at,omitempty"`
	Project     string     `json:"project,omitempty"`
	Tags        []string   `json:"tags"`
	ShortID     *int       `json:"short_id,omitempty"`
}

// taskJSON is used for JSON unmarshaling to handle string timestamps.
type taskJSON struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Status      Status   `json:"status"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
	DueAt       *string  `json:"due_at,omitempty"`
	Project     string   `json:"project,omitempty"`
	Tags        []string `json:"tags"`
	ShortID     *int     `json:"short_id,omitempty"`
}

// UnmarshalJSON implements custom JSON unmarshaling to parse ISO8601 timestamps.
func (t *Task) UnmarshalJSON(data []byte) error {
	var tj taskJSON
	if err := json.Unmarshal(data, &tj); err != nil {
		return err
	}

	t.ID = tj.ID
	t.Title = tj.Title
	t.Description = tj.Description
	t.Status = tj.Status
	t.Project = tj.Project
	t.Tags = tj.Tags
	t.ShortID = tj.ShortID

	// Parse timestamps
	if tj.CreatedAt != "" {
		createdAt, err := time.Parse(time.RFC3339, tj.CreatedAt)
		if err != nil {
			// Try parsing without timezone info
			createdAt, err = time.Parse("2006-01-02T15:04:05", tj.CreatedAt)
		}
		if err == nil {
			t.CreatedAt = createdAt.UTC()
		}
	}

	if tj.UpdatedAt != "" {
		updatedAt, err := time.Parse(time.RFC3339, tj.UpdatedAt)
		if err != nil {
			updatedAt, err = time.Parse("2006-01-02T15:04:05", tj.UpdatedAt)
		}
		if err == nil {
			t.UpdatedAt = updatedAt.UTC()
		}
	}

	if tj.DueAt != nil && *tj.DueAt != "" {
		dueAt, err := time.Parse(time.RFC3339, *tj.DueAt)
		if err != nil {
			dueAt, err = time.Parse("2006-01-02", *tj.DueAt)
		}
		if err == nil {
			t.DueAt = &dueAt
		}
	}

	return nil
}

// MarshalJSON implements custom JSON marshaling to format timestamps as ISO8601 strings
// and omit short_id when nil.
func (t *Task) MarshalJSON() ([]byte, error) {
	type Alias Task
	aux := &struct {
		CreatedAt string  `json:"created_at"`
		UpdatedAt string  `json:"updated_at"`
		DueAt     *string `json:"due_at,omitempty"`
		ShortID   *int    `json:"short_id,omitempty"`
		*Alias
	}{
		CreatedAt: t.CreatedAt.Format(time.RFC3339),
		UpdatedAt: t.UpdatedAt.Format(time.RFC3339),
		ShortID:   t.ShortID, // Will be omitted if nil due to omitempty
		Alias:     (*Alias)(t),
	}

	if t.DueAt != nil {
		s := t.DueAt.Format(time.RFC3339)
		aux.DueAt = &s
	}

	return json.Marshal(aux)
}

// NormalizeTags normalizes a list of tags by trimming whitespace and lowercasing.
func NormalizeTags(tags []string) []string {
	normalized := make([]string, 0, len(tags))
	seen := make(map[string]bool)
	for _, t := range tags {
		cleaned := strings.TrimSpace(strings.ToLower(t))
		if cleaned != "" && !seen[cleaned] {
			normalized = append(normalized, cleaned)
			seen[cleaned] = true
		}
	}
	return normalized
}

// Normalize ensures a task has all expected fields with reasonable defaults.
func (t *Task) Normalize() {
	if t.Title == "" {
		t.Title = ""
	}
	if t.Description == "" {
		t.Description = ""
	}
	if t.Status == "" {
		t.Status = StatusOpen
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now().UTC()
	}
	if t.UpdatedAt.IsZero() {
		t.UpdatedAt = t.CreatedAt
	}
	if t.Tags == nil {
		t.Tags = []string{}
	} else {
		t.Tags = NormalizeTags(t.Tags)
	}
}

// IsValidStatus checks if the status is a valid value.
func IsValidStatus(s Status) bool {
	return s == StatusOpen || s == StatusDone || s == StatusArchived
}
