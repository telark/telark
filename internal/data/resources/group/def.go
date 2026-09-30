package group

type Group struct {
	ID             string   `json:"id"`
	UserRefs       []string `json:"userRefs"`
	RoleRefs       []string `json:"roleRefs"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	CategoryRef    string   `json:"categoryRef"`
	CreationDate   string   `json:"creationDate"`
	LastUpdateDate *string  `json:"lastUpdateDate,omitempty"`
	CreatedBy      *string  `json:"createdBy,omitempty"`
	LastUpdatedBy  *string  `json:"lastUpdatedBy,omitempty"`
	// Projected from metadata by the owner while the cleanup finalizer holds the record.
	DeletionTimestamp *string `json:"deletionTimestamp,omitempty"`
}
