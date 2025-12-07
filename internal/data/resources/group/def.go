package group

type GroupAsResource struct {
	ID               string   `json:"id"`
	AssignedUsersIDs []string `json:"assignedUsersIDs"`
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	CategoryID       string   `json:"categoryID"`
	CreationDate     string   `json:"creationDate"`
	LastUpdateDate   *string  `json:"lastUpdateDate,omitempty"`
	CreatedBy        *string  `json:"createdBy,omitempty"`
	LastUpdatedBy    *string  `json:"lastUpdatedBy,omitempty"`
}
