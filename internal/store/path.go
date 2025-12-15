package store

import "path/filepath"

// ThreadPath returns the canonical filesystem path for a thread directory.
// Path function: bucket = tid[0:2], path = threads/{bucket}/{tid}/
//
// This function must be:
//   - Deterministic (same inputs always produce same output)
//   - Stable across versions (algorithm must never change)
//   - Single source of truth (only implemented here)
func ThreadPath(threadsDir, threadID string) string {
	bucket := threadID[0:2]
	return filepath.Join(threadsDir, bucket, threadID)
}

// ThreadFilePath returns the path to thread.json within a thread directory.
func ThreadFilePath(threadsDir, threadID string) string {
	return filepath.Join(ThreadPath(threadsDir, threadID), "thread.json")
}
