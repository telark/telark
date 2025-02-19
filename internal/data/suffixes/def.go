package suffixes

import (
	"fmt"

	"github.com/plsyro/common-pkg/v2/global"
)

type Suffix string

const (
	MAIN_FEAT_NAME_SUFFIX Suffix = "-maintenance-feat"
)

var ADMISSION_NAME_SUFFIX Suffix = Suffix(fmt.Sprintf(".%s.io", global.BaseNamespace))
