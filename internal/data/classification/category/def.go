package category

type CategoryAsClassification struct {
	Categories []Category `json:"categories"`
}

type Category struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	Scope        string      `json:"scope"`
	Type         CategoryType `json:"type"`
	CreationDate string      `json:"creationDate"`
}

type CategoryType string

const (
	CategoryTypeBuiltIn CategoryType = "built-in"
	CategoryTypeCustom  CategoryType = "custom"
)

