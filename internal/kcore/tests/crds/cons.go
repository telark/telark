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

const (
	TestVerbCreate      = "create"
	TestVerbPatch       = "patch"
	TestVerbUpdate      = "update"
	TestSpecKey         = "spec"
	TestStatusKey       = "status"
	TestMetadataKey     = "metadata"
	TestIDKey           = "id"
	TestDeletionKey     = "deletionTimestamp"
	TestSubresource     = "status"
	TestPhaseKey        = "phase"
	TestPhaseView       = "state"
	TestClusterKey      = "cluster"
	TestSizeKey         = "size"
	TestPhaseActive     = "active"
	TestPhaseDone       = "done"
	TestSizeValue       = "large"
	TestResourceVersion = "42"
	TestDeletionStamp   = "2026-01-02T03:04:05Z"
	TestWholeView       = "state"
	TestWholeStatusKey  = ""
	TestNameKey         = "name"
	TestNamespaceKey    = "namespace"
	TestRVKey           = "resourceVersion"
	NoPatches           = 0
	OnePatch            = 1
	TwoPatches          = 2

	ExpectedStatus   = "%s: status=%d err=%v, want %d"
	ExpectedField    = "%s: %s=%v, want %v"
	ExpectedActions  = "%s: status patch actions=%d, want %d"
	ExpectedNilView  = "%s: view=%v, want nil"
	ExpectedMissing  = "%s: key %q present, want absent"
	ExpectedSplitLen = "%s: spec=%v status=%v, want spec len %d status len %d"
)
