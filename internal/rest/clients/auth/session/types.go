package session

// The list endpoint answers spec plus a metadata subset; only the name is needed
// to address a session without its token.
type (
	sessionMetadata struct {
		Name string `json:"name"`
	}
	sessionRef struct {
		Metadata sessionMetadata `json:"metadata"`
	}
)
