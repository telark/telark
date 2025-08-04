package suffixes

import (
	"fmt"

	"github.com/plsyro/data-pkg/shared"
)

type Suffix string

const (
	MainFeatNameSuffix Suffix = "-maintenance-feat"
)

var AdmissionNameSuffix Suffix = Suffix(fmt.Sprintf(".%s.io", shared.BaseNamespace))
