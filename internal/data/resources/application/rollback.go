package application

import "time"

type RollbackEntry struct {
	ID                 string     `json:"id"`
	TargetSnapshotId   string     `json:"targetSnapshotId"`
	TargetGeneration   int        `json:"targetGeneration"`
	TargetPath         string     `json:"targetPath"`
	TriggeredBy        string     `json:"triggeredBy"`
	TriggeredAt        time.Time  `json:"triggeredAt"`
	CompletedAt        *time.Time `json:"completedAt,omitempty"`
	Status             string     `json:"status"`
	Error              string     `json:"error,omitempty"`
	RestoredGeneration *int       `json:"restoredGeneration,omitempty"`
	Namespace          string     `json:"namespace"`
}

