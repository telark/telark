package uninstall

import (
	metadatabase "github.com/telark/telark/internal/data/metadata/base"
)

type target struct {
	resourceType string
	metadata     metadatabase.Metadata
	finalizer    string
}

type Result struct {
	Removed int
	Failed  int
}
