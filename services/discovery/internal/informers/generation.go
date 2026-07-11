package informers

import (
	"github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/applications/history/diff"
)

func nextSnapshotGeneration(stored *application.Application) int {
	return diff.CurrentGenerationOrDefault(stored) + constants.DefaultAddValue
}
