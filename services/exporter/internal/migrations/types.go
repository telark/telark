package migrations

type Step struct {
	Number  int64
	Release string
	Name    string
	Apply   func() error
}
