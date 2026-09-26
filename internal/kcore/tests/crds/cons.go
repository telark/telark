package crds

const (
	TestNamespace = "test-namespace"
	TestName      = "test-name"
	TestGroup     = "example.io"
	TestVersion   = "v1"
	TestPlural    = "widgets"
	TestKind      = "Widget"
	TestListKind  = "WidgetList"
	TestResource  = "deployments"
	TestVerbGet   = "get"
	TestTimeoutS  = 1

	ExpectedExistsFromGetError = "%s: exists=%v err=%v, want exists=%v err=%v"
	ExpectedBadRequest         = "%s: status=%d err=%v, want %d and a BadRequest error"
)
