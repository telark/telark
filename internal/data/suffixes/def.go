package suffixes

import (
	"fmt"

	"github.com/plsyro/data/shared"
)

type Suffix string

var AdmissionNameSuffix Suffix = Suffix(fmt.Sprintf(".%s.io", shared.BaseNamespace))
