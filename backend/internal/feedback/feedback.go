package feedback

import "time"

type Entry struct {
	Content   map[string]any `json:"content"`
	Timestamp time.Time      `json:"timestamp"`
	Round     int            `json:"round"`
}
