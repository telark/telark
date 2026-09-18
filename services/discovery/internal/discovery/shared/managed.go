package shared

import (
	"github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/constants"
)

const (
	ManagedHelm    = "Helm"
	ManagedManual  = "manual"
	ManagedUnknown = "unknown"
)

func BuildManaged(managedBy, chartVal, versionVal string) application.Managed {
	by := ManagedUnknown
	if managedBy == ManagedHelm {
		by = ManagedHelm
	} else if managedBy != constants.EmptyString {
		by = ManagedManual
	}
	var chart, version *string
	if chartVal != constants.EmptyString {
		chart = &chartVal
	}
	if versionVal != constants.EmptyString {
		version = &versionVal
	}
	return application.Managed{By: by, Chart: chart, Version: version}
}
