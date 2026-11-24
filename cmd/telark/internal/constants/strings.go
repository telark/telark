package constants

const (
	EmptyString           = ""
	NewLine               = "\n"
	Space                 = " "
	ListSeparator         = ", "
	DefaultInitValue      = 0
	DefaultIncrementValue = 1
	ExitCodeSuccess       = 0
	ExitCodeFailure       = 1
)

const (
	CommandName      = "telark"
	AppName          = "telark"
	DefaultNamespace = "telark"
	DevVersion       = "0.0.0-dev"
	ClientQPS        = 50
	ClientBurst      = 100
	UnknownValue     = "unknown"
)

const (
	LabelPartOf    = "app.kubernetes.io/part-of"
	LabelVersion   = "app.kubernetes.io/version"
	LabelComponent = "app.kubernetes.io/component"
	TelarkSelector = LabelPartOf + "=" + AppName
	ServiceSuffix  = "-service"

	ImagePathSeparator   = "/"
	ImageTagSeparator    = ":"
	ImageDigestSeparator = "@"
)

const (
	FormatTable = "table"
	FormatJSON  = "json"
	FormatYAML  = "yaml"
	JSONIndent  = "  "
)

const (
	SymbolPass  = "✔"
	SymbolWarn  = "‼"
	SymbolFail  = "×"
	ColorGreen  = "\x1b[32m"
	ColorYellow = "\x1b[33m"
	ColorRed    = "\x1b[31m"
	ColorReset  = "\x1b[0m"
	EnvNoColor  = "NO_COLOR"
)
