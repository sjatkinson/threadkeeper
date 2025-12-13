package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/sjatkinson/threadkeeper/internal/task"
)

// FileStore provides file-based storage for tasks.
type FileStore struct {
	tasksDir string
}

// NewFileStore creates a new FileStore for the given tasks directory.
func NewFileStore(tasksDir string) *FileStore {
	return &FileStore{
		tasksDir: tasksDir,
	}
}

// LoadAll loads all tasks from the tasks directory.
func (s *FileStore) LoadAll() ([]*task.Task, error) {
	entries, err := os.ReadDir(s.tasksDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*task.Task{}, nil
		}
		return nil, fmt.Errorf("failed to read tasks directory: %w", err)
	}

	var tasks []*task.Task
	for _, entry := range entries {
		if entry.IsDir() || !entry.Type().IsRegular() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(s.tasksDir, entry.Name())
		t, err := s.loadTask(path)
		if err != nil {
			// Log but continue loading other tasks
			// In a production system, you might want to log this to stderr
			continue
		}
		tasks = append(tasks, t)
	}

	// Sort by created_at then ID for consistency
	sort.Slice(tasks, func(i, j int) bool {
		if !tasks[i].CreatedAt.Equal(tasks[j].CreatedAt) {
			return tasks[i].CreatedAt.Before(tasks[j].CreatedAt)
		}
		return tasks[i].ID < tasks[j].ID
	})

	return tasks, nil
}

// loadTask loads a single task from a JSON file.
func (s *FileStore) loadTask(path string) (*task.Task, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read task file %s: %w", path, err)
	}

	var t task.Task
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("failed to parse task file %s: %w", path, err)
	}

	// Normalize the task
	t.Normalize()

	return &t, nil
}

// GetByID loads a task by its durable ID.
// If the task is open and missing a short_id, one will be assigned automatically.
func (s *FileStore) GetByID(id string) (*task.Task, error) {
	path := filepath.Join(s.tasksDir, id+".json")
	t, err := s.loadTask(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("task %s not found", id)
		}
		return nil, err
	}

	// Ensure open tasks have short_ids
	if err := s.EnsureShortID(t); err != nil {
		// Log but don't fail - task is still valid without short_id
		// In production, you might want proper logging here
	}

	return t, nil
}

// GetByShortID finds a task by its short_id among open tasks only.
// Returns an error if not found or if multiple open tasks have the same short_id.
func (s *FileStore) GetByShortID(shortID int) (*task.Task, error) {
	tasks, err := s.LoadAll()
	if err != nil {
		return nil, err
	}

	var found *task.Task
	for _, t := range tasks {
		if t.Status == task.StatusOpen && t.ShortID != nil && *t.ShortID == shortID {
			if found != nil {
				// Ambiguity detected
				return nil, fmt.Errorf("short_id %d refers to multiple tasks (run reindex or use durable ID)", shortID)
			}
			found = t
		}
	}

	if found == nil {
		return nil, fmt.Errorf("no active task with short_id %d (use durable ID for completed tasks)", shortID)
	}

	return found, nil
}

// GenerateNextShortID finds the maximum existing short_id across all tasks
// and returns max + 1. If none exist, returns 1.
func (s *FileStore) GenerateNextShortID() (int, error) {
	tasks, err := s.LoadAll()
	if err != nil {
		return 0, err
	}

	maxSID := 0
	for _, t := range tasks {
		if t.ShortID != nil && *t.ShortID > maxSID {
			maxSID = *t.ShortID
		}
	}

	if maxSID == 0 {
		return 1, nil
	}
	return maxSID + 1, nil
}

// Save saves a task to its JSON file.
func (s *FileStore) Save(t *task.Task) error {
	path := filepath.Join(s.tasksDir, t.ID+".json")

	// Prepare data for JSON encoding
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	// Use atomic write: write to temp file, then rename
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write task file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath) // Clean up on error
		return fmt.Errorf("failed to rename task file: %w", err)
	}

	return nil
}

// EnsureShortID ensures an open task has a short_id. If it doesn't have one,
// assigns the next available short_id and saves the task.
func (s *FileStore) EnsureShortID(t *task.Task) error {
	// Only assign short_id to open tasks
	if t.Status != task.StatusOpen {
		return nil
	}

	// If it already has a short_id, nothing to do
	if t.ShortID != nil {
		return nil
	}

	// Generate and assign next short_id
	nextID, err := s.GenerateNextShortID()
	if err != nil {
		return fmt.Errorf("failed to generate short_id: %w", err)
	}

	t.ShortID = &nextID
	return s.Save(t)
}

// ResolveID resolves a task ID which may be either a durable ID or a short_id.
// Returns the task if found, or an error if not found or ambiguous.
// If the task is open and missing a short_id, one will be assigned automatically.
func (s *FileStore) ResolveID(idStr string) (*task.Task, error) {
	// First, try as durable ID
	t, err := s.GetByID(idStr)
	if err == nil {
		// EnsureShortID already called in GetByID
		return t, nil
	}

	// If not found, try as short_id
	shortID, err := strconv.Atoi(idStr)
	if err != nil {
		return nil, fmt.Errorf("'%s' is not a valid task ID or short_id", idStr)
	}

	t, err = s.GetByShortID(shortID)
	if err != nil {
		return nil, err
	}

	// Ensure it has short_id (should already have one, but be safe)
	if err := s.EnsureShortID(t); err != nil {
		// Log but don't fail
	}

	return t, nil
}
