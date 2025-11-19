package group

type GroupAsResource struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Description    string  `json:"description"`
	CategoryID     string  `json:"categoryID"`
	CreationDate   string  `json:"creationDate"`
	LastUpdateDate *string `json:"lastUpdateDate,omitempty"`
}

